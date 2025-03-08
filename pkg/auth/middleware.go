package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ContextKey type for context keys
type ContextKey string

// Context keys
const (
	UserContextKey ContextKey = "user"
	TenantContextKey ContextKey = "tenant"
)

// AuthenticationMiddleware is middleware for authenticating users
type AuthenticationMiddleware struct {
	OAuthManager *OAuthManager
	RBACManager  *RBACManager
	AllowedPaths map[string]bool  // Paths that don't require authentication
}

// NewAuthenticationMiddleware creates a new authentication middleware
func NewAuthenticationMiddleware(oauthManager *OAuthManager, rbacManager *RBACManager) *AuthenticationMiddleware {
	// Default paths that don't require authentication
	allowedPaths := map[string]bool{
		"/health":      true,
		"/metrics":     true,
		"/auth/login":  true,
		"/auth/logout": true,
		"/auth/callback": true,
	}

	return &AuthenticationMiddleware{
		OAuthManager: oauthManager,
		RBACManager:  rbacManager,
		AllowedPaths: allowedPaths,
	}
}

// AllowPath adds a path to the allowed paths list
func (am *AuthenticationMiddleware) AllowPath(path string) {
	am.AllowedPaths[path] = true
}

// Authenticate is a middleware that authenticates requests
func (am *AuthenticationMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication for allowed paths
		for allowedPath := range am.AllowedPaths {
			if strings.HasPrefix(r.URL.Path, allowedPath) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Get the token from the Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Check if it's a Bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "Invalid authorization header format, expected 'Bearer <token>'", http.StatusUnauthorized)
			return
		}

		token := parts[1]

		// Validate the token using the default provider for now
		// In a more sophisticated implementation, we would determine the provider from the token
		provider, err := am.OAuthManager.GetDefaultProvider()
		if err != nil {
			http.Error(w, "No authentication provider configured", http.StatusInternalServerError)
			return
		}

		// Validate the token
		tokenInfo, err := provider.ValidateToken(token)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid token: %s", err.Error()), http.StatusUnauthorized)
			return
		}

		// Get the user from the token
		userInfo, err := provider.GetUserInfo(tokenInfo)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get user info: %s", err.Error()), http.StatusInternalServerError)
			return
		}

		// Try to find the user in the RBAC manager by external ID
		user, err := am.RBACManager.GetUserByExternalID(userInfo.Sub)
		if err != nil {
			// User does not exist, we should create one
			// In a production system, you might want to restrict this or implement an approval process
			user = NewUser(
				userInfo.Sub, // Use the sub as the ID for simplicity
				userInfo.Sub,
				userInfo.PreferredUsername,
				userInfo.Email,
				"default", // Default tenant for now
			)

			// Assign default roles
			defaultRole, err := am.RBACManager.GetRole("user")
			if err == nil { // If default role exists
				user.AssignRole(defaultRole)
			}

			// Register the user
			if err := am.RBACManager.RegisterUser(user); err != nil {
				http.Error(w, fmt.Sprintf("Failed to register user: %s", err.Error()), http.StatusInternalServerError)
				return
			}
		}

		// Add the user to the request context
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		// Add the tenant to the request context
		ctx = context.WithValue(ctx, TenantContextKey, user.TenantID)

		// Call the next handler with the updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthorizationMiddleware is middleware for authorizing users
type AuthorizationMiddleware struct {
	RBACManager *RBACManager
}

// NewAuthorizationMiddleware creates a new authorization middleware
func NewAuthorizationMiddleware(rbacManager *RBACManager) *AuthorizationMiddleware {
	return &AuthorizationMiddleware{
		RBACManager: rbacManager,
	}
}

