package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Errors
var (
	ErrConnectorNotFound   = errors.New("connector not found")
	ErrConnectorExists     = errors.New("connector already exists")
	ErrInvalidConfiguration = errors.New("invalid connector configuration")
	ErrConnectorDisabled   = errors.New("connector is disabled")
	ErrAuthenticationFailed = errors.New("authentication failed")
	ErrOperationFailed     = errors.New("operation failed")
	ErrRateLimitExceeded   = errors.New("rate limit exceeded")
	ErrUnsupportedMethod   = errors.New("unsupported method")
)

// ConnectorStatus represents the status of a connector
type ConnectorStatus string

// Connector statuses
const (
	ConnectorStatusActive    ConnectorStatus = "active"
	ConnectorStatusDisabled  ConnectorStatus = "disabled"
	ConnectorStatusConfiguring ConnectorStatus = "configuring"
	ConnectorStatusError     ConnectorStatus = "error"
)

// ConnectorEvent represents an event from a connector
type ConnectorEvent struct {
	ID           string                 `json:"id"`
	ConnectorID  string                 `json:"connector_id"`
	ConnectorType string                `json:"connector_type"`
	EventType    string                 `json:"event_type"`
	Data         map[string]interface{} `json:"data"`
	Timestamp    time.Time              `json:"timestamp"`
	Status       string                 `json:"status"`
	Error        string                 `json:"error,omitempty"`
}

// ConnectorType represents a type of connector
type ConnectorType string

// Common connector types
const (
	ConnectorTypeJira        ConnectorType = "jira"
	ConnectorTypeGitHub      ConnectorType = "github"
	ConnectorTypeSlack       ConnectorType = "slack"
	ConnectorTypeJenkins     ConnectorType = "jenkins"
	ConnectorTypeTeams       ConnectorType = "teams"
	ConnectorTypeServiceNow  ConnectorType = "servicenow"
	ConnectorTypeSalesforce  ConnectorType = "salesforce"
	ConnectorTypeWebhook     ConnectorType = "webhook"
	ConnectorTypeS3          ConnectorType = "s3"
	ConnectorTypeGCS         ConnectorType = "gcs"
	ConnectorTypeAzureBlob   ConnectorType = "azureblob"
	ConnectorTypeJDBC        ConnectorType = "jdbc"
	ConnectorTypeRabbitMQ    ConnectorType = "rabbitmq"
	ConnectorTypeKafka       ConnectorType = "kafka"
	ConnectorTypeEmail       ConnectorType = "email"
	ConnectorTypeSMTP        ConnectorType = "smtp"
	ConnectorTypeAzureDevOps ConnectorType = "azuredevops"
	ConnectorTypeDatadog     ConnectorType = "datadog"
	ConnectorTypeNewRelic    ConnectorType = "newrelic"
	ConnectorTypePrometheus  ConnectorType = "prometheus"
)

