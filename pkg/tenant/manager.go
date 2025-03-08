package tenant

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Errors
var (
	ErrTenantNotFound     = errors.New("tenant not found")
	ErrTenantExists       = errors.New("tenant already exists")
	ErrTenantDisabled     = errors.New("tenant is disabled")
	ErrInvalidTenantID    = errors.New("invalid tenant ID")
	ErrPlanNotFound       = errors.New("subscription plan not found")
	ErrQuotaExceeded      = errors.New("resource quota exceeded")
	ErrOperationNotAllowed = errors.New("operation not allowed for tenant")
)

// TenantStatus represents the status of a tenant
type TenantStatus string

// Tenant statuses
const (
	TenantStatusActive   TenantStatus = "active"
	TenantStatusDisabled TenantStatus = "disabled"
	TenantStatusPending  TenantStatus = "pending"
	TenantStatusArchived TenantStatus = "archived"
)

// Tenant represents a multi-tenant entity
type Tenant struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	DisplayName    string        `json:"display_name"`
	Status         TenantStatus  `json:"status"`
	PlanID         string        `json:"plan_id"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
	ExpiresAt      *time.Time    `json:"expires_at,omitempty"`
	Settings       TenantSettings `json:"settings"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	ContactEmail   string        `json:"contact_email"`
	ResourceQuotas ResourceQuotas `json:"resource_quotas"`
	Usage          ResourceUsage  `json:"usage"`
}

// TenantSettings contains tenant-specific settings
type TenantSettings struct {
	EnabledFeatures   map[string]bool    `json:"enabled_features"`
	RateLimits        map[string]int     `json:"rate_limits"`
	RetentionPolicies map[string]int     `json:"retention_policies"` // in days
	EncryptionEnabled bool               `json:"encryption_enabled"`
	CustomDomain      string             `json:"custom_domain,omitempty"`
	UITheme           string             `json:"ui_theme,omitempty"`
	LogLevel          string             `json:"log_level"`
	Integrations      map[string]string  `json:"integrations,omitempty"`
	NotificationConfig map[string]bool   `json:"notification_config,omitempty"`
}

// ResourceQuotas defines resource limitations for a tenant
type ResourceQuotas struct {
	MaxTasks          int    `json:"max_tasks"`
	MaxWorkers        int    `json:"max_workers"`
	MaxConcurrentTasks int    `json:"max_concurrent_tasks"`
	MaxTaskMemoryMB   int    `json:"max_task_memory_mb"`
	MaxTaskCPU        int    `json:"max_task_cpu"`
	MaxStorageGB      int    `json:"max_storage_gb"`
	MaxBandwidthGB    int    `json:"max_bandwidth_gb"`
	MaxUsers          int    `json:"max_users"`
	MaxRetentionDays  int    `json:"max_retention_days"`
}

// ResourceUsage tracks a tenant's resource consumption
type ResourceUsage struct {
	TaskCount         int       `json:"task_count"`
	WorkerCount       int       `json:"worker_count"`
	ConcurrentTasks   int       `json:"concurrent_tasks"`
	StorageUsedGB     float64   `json:"storage_used_gb"`
	BandwidthUsedGB   float64   `json:"bandwidth_used_gb"`
	UserCount         int       `json:"user_count"`
	LastUpdated       time.Time `json:"last_updated"`
}

// SubscriptionPlan defines a tenant's subscription level
type SubscriptionPlan struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Price       float64       `json:"price"`
	Interval    string        `json:"interval"` // monthly, annual, etc.
	Features    []string      `json:"features"`
	Quotas      ResourceQuotas `json:"quotas"`
}

// TenantStore defines the interface for tenant storage
type TenantStore interface {
	GetTenant(tenantID string) (*Tenant, error)
	ListTenants() ([]*Tenant, error)
	CreateTenant(tenant *Tenant) error
	UpdateTenant(tenant *Tenant) error
	DeleteTenant(tenantID string) error
	GetPlan(planID string) (*SubscriptionPlan, error)
	ListPlans() ([]*SubscriptionPlan, error)
	UpdateTenantUsage(tenantID string, usage ResourceUsage) error
}

// DatabaseTenantStore implements TenantStore with a SQL database
type DatabaseTenantStore struct {
	db *sql.DB
}

