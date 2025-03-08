package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"path"

	"github.com/go-chi/chi/v5"
	"github.com/yourusername/taskscheduler/pkg/cache"
)

// CacheConfig contains configuration for the cache middleware
type CacheConfig struct {
	Enabled            bool
	DefaultTTL         time.Duration
	CacheTagPrefix     string
	MethodsToCache     []string
	PathsToCache       []string
	PathsToExclude     []string
	QueryParamsToSkip  []string
	HeadersForKeyGen   []string
	StatusesToCache    []int
	StaleWhileRevalidate time.Duration
	StaleIfError        time.Duration
	VaryByHeader        []string
	VaryByParam         []string
	GetCacheKeyFn      func(*http.Request) string
}

// NewDefaultCacheConfig returns a default cache configuration
func NewDefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Enabled:        true,
		DefaultTTL:     time.Minute * 5,
		CacheTagPrefix: "api:",
		MethodsToCache: []string{http.MethodGet},
		PathsToCache:   []string{"/api/"},
		PathsToExclude: []string{"/api/health", "/api/metrics"},
		QueryParamsToSkip: []string{
			"api_key", 
			"token", 
			"jwt", 
			"timestamp", 
			"nonce",
		},
		HeadersForKeyGen: []string{
			"Accept", 
			"Accept-Encoding", 
			"Accept-Language",
		},
		StatusesToCache:    []int{http.StatusOK, http.StatusNotFound},
		StaleWhileRevalidate: time.Minute * 1,
		StaleIfError:        time.Hour * 24,
		VaryByHeader:        []string{"Accept", "Accept-Encoding"},
		VaryByParam:         []string{"limit", "offset", "page", "sort"},
	}
}

// cachedResponseWriter is a wrapper around http.ResponseWriter that captures the response
type cachedResponseWriter struct {
	http.ResponseWriter
	statusCode int
	buf        *bytes.Buffer
	header     http.Header
}

// newCachedResponseWriter creates a new cachedResponseWriter
func newCachedResponseWriter(w http.ResponseWriter) *cachedResponseWriter {
	return &cachedResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		buf:            &bytes.Buffer{},
		header:         make(http.Header),
	}
}

// WriteHeader captures the status code and delegates to the wrapped ResponseWriter
func (w *cachedResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	if code != http.StatusNotModified {
		w.ResponseWriter.WriteHeader(code)
	}
}

// Write captures the response body and delegates to the wrapped ResponseWriter
func (w *cachedResponseWriter) Write(b []byte) (int, error) {
	if w.statusCode != http.StatusNotModified {
		w.buf.Write(b)
		return w.ResponseWriter.Write(b)
	}
	return len(b), nil
}