// ConnectorConfig represents a connector configuration
type ConnectorConfig struct {
	ID              string                    `json:"id"`
	Name            string                    `json:"name"`
	Description     string                    `json:"description,omitempty"`
	Type            ConnectorType             `json:"type"`
	TenantID        string                    `json:"tenant_id"`
	Status          ConnectorStatus           `json:"status"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
	Settings        map[string]interface{}    `json:"settings"`
	Credentials     map[string]interface{}    `json:"credentials,omitempty"`
	RateLimits      *RateLimitConfig          `json:"rate_limits,omitempty"`
	Metadata        map[string]string         `json:"metadata,omitempty"`
	HealthCheckURL  string                    `json:"health_check_url,omitempty"`
	LastHealthCheck *ConnectorHealthCheckResult `json:"last_health_check,omitempty"`
	Tags            []string                  `json:"tags,omitempty"`
}

// RateLimitConfig specifies rate limiting for a connector
type RateLimitConfig struct {
	RequestsPerMinute int           `json:"requests_per_minute"`
	BurstSize         int           `json:"burst_size"`
	RetryPolicy       *RetryPolicy  `json:"retry_policy,omitempty"`
}

// RetryPolicy defines how to retry failed operations
type RetryPolicy struct {
	MaxRetries      int           `json:"max_retries"`
	InitialInterval time.Duration `json:"initial_interval"`
	MaxInterval     time.Duration `json:"max_interval"`
	Multiplier      float64       `json:"multiplier"`
	Jitter          float64       `json:"jitter"`
}

// ConnectorHealthCheckResult represents the result of a connector health check
type ConnectorHealthCheckResult struct {
	Status     string     `json:"status"`
	Message    string     `json:"message,omitempty"`
	Timestamp  time.Time  `json:"timestamp"`
	Latency    int64      `json:"latency_ms"`
	Error      string     `json:"error,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// ConnectorAction represents an action to perform with a connector
type ConnectorAction struct {
	Method  string                 `json:"method"`
	Path    string                 `json:"path,omitempty"`
	Params  map[string]interface{} `json:"params,omitempty"`
	Body    map[string]interface{} `json:"body,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// ConnectorResponse represents a response from a connector
type ConnectorResponse struct {
	StatusCode int                    `json:"status_code"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Headers    map[string]string      `json:"headers,omitempty"`
	Error      string                 `json:"error,omitempty"`
	RequestID  string                 `json:"request_id,omitempty"`
	Latency    int64                  `json:"latency_ms"`
	Timestamp  time.Time              `json:"timestamp"`
	Size       int64                  `json:"size_bytes,omitempty"`
}

// ConnectorHook defines a hook for pre/post processing
type ConnectorHook interface {
	OnBeforeAction(ctx context.Context, connectorID string, action *ConnectorAction) (*ConnectorAction, error)
	OnAfterAction(ctx context.Context, connectorID string, action *ConnectorAction, response *ConnectorResponse) (*ConnectorResponse, error)
	OnError(ctx context.Context, connectorID string, action *ConnectorAction, err error) error
}

// Connector defines the interface for connectors
type Connector interface {
	// Core methods
	Initialize(ctx context.Context, config *ConnectorConfig) error
	Execute(ctx context.Context, action *ConnectorAction) (*ConnectorResponse, error)
	HealthCheck(ctx context.Context) (*ConnectorHealthCheckResult, error)
	Shutdown(ctx context.Context) error
	
	// Getters
	GetConfig() *ConnectorConfig
	GetType() ConnectorType
	GetStatus() ConnectorStatus
	
	// Event subscription
	Subscribe(ctx context.Context, eventTypes []string) (<-chan *ConnectorEvent, error)
}

// ConnectorFactory creates connectors
type ConnectorFactory interface {
	CreateConnector(ctx context.Context, config *ConnectorConfig) (Connector, error)
	GetSupportedType() ConnectorType
	ValidateConfig(config *ConnectorConfig) error
}

// ConnectorStore defines the interface for connector storage
type ConnectorStore interface {
	GetConnector(id string) (*ConnectorConfig, error)
	ListConnectors(tenantID string) ([]*ConnectorConfig, error)
	CreateConnector(config *ConnectorConfig) error
	UpdateConnector(config *ConnectorConfig) error
	DeleteConnector(id string) error
	ListConnectorTypes() ([]ConnectorType, error)
}

// ConnectorManager manages connectors
type ConnectorManager struct {
	mu             sync.RWMutex
	connectors     map[string]Connector
	factories      map[ConnectorType]ConnectorFactory
	store          ConnectorStore
	hooks          []ConnectorHook
	metrics        ConnectorMetrics
	credentialManager CredentialManager
}

// ConnectorMetrics collects metrics about connector usage
type ConnectorMetrics interface {
	RecordRequest(connectorID string, connectorType ConnectorType, method string, statusCode int, latency time.Duration, size int64)
	RecordError(connectorID string, connectorType ConnectorType, method string, errorType string)
}

// CredentialManager manages credentials for connectors
type CredentialManager interface {
	GetCredentials(ctx context.Context, connectorID string) (map[string]interface{}, error)
	StoreCredentials(ctx context.Context, connectorID string, credentials map[string]interface{}) error
	DeleteCredentials(ctx context.Context, connectorID string) error
	RotateCredentials(ctx context.Context, connectorID string) (map[string]interface{}, error)
}

// NewConnectorManager creates a new connector manager
func NewConnectorManager(store ConnectorStore, credMgr CredentialManager, metrics ConnectorMetrics) *ConnectorManager {
	return &ConnectorManager{
		connectors:        make(map[string]Connector),
		factories:         make(map[ConnectorType]ConnectorFactory),
		store:             store,
		hooks:             make([]ConnectorHook, 0),
		metrics:           metrics,
		credentialManager: credMgr,
	}
}

// RegisterFactory registers a connector factory
func (cm *ConnectorManager) RegisterFactory(factory ConnectorFactory) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	cm.factories[factory.GetSupportedType()] = factory
}