// NewDatabaseTenantStore creates a new database tenant store
func NewDatabaseTenantStore(db *sql.DB) *DatabaseTenantStore {
	return &DatabaseTenantStore{
		db: db,
	}
}

// GetTenant retrieves a tenant by ID
func (dts *DatabaseTenantStore) GetTenant(tenantID string) (*Tenant, error) {
	// Implementation would use SQL queries to retrieve tenant data
	// This is a placeholder for the actual implementation
	return nil, fmt.Errorf("not implemented")
}

// ListTenants lists all tenants
func (dts *DatabaseTenantStore) ListTenants() ([]*Tenant, error) {
	// Implementation would use SQL queries to list all tenants
	// This is a placeholder for the actual implementation
	return nil, fmt.Errorf("not implemented")
}

// CreateTenant creates a new tenant
func (dts *DatabaseTenantStore) CreateTenant(tenant *Tenant) error {
	// Implementation would use SQL queries to create a tenant
	// This is a placeholder for the actual implementation
	return fmt.Errorf("not implemented")
}

// UpdateTenant updates a tenant
func (dts *DatabaseTenantStore) UpdateTenant(tenant *Tenant) error {
	// Implementation would use SQL queries to update a tenant
	// This is a placeholder for the actual implementation
	return fmt.Errorf("not implemented")
}

// DeleteTenant deletes a tenant
func (dts *DatabaseTenantStore) DeleteTenant(tenantID string) error {
	// Implementation would use SQL queries to delete a tenant
	// This is a placeholder for the actual implementation
	return fmt.Errorf("not implemented")
}

// GetPlan retrieves a subscription plan by ID
func (dts *DatabaseTenantStore) GetPlan(planID string) (*SubscriptionPlan, error) {
	// Implementation would use SQL queries to retrieve a plan
	// This is a placeholder for the actual implementation
	return nil, fmt.Errorf("not implemented")
}

// ListPlans lists all subscription plans
func (dts *DatabaseTenantStore) ListPlans() ([]*SubscriptionPlan, error) {
	// Implementation would use SQL queries to list all plans
	// This is a placeholder for the actual implementation
	return nil, fmt.Errorf("not implemented")
}

// UpdateTenantUsage updates a tenant's resource usage
func (dts *DatabaseTenantStore) UpdateTenantUsage(tenantID string, usage ResourceUsage) error {
	// Implementation would use SQL queries to update tenant usage
	// This is a placeholder for the actual implementation
	return fmt.Errorf("not implemented")
}

// InMemoryTenantStore implements TenantStore with in-memory storage
type InMemoryTenantStore struct {
	tenants map[string]*Tenant
	plans   map[string]*SubscriptionPlan
	mu      sync.RWMutex
}

// NewInMemoryTenantStore creates a new in-memory tenant store
func NewInMemoryTenantStore() *InMemoryTenantStore {
	return &InMemoryTenantStore{
		tenants: make(map[string]*Tenant),
		plans:   make(map[string]*SubscriptionPlan),
	}
}

// GetTenant retrieves a tenant by ID
func (imts *InMemoryTenantStore) GetTenant(tenantID string) (*Tenant, error) {
	imts.mu.RLock()
	defer imts.mu.RUnlock()

	tenant, exists := imts.tenants[tenantID]
	if !exists {
		return nil, ErrTenantNotFound
	}

	// Return a copy to prevent modification
	tenantCopy := *tenant
	return &tenantCopy, nil
}

// ListTenants lists all tenants
func (imts *InMemoryTenantStore) ListTenants() ([]*Tenant, error) {
	imts.mu.RLock()
	defer imts.mu.RUnlock()

	tenants := make([]*Tenant, 0, len(imts.tenants))
	for _, tenant := range imts.tenants {
		tenantCopy := *tenant
		tenants = append(tenants, &tenantCopy)
	}

	return tenants, nil
}