// Authorize creates a middleware that authorizes requests based on required permissions
func (am *AuthorizationMiddleware) Authorize(resourceType ResourceType, action Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get the user from the context
			user, ok := r.Context().Value(UserContextKey).(*User)
			if !ok {
				http.Error(w, "User not found in request context", http.StatusUnauthorized)
				return
			}

			// Get tenant from context
			tenantID, ok := r.Context().Value(TenantContextKey).(string)
			if !ok {
				tenantID = "default" // Fallback to default tenant
			}

			// Determine the resource ID from the request
			// This is a simplified example - in a real application,
			// you would extract the resource ID from the URL or request body
			var resourceID ResourceID = WildcardID

			// For specific endpoints that operate on a resource, extract the ID
			// Example: /api/v1/tasks/123 -> resourceID = "123"
			pathParts := strings.Split(r.URL.Path, "/")
			if len(pathParts) >= 4 && pathParts[len(pathParts)-2] == string(resourceType) {
				resourceID = ResourceID(pathParts[len(pathParts)-1])
			}

			// Create the authorization request
			authRequest := AuthorizationRequest{
				UserID:       user.ID,
				Action:       action,
				ResourceType: resourceType,
				ResourceID:   resourceID,
				TenantID:     tenantID,
				Context:      map[string]interface{}{
					"method": r.Method,
					"path":   r.URL.Path,
					"query":  r.URL.Query(),
				},
			}

			// Check if the user is authorized
			result, err := am.RBACManager.IsAuthorized(authRequest)
			if err != nil {
				http.Error(w, fmt.Sprintf("Authorization error: %s", err.Error()), http.StatusInternalServerError)
				return
			}

			if !result.Allowed {
				http.Error(w, fmt.Sprintf("Access denied: %s", result.Reason), http.StatusForbidden)
				return
			}

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// ResourceBasedAuthorize creates a middleware that authorizes requests based on the resource ID in the URL
func (am *AuthorizationMiddleware) ResourceBasedAuthorize(resourceType ResourceType, action Permission, idParam string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get the user from the context
			user, ok := r.Context().Value(UserContextKey).(*User)
			if !ok {
				http.Error(w, "User not found in request context", http.StatusUnauthorized)
				return
			}

			// Get tenant from context
			tenantID, ok := r.Context().Value(TenantContextKey).(string)
			if !ok {
				tenantID = "default" // Fallback to default tenant
			}

			// Extract resource ID from the URL path using the provided parameter name
			resourceID := ResourceID(WildcardID)
			
			// This example assumes you're using a router that supports path parameters
			// For a more robust implementation, you'd use your router's parameter extraction method
			// Example with Gorilla Mux: vars := mux.Vars(r); resourceID = ResourceID(vars[idParam])
			
			// Simplified extraction (assumes consistent path format)
			pathParts := strings.Split(r.URL.Path, "/")
			for i, part := range pathParts {
				if part == idParam && i+1 < len(pathParts) {
					resourceID = ResourceID(pathParts[i+1])
					break
				}
			}

			// Create the authorization request
			authRequest := AuthorizationRequest{
				UserID:       user.ID,
				Action:       action,
				ResourceType: resourceType,
				ResourceID:   resourceID,
				TenantID:     tenantID,
				Context: map[string]interface{}{
					"method": r.Method,
					"path":   r.URL.Path,
				},
			}

			// Check if the user is authorized
			result, err := am.RBACManager.IsAuthorized(authRequest)
			if err != nil {
				http.Error(w, fmt.Sprintf("Authorization error: %s", err.Error()), http.StatusInternalServerError)
				return
			}

			if !result.Allowed {
				http.Error(w, fmt.Sprintf("Access denied: %s", result.Reason), http.StatusForbidden)
				return
			}

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// AuditLogMiddleware is middleware for logging audit events
type AuditLogMiddleware struct {
	Logger AuditLogger
}

// AuditLogger defines the interface for audit logging
type AuditLogger interface {
	LogEvent(event AuditEvent) error
}

// AuditEvent represents an audit log event
type AuditEvent struct {
	UserID        string                 `json:"user_id"`
	TenantID      string                 `json:"tenant_id"`
	Action        string                 `json:"action"`
	ResourceType  string                 `json:"resource_type"`
	ResourceID    string                 `json:"resource_id"`
	Timestamp     time.Time              `json:"timestamp"`
	Status        int                    `json:"status"`
	StatusMessage string                 `json:"status_message"`
	RequestMethod string                 `json:"request_method"`
	RequestPath   string                 `json:"request_path"`
	RequestIP     string                 `json:"request_ip"`
	RequestData   map[string]interface{} `json:"request_data,omitempty"`
	ResponseData  map[string]interface{} `json:"response_data,omitempty"`
}

// NewAuditLogMiddleware creates a new audit log middleware
func NewAuditLogMiddleware(logger AuditLogger) *AuditLogMiddleware {
	return &AuditLogMiddleware{
		Logger: logger,
	}
}

// responseRecorder is a wrapper for http.ResponseWriter that captures the status code
type responseRecorder struct {
	http.ResponseWriter
	StatusCode int
	Body       []byte
}

// WriteHeader captures the status code
func (r *responseRecorder) WriteHeader(statusCode int) {
	r.StatusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response body
func (r *responseRecorder) Write(b []byte) (int, error) {
	r.Body = append(r.Body, b...)
	return r.ResponseWriter.Write(b)
}

// LogAudit is a middleware that logs audit events
func (alm *AuditLogMiddleware) LogAudit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a response recorder
		recorder := &responseRecorder{
			ResponseWriter: w,
			StatusCode:     http.StatusOK, // Default status code
		}

		// Extract user and tenant information from context
		var userID, tenantID string
		if user, ok := r.Context().Value(UserContextKey).(*User); ok {
			userID = user.ID
			tenantID = user.TenantID
		}

		// Get resource information from path
		// This is a simplified example - in a real application,
		// you would extract more meaningful information
		pathParts := strings.Split(r.URL.Path, "/")
		var resourceType, resourceID string
		if len(pathParts) >= 3 {
			resourceType = pathParts[2]
			if len(pathParts) >= 4 {
				resourceID = pathParts[3]
			}
		}

		// Determine action from HTTP method
		action := r.Method

		// Get request data for auditing
		requestData := make(map[string]interface{})
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			// Parse JSON body if present
			if r.Header.Get("Content-Type") == "application/json" {
				var data map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&data)
				if err == nil {
					requestData = data
				}
				// Important: since we've read the body, we need to restore it
				// This example omits this step for simplicity
			}
		}

		// Get client IP
		clientIP := r.RemoteAddr
		if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
			clientIP = forwardedFor
		}

		// Call the next handler
		start := time.Now()
		next.ServeHTTP(recorder, r)
		duration := time.Since(start)

		// Prepare response data
		responseData := make(map[string]interface{})
		if recorder.StatusCode >= 200 && recorder.StatusCode < 300 {
			// Only try to parse successful JSON responses
			if strings.Contains(w.Header().Get("Content-Type"), "application/json") {
				var data map[string]interface{}
				err := json.Unmarshal(recorder.Body, &data)
				if err == nil {
					responseData = data
				}
			}
		}

		// Create the audit event
		event := AuditEvent{
			UserID:        userID,
			TenantID:      tenantID,
			Action:        action,
			ResourceType:  resourceType,
			ResourceID:    resourceID,
			Timestamp:     time.Now(),
			Status:        recorder.StatusCode,
			StatusMessage: http.StatusText(recorder.StatusCode),
			RequestMethod: r.Method,
			RequestPath:   r.URL.Path,
			RequestIP:     clientIP,
			RequestData:   requestData,
			ResponseData:  responseData,
		}

		// Log the event
		if err := alm.Logger.LogEvent(event); err != nil {
			// Just log the error, don't affect the response
			fmt.Printf("Error logging audit event: %s\n", err.Error())
		}
	})
}

// DefaultAuditLogger is a simple implementation of AuditLogger
type DefaultAuditLogger struct {
	LogFile string
}

// LogEvent implements AuditLogger interface
func (dal *DefaultAuditLogger) LogEvent(event AuditEvent) error {
	// In a real implementation, you would write to a file, database, or log service
	// For simplicity, we're just printing to stdout
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	fmt.Printf("AUDIT: %s\n", string(data))
	return nil
}

// NewDefaultAuditLogger creates a new default audit logger
func NewDefaultAuditLogger(logFile string) *DefaultAuditLogger {
	return &DefaultAuditLogger{
		LogFile: logFile,
	}
} 