// RegisterHook registers a connector hook
func (cm *ConnectorManager) RegisterHook(hook ConnectorHook) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	cm.hooks = append(cm.hooks, hook)
}

// GetConnector gets a connector by ID
func (cm *ConnectorManager) GetConnector(ctx context.Context, id string) (Connector, error) {
	cm.mu.RLock()
	connector, exists := cm.connectors[id]
	cm.mu.RUnlock()
	
	if exists {
		return connector, nil
	}
	
	// Load from store
	config, err := cm.store.GetConnector(id)
	if err != nil {
		return nil, err
	}
	
	// Create and initialize connector
	return cm.initializeConnector(ctx, config)
}

// CreateConnector creates a new connector
func (cm *ConnectorManager) CreateConnector(ctx context.Context, config *ConnectorConfig) (string, error) {
	// Validate config
	factory, exists := cm.factories[config.Type]
	if !exists {
		return "", fmt.Errorf("no factory registered for connector type: %s", config.Type)
	}
	
	if err := factory.ValidateConfig(config); err != nil {
		return "", err
	}
	
	// Set timestamps
	now := time.Now()
	config.CreatedAt = now
	config.UpdatedAt = now
	
	// Store credentials separately if credential manager is available
	if cm.credentialManager != nil && config.Credentials != nil {
		if err := cm.credentialManager.StoreCredentials(ctx, config.ID, config.Credentials); err != nil {
			return "", err
		}
		// Don't store credentials in the config
		config.Credentials = nil
	}
	
	// Store in database
	if err := cm.store.CreateConnector(config); err != nil {
		return "", err
	}
	
	// Create and initialize connector if it's active
	if config.Status == ConnectorStatusActive {
		_, err := cm.initializeConnector(ctx, config)
		if err != nil {
			// Update status to error
			config.Status = ConnectorStatusError
			config.UpdatedAt = time.Now()
			_ = cm.store.UpdateConnector(config)
			return config.ID, err
		}
	}
	
	return config.ID, nil
}

// initializeConnector initializes a connector
func (cm *ConnectorManager) initializeConnector(ctx context.Context, config *ConnectorConfig) (Connector, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	// Check if it already exists
	if connector, exists := cm.connectors[config.ID]; exists {
		return connector, nil
	}
	
	// Get factory
	factory, exists := cm.factories[config.Type]
	if !exists {
		return nil, fmt.Errorf("no factory registered for connector type: %s", config.Type)
	}
	
	// Get credentials if needed
	if cm.credentialManager != nil && (config.Credentials == nil || len(config.Credentials) == 0) {
		creds, err := cm.credentialManager.GetCredentials(ctx, config.ID)
		if err != nil {
			return nil, err
		}
		config.Credentials = creds
	}
	
	// Create connector
	connector, err := factory.CreateConnector(ctx, config)
	if err != nil {
		return nil, err
	}
	
	// Initialize connector
	if err := connector.Initialize(ctx, config); err != nil {
		return nil, err
	}
	
	// Store in memory
	cm.connectors[config.ID] = connector
	
	return connector, nil
}

// UpdateConnector updates a connector
func (cm *ConnectorManager) UpdateConnector(ctx context.Context, config *ConnectorConfig) error {
	// Validate config
	factory, exists := cm.factories[config.Type]
	if !exists {
		return fmt.Errorf("no factory registered for connector type: %s", config.Type)
	}
	
	if err := factory.ValidateConfig(config); err != nil {
		return err
	}
	
	// Update timestamp
	config.UpdatedAt = time.Now()
	
	// Handle credentials
	if cm.credentialManager != nil && config.Credentials != nil {
		if err := cm.credentialManager.StoreCredentials(ctx, config.ID, config.Credentials); err != nil {
			return err
		}
		// Don't store credentials in the config
		config.Credentials = nil
	}
	
	// Update in database
	if err := cm.store.UpdateConnector(config); err != nil {
		return err
	}
	
	// Shut down existing connector if it exists
	cm.mu.Lock()
	if connector, exists := cm.connectors[config.ID]; exists {
		_ = connector.Shutdown(ctx)
		delete(cm.connectors, config.ID)
	}
	cm.mu.Unlock()
	
	// Initialize new connector if it's active
	if config.Status == ConnectorStatusActive {
		_, err := cm.initializeConnector(ctx, config)
		if err != nil {
			// Update status to error
			config.Status = ConnectorStatusError
			config.UpdatedAt = time.Now()
			_ = cm.store.UpdateConnector(config)
			return err
		}
	}
	
	return nil
}