// CreateTenant creates a new tenant
func (imts *InMemoryTenantStore) CreateTenant(tenant *Tenant) error {
	imts.mu.Lock()
	defer imts.mu.Unlock()

	if _, exists := imts.tenants[tenant.ID]; exists {
		return ErrTenantExists
	}

	// Check if plan exists
	if _, exists := imts.plans[tenant.PlanID]; !exists {
		return ErrPlanNotFound
	}

	// Initialize timestamps
	now := time.Now()
	tenant.CreatedAt = now
	tenant.UpdatedAt = now
	tenant.Usage.LastUpdated = now

	// Create a copy to store
	tenantCopy := *tenant
	imts.tenants[tenant.ID] = &tenantCopy

	return nil
}

// UpdateTenant updates a tenant
func (imts *InMemoryTenantStore) UpdateTenant(tenant *Tenant) error {
	imts.mu.Lock()
	defer imts.mu.Unlock()

	if _, exists := imts.tenants[tenant.ID]; !exists {
		return ErrTenantNotFound
	}

	// Check if plan exists if it's being changed
	if _, exists := imts.plans[tenant.PlanID]; !exists {
		return ErrPlanNotFound
	}

	// Update timestamps
	tenant.UpdatedAt = time.Now()

	// Create a copy to store
	tenantCopy := *tenant
	imts.tenants[tenant.ID] = &tenantCopy

	return nil
}

// DeleteTenant deletes a tenant
func (imts *InMemoryTenantStore) DeleteTenant(tenantID string) error {
	imts.mu.Lock()
	defer imts.mu.Unlock()

	if _, exists := imts.tenants[tenantID]; !exists {
		return ErrTenantNotFound
	}

	delete(imts.tenants, tenantID)
	return nil
}

// GetPlan retrieves a subscription plan by ID
func (imts *InMemoryTenantStore) GetPlan(planID string) (*SubscriptionPlan, error) {
	imts.mu.RLock()
	defer imts.mu.RUnlock()

	plan, exists := imts.plans[planID]
	if !exists {
		return nil, ErrPlanNotFound
	}

	// Return a copy to prevent modification
	planCopy := *plan
	return &planCopy, nil
}

// ListPlans lists all subscription plans
func (imts *InMemoryTenantStore) ListPlans() ([]*SubscriptionPlan, error) {
	imts.mu.RLock()
	defer imts.mu.RUnlock()

	plans := make([]*SubscriptionPlan, 0, len(imts.plans))
	for _, plan := range imts.plans {
		planCopy := *plan
		plans = append(plans, &planCopy)
	}

	return plans, nil
}

// UpdateTenantUsage updates a tenant's resource usage
func (imts *InMemoryTenantStore) UpdateTenantUsage(tenantID string, usage ResourceUsage) error {
	imts.mu.Lock()
	defer imts.mu.Unlock()

	tenant, exists := imts.tenants[tenantID]
	if !exists {
		return ErrTenantNotFound
	}

	usage.LastUpdated = time.Now()
	tenant.Usage = usage

	return nil
}

// TenantManager manages multi-tenancy
type TenantManager struct {
	store      TenantStore
	cache      map[string]*Tenant
	cacheMu    sync.RWMutex
	cacheValid bool
}

// NewTenantManager creates a new tenant manager
func NewTenantManager(store TenantStore) *TenantManager {
	return &TenantManager{
		store:      store,
		cache:      make(map[string]*Tenant),
		cacheValid: false,
	}
}

// GetTenant retrieves a tenant by ID
func (tm *TenantManager) GetTenant(tenantID string) (*Tenant, error) {
	// Check cache first
	tm.cacheMu.RLock()
	if tm.cacheValid {
		if tenant, exists := tm.cache[tenantID]; exists {
			tm.cacheMu.RUnlock()
			return tenant, nil
		}
	}
	tm.cacheMu.RUnlock()

	// Fetch from store
	tenant, err := tm.store.GetTenant(tenantID)
	if err != nil {
		return nil, err
	}

	// Update cache
	tm.cacheMu.Lock()
	tm.cache[tenantID] = tenant
	tm.cacheMu.Unlock()

	return tenant, nil
}

// CreateTenant creates a new tenant
func (tm *TenantManager) CreateTenant(tenant *Tenant) error {
	err := tm.store.CreateTenant(tenant)
	if err != nil {
		return err
	}

	// Update cache
	tm.cacheMu.Lock()
	tm.cache[tenant.ID] = tenant
	tm.cacheMu.Unlock()

	return nil
}