// Header captures response headers and delegates to the wrapped ResponseWriter
func (w *cachedResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

// cacheResponse represents a cached HTTP response
type cacheResponse struct {
	Status  int         `json:"status"`
	Headers http.Header `json:"headers"`
	Body    []byte      `json:"body"`
}

// generateCacheKey generates a cache key from an HTTP request
func generateCacheKey(r *http.Request, config *CacheConfig) string {
	if config.GetCacheKeyFn != nil {
		return config.GetCacheKeyFn(r)
	}

	// Start with the method and path
	parts := []string{r.Method, r.URL.Path}

	// Add selected headers
	for _, header := range config.HeadersForKeyGen {
		if value := r.Header.Get(header); value != "" {
			parts = append(parts, fmt.Sprintf("%s=%s", header, value))
		}
	}

	// VaryByHeader
	for _, header := range config.VaryByHeader {
		if value := r.Header.Get(header); value != "" {
			parts = append(parts, fmt.Sprintf("h:%s=%s", header, value))
		}
	}

	// Add query parameters, sorted by key
	if r.URL.RawQuery != "" {
		queryParams := r.URL.Query()
		var queryKeys []string
		for key := range queryParams {
			// Skip excluded query parameters
			skip := false
			for _, excludeParam := range config.QueryParamsToSkip {
				if key == excludeParam {
					skip = true
					break
				}
			}
			if !skip {
				queryKeys = append(queryKeys, key)
			}
		}
		sort.Strings(queryKeys)

		// Check if this param is specifically in VaryByParam
		for _, param := range config.VaryByParam {
			if values, ok := queryParams[param]; ok {
				for _, value := range values {
					parts = append(parts, fmt.Sprintf("p:%s=%s", param, value))
				}
			}
		}

		// Add remaining query parameters
		for _, key := range queryKeys {
			if !contains(config.VaryByParam, key) {
				values := queryParams[key]
				sort.Strings(values)
				for _, value := range values {
					parts = append(parts, fmt.Sprintf("q:%s=%s", key, value))
				}
			}
		}
	}

	// Get route pattern if using chi router
	if chi, ok := r.Context().Value(chi.RouteCtxKey).(*chi.Context); ok && chi != nil {
		if pattern := chi.RoutePattern(); pattern != "" {
			parts = append(parts, fmt.Sprintf("route=%s", pattern))
		}
	}

	// Hash the key components
	hash := sha256.Sum256([]byte(strings.Join(parts, ";")))
	return hex.EncodeToString(hash[:])
}

// shouldCacheMethod checks if a method should be cached
func shouldCacheMethod(method string, config *CacheConfig) bool {
	for _, m := range config.MethodsToCache {
		if m == method {
			return true
		}
	}
	return false
}

// shouldCachePath checks if a path should be cached
func shouldCachePath(path string, config *CacheConfig) bool {
	// Check if path is in excluded paths
	for _, excludePath := range config.PathsToExclude {
		if strings.HasPrefix(path, excludePath) {
			return false
		}
	}

	// Check if path is in included paths
	for _, includePath := range config.PathsToCache {
		if strings.HasPrefix(path, includePath) {
			return true
		}
	}

	return false
}

// shouldCacheStatus checks if a status code should be cached
func shouldCacheStatus(status int, config *CacheConfig) bool {
	for _, s := range config.StatusesToCache {
		if s == status {
			return true
		}
	}
	return false
}

// contains checks if a string slice contains a value
func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

// getCacheTags generates cache tags based on the request
func getCacheTags(r *http.Request, config *CacheConfig) cache.CacheTags {
	tags := []string{
		fmt.Sprintf("%smethod:%s", config.CacheTagPrefix, r.Method),
		fmt.Sprintf("%spath:%s", config.CacheTagPrefix, path.Dir(r.URL.Path)),
	}

	// Add route pattern tags if using chi router
	if chi, ok := r.Context().Value(chi.RouteCtxKey).(*chi.Context); ok && chi != nil {
		if pattern := chi.RoutePattern(); pattern != "" {
			tags = append(tags, fmt.Sprintf("%sroute:%s", config.CacheTagPrefix, pattern))
		}
	}

	return tags
}

// CacheMiddleware creates a middleware for caching API responses
func CacheMiddleware(cacheClient cache.Cache, config *CacheConfig) func(http.Handler) http.Handler {
	if config == nil {
		config = NewDefaultCacheConfig()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if caching is disabled
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Skip if method is not cacheable
			if !shouldCacheMethod(r.Method, config) {
				next.ServeHTTP(w, r)
				return
			}

			// Skip if path is not cacheable
			if !shouldCachePath(r.URL.Path, config) {
				next.ServeHTTP(w, r)
				return
			}

			// Generate cache key
			cacheKey := cache.CacheKey(generateCacheKey(r, config))
			
			// Check if response is cached
			ctx := r.Context()
			cachedData, err := cacheClient.Get(ctx, cacheKey)
			if err == nil && cachedData != nil {
				// Set revalidation headers
				w.Header().Set("X-Cache", "HIT")
				
				// Deserialize cached response
				var response cacheResponse
				if err := json.Unmarshal(cachedData, &response); err == nil {
					// Copy headers from cached response
					for key, values := range response.Headers {
						for _, value := range values {
							w.Header().Add(key, value)
						}
					}
					
					// Set additional cache headers
					w.Header().Set("X-Cache-Key", string(cacheKey))
					
					// Write status and body
					w.WriteHeader(response.Status)
					w.Write(response.Body)
					return
				}
			}

			// Execute the request with capturing middleware
			crw := newCachedResponseWriter(w)
			crw.Header().Set("X-Cache", "MISS")
			crw.Header().Set("X-Cache-Key", string(cacheKey))
			
			next.ServeHTTP(crw, r)
			
			// Only cache successful responses (or ones explicitly in the configuration)
			if !shouldCacheStatus(crw.statusCode, config) {
				return
			}
			
			// Prepare the response for caching
			headers := http.Header{}
			for k, v := range crw.Header() {
				// Don't cache certain headers that should be generated per-request
				if !strings.HasPrefix(strings.ToLower(k), "x-cache") && 
				   k != "Set-Cookie" && 
				   k != "Date" && 
				   k != "Content-Length" {
					headers[k] = v
				}
			}

			response := cacheResponse{
				Status:  crw.statusCode,
				Headers: headers,
				Body:    crw.buf.Bytes(),
			}
			
			// Serialize and cache the response
			responseData, err := json.Marshal(response)
			if err == nil {
				// Store with tags for invalidation
				tags := getCacheTags(r, config)
				
				err = cacheClient.Set(ctx, &cache.Item{
					Key:        cacheKey,
					Value:      responseData,
					Expiration: config.DefaultTTL,
					Tags:       tags,
				})
				
				if err != nil {
					// Log error but continue processing
					fmt.Printf("Error caching response: %v\n", err)
				}
			}
		})
	}
}

// InvalidateCacheByTags invalidates cache entries based on tags
func InvalidateCacheByTags(cacheClient cache.Cache, tags ...string) error {
	ctx := context.Background()
	
	for _, tag := range tags {
		if err := cacheClient.DeleteByTag(ctx, tag); err != nil {
			return err
		}
	}
	
	return nil
}

// CacheControl sets cache control headers based on configuration
func CacheControl(duration time.Duration, staleWhileRevalidate, staleIfError time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set Cache-Control header
			directives := []string{
				fmt.Sprintf("max-age=%d", int(duration.Seconds())),
			}
			
			if staleWhileRevalidate > 0 {
				directives = append(directives, fmt.Sprintf("stale-while-revalidate=%d", int(staleWhileRevalidate.Seconds())))
			}
			
			if staleIfError > 0 {
				directives = append(directives, fmt.Sprintf("stale-if-error=%d", int(staleIfError.Seconds())))
			}
			
			w.Header().Set("Cache-Control", strings.Join(directives, ", "))
			
			// Execute next handler
			next.ServeHTTP(w, r)
		})
	}
} 