// DeleteConnector deletes a connector
func (cm *ConnectorManager) DeleteConnector(ctx context.Context, id string) error {
	// Shut down connector if it exists
	cm.mu.Lock()
	if connector, exists := cm.connectors[id]; exists {
		_ = connector.Shutdown(ctx)
		delete(cm.connectors, id)
	}
	cm.mu.Unlock()
	
	// Delete credentials
	if cm.credentialManager != nil {
		_ = cm.credentialManager.DeleteCredentials(ctx, id)
	}
	
	// Delete from database
	return cm.store.DeleteConnector(id)
}

// ListConnectorTypes lists supported connector types
func (cm *ConnectorManager) ListConnectorTypes() ([]ConnectorType, error) {
	return cm.store.ListConnectorTypes()
}

// ListConnectors lists connectors for a tenant
func (cm *ConnectorManager) ListConnectors(tenantID string) ([]*ConnectorConfig, error) {
	return cm.store.ListConnectors(tenantID)
}

// ExecuteConnectorAction executes an action on a connector
func (cm *ConnectorManager) ExecuteConnectorAction(ctx context.Context, connectorID string, action *ConnectorAction) (*ConnectorResponse, error) {
	// Get connector
	connector, err := cm.GetConnector(ctx, connectorID)
	if err != nil {
		return nil, err
	}
	
	// Check status
	if connector.GetStatus() != ConnectorStatusActive {
		return nil, ErrConnectorDisabled
	}
	
	// Make a copy of the action for hooks
	actionCopy := *action
	
	// Apply pre-hooks
	for _, hook := range cm.hooks {
		modifiedAction, err := hook.OnBeforeAction(ctx, connectorID, &actionCopy)
		if err != nil {
			return nil, err
		}
		if modifiedAction != nil {
			actionCopy = *modifiedAction
		}
	}
	
	startTime := time.Now()
	
	// Execute action
	response, err := connector.Execute(ctx, &actionCopy)
	if err != nil {
		// Apply error hooks
		for _, hook := range cm.hooks {
			errFromHook := hook.OnError(ctx, connectorID, &actionCopy, err)
			if errFromHook != nil {
				// Log the error but continue with other hooks
				fmt.Printf("Error from hook: %v\n", errFromHook)
			}
		}
		
		// Record metrics
		if cm.metrics != nil {
			cm.metrics.RecordError(connectorID, connector.GetType(), actionCopy.Method, err.Error())
		}
		
		return nil, err
	}
	
	endTime := time.Now()
	latency := endTime.Sub(startTime)
	
	// Set response metadata
	if response != nil {
		response.Timestamp = endTime
		response.Latency = latency.Milliseconds()
	}
	
	// Apply post-hooks
	for _, hook := range cm.hooks {
		modifiedResponse, err := hook.OnAfterAction(ctx, connectorID, &actionCopy, response)
		if err != nil {
			return nil, err
		}
		if modifiedResponse != nil {
			response = modifiedResponse
		}
	}
	
	// Record metrics
	if cm.metrics != nil && response != nil {
		var size int64
		if response.Size > 0 {
			size = response.Size
		} else if response.Data != nil {
			// Estimate size by marshaling to JSON
			data, _ := json.Marshal(response.Data)
			size = int64(len(data))
		}
		
		cm.metrics.RecordRequest(connectorID, connector.GetType(), actionCopy.Method, response.StatusCode, latency, size)
	}
	
	return response, nil
}