// UpdateTenant updates a tenant
func (tm *TenantManager) UpdateTenant(tenant *Tenant) error {
	err := tm.store.UpdateTenant(tenant)
	if err != nil {
		return err
	}

	// Update cache
	tm.cacheMu.Lock()
	tm.cache[tenant.ID] = tenant
	tm.cacheMu.Unlock()

	return nil
}

// DeleteTenant deletes a tenant
func (tm *TenantManager) DeleteTenant(tenantID string) error {
	err := tm.store.DeleteTenant(tenantID)
	if err != nil {
		return err
	}

	// Update cache
	tm.cacheMu.Lock()
	delete(tm.cache, tenantID)
	tm.cacheMu.Unlock()

	return nil
}

// ListTenants lists all tenants
func (tm *TenantManager) ListTenants() ([]*Tenant, error) {
	// Check if cache is valid
	tm.cacheMu.RLock()
	if tm.cacheValid {
		tenants := make([]*Tenant, 0, len(tm.cache))
		for _, tenant := range tm.cache {
			tenants = append(tenants, tenant)
		}
		tm.cacheMu.RUnlock()
		return tenants, nil
	}
	tm.cacheMu.RUnlock()

	// Fetch from store
	tenants, err := tm.store.ListTenants()
	if err != nil {
		return nil, err
	}

	// Update cache
	tm.cacheMu.Lock()
	tm.cache = make(map[string]*Tenant)
	for _, tenant := range tenants {
		tm.cache[tenant.ID] = tenant
	}
	tm.cacheValid = true
	tm.cacheMu.Unlock()

	return tenants, nil
}

// GetPlan retrieves a subscription plan by ID
func (tm *TenantManager) GetPlan(planID string) (*SubscriptionPlan, error) {
	return tm.store.GetPlan(planID)
}

// ListPlans lists all subscription plans
func (tm *TenantManager) ListPlans() ([]*SubscriptionPlan, error) {
	return tm.store.ListPlans()
}

// UpdateTenantUsage updates a tenant's resource usage
func (tm *TenantManager) UpdateTenantUsage(tenantID string, usage ResourceUsage) error {
	err := tm.store.UpdateTenantUsage(tenantID, usage)
	if err != nil {
		return err
	}

	// Update cache if tenant exists in cache
	tm.cacheMu.Lock()
	if tenant, exists := tm.cache[tenantID]; exists {
		tenant.Usage = usage
	}
	tm.cacheMu.Unlock()

	return nil
}

// CheckResourceQuota checks if a tenant has exceeded a resource quota
func (tm *TenantManager) CheckResourceQuota(tenantID string, resourceType string, amount int) error {
	tenant, err := tm.GetTenant(tenantID)
	if err != nil {
		return err
	}

	if tenant.Status != TenantStatusActive {
		return ErrTenantDisabled
	}

	// Check quota based on resource type
	switch resourceType {
	case "tasks":
		if tenant.Usage.TaskCount+amount > tenant.ResourceQuotas.MaxTasks {
			return ErrQuotaExceeded
		}
	case "workers":
		if tenant.Usage.WorkerCount+amount > tenant.ResourceQuotas.MaxWorkers {
			return ErrQuotaExceeded
		}
	case "concurrent_tasks":
		if tenant.Usage.ConcurrentTasks+amount > tenant.ResourceQuotas.MaxConcurrentTasks {
			return ErrQuotaExceeded
		}
	case "storage":
		if tenant.Usage.StorageUsedGB+float64(amount) > float64(tenant.ResourceQuotas.MaxStorageGB) {
			return ErrQuotaExceeded
		}
	case "users":
		if tenant.Usage.UserCount+amount > tenant.ResourceQuotas.MaxUsers {
			return ErrQuotaExceeded
		}
	}

	return nil
}

// IsFeatureEnabled checks if a feature is enabled for a tenant
func (tm *TenantManager) IsFeatureEnabled(tenantID, feature string) (bool, error) {
	tenant, err := tm.GetTenant(tenantID)
	if err != nil {
		return false, err
	}

	if tenant.Status != TenantStatusActive {
		return false, ErrTenantDisabled
	}

	// Check if the feature is enabled
	if enabled, exists := tenant.Settings.EnabledFeatures[feature]; exists {
		return enabled, nil
	}

	// Feature not configured, assume disabled
	return false, nil
}