// GetConnectorStatus gets the status and health check information for a connector
func (cm *ConnectorManager) GetConnectorStatus(ctx context.Context, id string) (*ConnectorHealthCheckResult, error) {
	// Get connector
	connector, err := cm.GetConnector(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// Perform health check
	result, err := connector.HealthCheck(ctx)
	if err != nil {
		return nil, err
	}
	
	// Update connector config with health check result
	config := connector.GetConfig()
	config.LastHealthCheck = result
	config.UpdatedAt = time.Now()
	
	// Status change if health check failed
	if result.Status == "error" && config.Status == ConnectorStatusActive {
		config.Status = ConnectorStatusError
	}
	
	// Update in database
	_ = cm.store.UpdateConnector(config)
	
	return result, nil
}

// RotateConnectorCredentials rotates credentials for a connector
func (cm *ConnectorManager) RotateConnectorCredentials(ctx context.Context, id string) error {
	if cm.credentialManager == nil {
		return errors.New("credential manager not available")
	}
	
	// Rotate credentials
	newCreds, err := cm.credentialManager.RotateCredentials(ctx, id)
	if err != nil {
		return err
	}
	
	// Get connector config
	config, err := cm.store.GetConnector(id)
	if err != nil {
		return err
	}
	
	// Update and reinitialize connector
	config.UpdatedAt = time.Now()
	config.Credentials = newCreds
	
	return cm.UpdateConnector(ctx, config)
}

// ShutdownAllConnectors shuts down all connectors
func (cm *ConnectorManager) ShutdownAllConnectors(ctx context.Context) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	for id, connector := range cm.connectors {
		err := connector.Shutdown(ctx)
		if err != nil {
			fmt.Printf("Error shutting down connector %s: %v\n", id, err)
		}
	}
	
	cm.connectors = make(map[string]Connector)
}

// BaseConnector provides common functionality for connectors
type BaseConnector struct {
	config       *ConnectorConfig
	initialized  bool
	status       ConnectorStatus
	lastActivity time.Time
	mu           sync.RWMutex
}

// Initialize initializes the base connector
func (bc *BaseConnector) Initialize(ctx context.Context, config *ConnectorConfig) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	bc.config = config
	bc.status = config.Status
	bc.initialized = true
	bc.lastActivity = time.Now()
	
	return nil
}

// GetConfig returns the connector config
func (bc *BaseConnector) GetConfig() *ConnectorConfig {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	
	return bc.config
}

// GetType returns the connector type
func (bc *BaseConnector) GetType() ConnectorType {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	
	return bc.config.Type
}

// GetStatus returns the connector status
func (bc *BaseConnector) GetStatus() ConnectorStatus {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	
	return bc.status
}

// UpdateLastActivity updates the last activity timestamp
func (bc *BaseConnector) UpdateLastActivity() {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	bc.lastActivity = time.Now()
}

// NoOpConnectorHook is a no-op implementation of ConnectorHook
type NoOpConnectorHook struct{}

// OnBeforeAction is a no-op before action hook
func (n *NoOpConnectorHook) OnBeforeAction(ctx context.Context, connectorID string, action *ConnectorAction) (*ConnectorAction, error) {
	return action, nil
}

// OnAfterAction is a no-op after action hook
func (n *NoOpConnectorHook) OnAfterAction(ctx context.Context, connectorID string, action *ConnectorAction, response *ConnectorResponse) (*ConnectorResponse, error) {
	return response, nil
}

// OnError is a no-op error hook
func (n *NoOpConnectorHook) OnError(ctx context.Context, connectorID string, action *ConnectorAction, err error) error {
	return nil
}

// NoOpConnectorMetrics is a no-op implementation of ConnectorMetrics
type NoOpConnectorMetrics struct{}

// RecordRequest is a no-op request metrics recorder
func (n *NoOpConnectorMetrics) RecordRequest(connectorID string, connectorType ConnectorType, method string, statusCode int, latency time.Duration, size int64) {
	// No-op
}

// RecordError is a no-op error metrics recorder
func (n *NoOpConnectorMetrics) RecordError(connectorID string, connectorType ConnectorType, method string, errorType string) {
	// No-op
}

// InMemoryConnectorStore is an in-memory implementation of ConnectorStore
type InMemoryConnectorStore struct {
	connectors map[string]*ConnectorConfig
	mu         sync.RWMutex
}

// NewInMemoryConnectorStore creates a new in-memory connector store
func NewInMemoryConnectorStore() *InMemoryConnectorStore {
	return &InMemoryConnectorStore{
		connectors: make(map[string]*ConnectorConfig),
	}
}

// GetConnector gets a connector by ID
func (s *InMemoryConnectorStore) GetConnector(id string) (*ConnectorConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	connector, exists := s.connectors[id]
	if !exists {
		return nil, ErrConnectorNotFound
	}
	
	// Return a copy to prevent modification
	configCopy := *connector
	return &configCopy, nil
}