// GetTenantSetting gets a tenant setting value
func (tm *TenantManager) GetTenantSetting(tenantID, setting string) (interface{}, error) {
	tenant, err := tm.GetTenant(tenantID)
	if err != nil {
		return nil, err
	}

	// Check different setting types
	switch setting {
	case "encryption_enabled":
		return tenant.Settings.EncryptionEnabled, nil
	case "custom_domain":
		return tenant.Settings.CustomDomain, nil
	case "ui_theme":
		return tenant.Settings.UITheme, nil
	case "log_level":
		return tenant.Settings.LogLevel, nil
	}

	// Check rate limits
	if limit, exists := tenant.Settings.RateLimits[setting]; exists {
		return limit, nil
	}

	// Check retention policies
	if retention, exists := tenant.Settings.RetentionPolicies[setting]; exists {
		return retention, nil
	}

	// Check integrations
	if integration, exists := tenant.Settings.Integrations[setting]; exists {
		return integration, nil
	}

	return nil, fmt.Errorf("setting %s not found", setting)
}

// InvalidateCache invalidates the tenant cache
func (tm *TenantManager) InvalidateCache() {
	tm.cacheMu.Lock()
	tm.cacheValid = false
	tm.cacheMu.Unlock()
}

// CreateDefaultPlans creates default subscription plans
func (tm *TenantManager) CreateDefaultPlans() error {
	// Free plan
	freePlan := &SubscriptionPlan{
		ID:          "free",
		Name:        "Free",
		Description: "Basic plan for small projects",
		Price:       0,
		Interval:    "monthly",
		Features:    []string{"basic_task_scheduling", "web_dashboard", "api_access"},
		Quotas: ResourceQuotas{
			MaxTasks:          100,
			MaxWorkers:        2,
			MaxConcurrentTasks: 5,
			MaxTaskMemoryMB:   512,
			MaxTaskCPU:        1,
			MaxStorageGB:      1,
			MaxBandwidthGB:    5,
			MaxUsers:          2,
			MaxRetentionDays:  7,
		},
	}

	// Pro plan
	proPlan := &SubscriptionPlan{
		ID:          "pro",
		Name:        "Professional",
		Description: "Advanced features for teams",
		Price:       49.99,
		Interval:    "monthly",
		Features:    []string{"advanced_scheduling", "priority_execution", "workflow_templates", "api_access", "web_dashboard", "email_notifications", "advanced_monitoring"},
		Quotas: ResourceQuotas{
			MaxTasks:          1000,
			MaxWorkers:        10,
			MaxConcurrentTasks: 20,
			MaxTaskMemoryMB:   2048,
			MaxTaskCPU:        2,
			MaxStorageGB:      10,
			MaxBandwidthGB:    50,
			MaxUsers:          10,
			MaxRetentionDays:  30,
		},
	}

	// Enterprise plan
	enterprisePlan := &SubscriptionPlan{
		ID:          "enterprise",
		Name:        "Enterprise",
		Description: "Maximum scalability and security for large organizations",
		Price:       299.99,
		Interval:    "monthly",
		Features:    []string{"advanced_scheduling", "priority_execution", "workflow_templates", "custom_integrations", "dedicated_support", "sla", "advanced_security", "advanced_monitoring", "custom_reporting"},
		Quotas: ResourceQuotas{
			MaxTasks:          10000,
			MaxWorkers:        100,
			MaxConcurrentTasks: 200,
			MaxTaskMemoryMB:   8192,
			MaxTaskCPU:        8,
			MaxStorageGB:      100,
			MaxBandwidthGB:    500,
			MaxUsers:          100,
			MaxRetentionDays:  90,
		},
	}

	// Additional plans can be added here

	// Store plans in database
	plans := []*SubscriptionPlan{freePlan, proPlan, enterprisePlan}
	
	// For InMemoryTenantStore, directly add to the plans map
	if imts, ok := tm.store.(*InMemoryTenantStore); ok {
		imts.mu.Lock()
		for _, plan := range plans {
			imts.plans[plan.ID] = plan
		}
		imts.mu.Unlock()
		return nil
	}
	
	// For other stores, would need to implement this

	return nil
} 