// ListConnectors lists connectors for a tenant
func (s *InMemoryConnectorStore) ListConnectors(tenantID string) ([]*ConnectorConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var result []*ConnectorConfig
	for _, connector := range s.connectors {
		if connector.TenantID == tenantID {
			// Return copies to prevent modification
			configCopy := *connector
			result = append(result, &configCopy)
		}
	}
	
	return result, nil
}

// CreateConnector creates a connector
func (s *InMemoryConnectorStore) CreateConnector(config *ConnectorConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, exists := s.connectors[config.ID]; exists {
		return ErrConnectorExists
	}
	
	// Store a copy to prevent modification
	configCopy := *config
	s.connectors[config.ID] = &configCopy
	
	return nil
}

// UpdateConnector updates a connector
func (s *InMemoryConnectorStore) UpdateConnector(config *ConnectorConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, exists := s.connectors[config.ID]; !exists {
		return ErrConnectorNotFound
	}
	
	// Store a copy to prevent modification
	configCopy := *config
	s.connectors[config.ID] = &configCopy
	
	return nil
}

// DeleteConnector deletes a connector
func (s *InMemoryConnectorStore) DeleteConnector(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, exists := s.connectors[id]; !exists {
		return ErrConnectorNotFound
	}
	
	delete(s.connectors, id)
	
	return nil
}

// ListConnectorTypes lists supported connector types
func (s *InMemoryConnectorStore) ListConnectorTypes() ([]ConnectorType, error) {
	return []ConnectorType{
		ConnectorTypeJira,
		ConnectorTypeGitHub,
		ConnectorTypeSlack,
		ConnectorTypeJenkins,
		ConnectorTypeTeams,
		ConnectorTypeServiceNow,
		ConnectorTypeSalesforce,
		ConnectorTypeWebhook,
		ConnectorTypeS3,
		ConnectorTypeGCS,
		ConnectorTypeAzureBlob,
		ConnectorTypeJDBC,
		ConnectorTypeRabbitMQ,
		ConnectorTypeKafka,
		ConnectorTypeEmail,
		ConnectorTypeSMTP,
	}, nil
}

// InMemoryCredentialManager is an in-memory implementation of CredentialManager
type InMemoryCredentialManager struct {
	credentials map[string]map[string]interface{}
	mu          sync.RWMutex
}

// NewInMemoryCredentialManager creates a new in-memory credential manager
func NewInMemoryCredentialManager() *InMemoryCredentialManager {
	return &InMemoryCredentialManager{
		credentials: make(map[string]map[string]interface{}),
	}
}

// GetCredentials gets credentials for a connector
func (cm *InMemoryCredentialManager) GetCredentials(ctx context.Context, connectorID string) (map[string]interface{}, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	creds, exists := cm.credentials[connectorID]
	if !exists {
		return map[string]interface{}{}, nil
	}
	
	// Return a copy to prevent modification
	result := make(map[string]interface{})
	for k, v := range creds {
		result[k] = v
	}
	
	return result, nil
}

// StoreCredentials stores credentials for a connector
func (cm *InMemoryCredentialManager) StoreCredentials(ctx context.Context, connectorID string, credentials map[string]interface{}) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	// Store a copy to prevent modification
	result := make(map[string]interface{})
	for k, v := range credentials {
		result[k] = v
	}
	
	cm.credentials[connectorID] = result
	
	return nil
}

// DeleteCredentials deletes credentials for a connector
func (cm *InMemoryCredentialManager) DeleteCredentials(ctx context.Context, connectorID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	delete(cm.credentials, connectorID)
	
	return nil
}

// RotateCredentials rotates credentials for a connector
func (cm *InMemoryCredentialManager) RotateCredentials(ctx context.Context, connectorID string) (map[string]interface{}, error) {
	// This is just a stub - real implementation would depend on the connector type
	// and would actually rotate the credentials with the external service
	
	cm.mu.RLock()
	creds, exists := cm.credentials[connectorID]
	cm.mu.RUnlock()
	
	if !exists {
		return nil, ErrConnectorNotFound
	}
	
	// Return a copy of the existing credentials
	result := make(map[string]interface{})
	for k, v := range creds {
		result[k] = v
	}
	
	return result, nil
} 