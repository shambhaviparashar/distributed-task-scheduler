package auth

import (
	"errors"
	"fmt"
	"sync"
)

// Permission represents an action that can be performed on a resource
type Permission string

// Common permission constants
const (
	PermissionRead   Permission = "read"
	PermissionWrite  Permission = "write"
	PermissionCreate Permission = "create"
	PermissionUpdate Permission = "update"
	PermissionDelete Permission = "delete"
	PermissionAdmin  Permission = "admin"
	PermissionExecute Permission = "execute"
)

// ResourceType defines the type of resource
type ResourceType string

// Common resource types
const (
	ResourceTask      ResourceType = "task"
	ResourceWorkflow  ResourceType = "workflow"
	ResourceWorker    ResourceType = "worker"
	ResourceTemplate  ResourceType = "template"
	ResourceScheduler ResourceType = "scheduler"
	ResourceSystem    ResourceType = "system"
	ResourceTenant    ResourceType = "tenant"
	ResourceUser      ResourceType = "user"
	ResourceRole      ResourceType = "role"
)

// ResourceID represents the identifier of a specific resource
type ResourceID string

// WildcardID represents all resources of a type
const WildcardID ResourceID = "*"

// Resource represents a resource in the system
type Resource struct {
	Type ResourceType
	ID   ResourceID
}

// NewResource creates a new resource
func NewResource(resourceType ResourceType, id ResourceID) Resource {
	return Resource{
		Type: resourceType,
		ID:   id,
	}
}

// String returns a string representation of the resource
func (r Resource) String() string {
	return fmt.Sprintf("%s:%s", r.Type, r.ID)
}

// Effect defines whether a policy allows or denies access
type Effect string

// Effect constants
const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

// Policy defines access control for resources
type Policy struct {
	ID          string
	Description string
	Effect      Effect
	Actions     []Permission
	Resources   []Resource
	Conditions  map[string]interface{} // Optional conditions for policy evaluation
}

// NewPolicy creates a new policy
func NewPolicy(id, description string, effect Effect, actions []Permission, resources []Resource) *Policy {
	return &Policy{
		ID:          id,
		Description: description,
		Effect:      effect,
		Actions:     actions,
		Resources:   resources,
		Conditions:  make(map[string]interface{}),
	}
}

// Role represents a collection of policies
type Role struct {
	ID          string
	Name        string
	Description string
	Policies    []*Policy
}

// NewRole creates a new role
func NewRole(id, name, description string) *Role {
	return &Role{
		ID:          id,
		Name:        name,
		Description: description,
		Policies:    make([]*Policy, 0),
	}
}

// AddPolicy adds a policy to a role
func (r *Role) AddPolicy(policy *Policy) {
	r.Policies = append(r.Policies, policy)
}

// User represents a user in the RBAC system
type User struct {
	ID          string
	ExternalID  string // ID from the identity provider
	Username    string
	Email       string
	Roles       []*Role
	TenantID    string
	DirectGrants []*Policy // Policies directly assigned to user
}

// NewUser creates a new user
func NewUser(id, externalID, username, email, tenantID string) *User {
	return &User{
		ID:          id,
		ExternalID:  externalID,
		Username:    username,
		Email:       email,
		Roles:       make([]*Role, 0),
		TenantID:    tenantID,
		DirectGrants: make([]*Policy, 0),
	}
}

// AssignRole assigns a role to a user
func (u *User) AssignRole(role *Role) {
	u.Roles = append(u.Roles, role)
}

// UnassignRole removes a role from a user
func (u *User) UnassignRole(roleID string) bool {
	for i, role := range u.Roles {
		if role.ID == roleID {
			u.Roles = append(u.Roles[:i], u.Roles[i+1:]...)
			return true
		}
	}
	return false
}

// GrantPolicy directly grants a policy to a user
func (u *User) GrantPolicy(policy *Policy) {
	u.DirectGrants = append(u.DirectGrants, policy)
}

// RevokePolicy removes a direct policy grant from a user
func (u *User) RevokePolicy(policyID string) bool {
	for i, policy := range u.DirectGrants {
		if policy.ID == policyID {
			u.DirectGrants = append(u.DirectGrants[:i], u.DirectGrants[i+1:]...)
			return true
		}
	}
	return false
}

// RBACManager manages RBAC resources
type RBACManager struct {
	roles    map[string]*Role
	policies map[string]*Policy
	users    map[string]*User
	mu       sync.RWMutex
}

// NewRBACManager creates a new RBAC manager
func NewRBACManager() *RBACManager {
	return &RBACManager{
		roles:    make(map[string]*Role),
		policies: make(map[string]*Policy),
		users:    make(map[string]*User),
	}
}

// RegisterRole registers a role
func (rm *RBACManager) RegisterRole(role *Role) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.roles[role.ID]; exists {
		return fmt.Errorf("role with ID %s already exists", role.ID)
	}

	rm.roles[role.ID] = role
	return nil
}

// GetRole returns a role by ID
func (rm *RBACManager) GetRole(id string) (*Role, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	role, exists := rm.roles[id]
	if !exists {
		return nil, fmt.Errorf("role with ID %s not found", id)
	}

	return role, nil
}

// UpdateRole updates a role
func (rm *RBACManager) UpdateRole(role *Role) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.roles[role.ID]; !exists {
		return fmt.Errorf("role with ID %s not found", role.ID)
	}

	rm.roles[role.ID] = role
	return nil
}

// DeleteRole deletes a role
func (rm *RBACManager) DeleteRole(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.roles[id]; !exists {
		return fmt.Errorf("role with ID %s not found", id)
	}

	delete(rm.roles, id)
	return nil
}

// RegisterPolicy registers a policy
func (rm *RBACManager) RegisterPolicy(policy *Policy) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.policies[policy.ID]; exists {
		return fmt.Errorf("policy with ID %s already exists", policy.ID)
	}

	rm.policies[policy.ID] = policy
	return nil
}

// GetPolicy returns a policy by ID
func (rm *RBACManager) GetPolicy(id string) (*Policy, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	policy, exists := rm.policies[id]
	if !exists {
		return nil, fmt.Errorf("policy with ID %s not found", id)
	}

	return policy, nil
}

// UpdatePolicy updates a policy
func (rm *RBACManager) UpdatePolicy(policy *Policy) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.policies[policy.ID]; !exists {
		return fmt.Errorf("policy with ID %s not found", policy.ID)
	}

	rm.policies[policy.ID] = policy
	return nil
}

// DeletePolicy deletes a policy
func (rm *RBACManager) DeletePolicy(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.policies[id]; !exists {
		return fmt.Errorf("policy with ID %s not found", id)
	}

	delete(rm.policies, id)
	return nil
}

// RegisterUser registers a user
func (rm *RBACManager) RegisterUser(user *User) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.users[user.ID]; exists {
		return fmt.Errorf("user with ID %s already exists", user.ID)
	}

	rm.users[user.ID] = user
	return nil
}

// GetUser returns a user by ID
func (rm *RBACManager) GetUser(id string) (*User, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	user, exists := rm.users[id]
	if !exists {
		return nil, fmt.Errorf("user with ID %s not found", id)
	}

	return user, nil
}

// GetUserByExternalID returns a user by external ID
func (rm *RBACManager) GetUserByExternalID(externalID string) (*User, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	for _, user := range rm.users {
		if user.ExternalID == externalID {
			return user, nil
		}
	}

	return nil, fmt.Errorf("user with external ID %s not found", externalID)
}

// UpdateUser updates a user
func (rm *RBACManager) UpdateUser(user *User) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.users[user.ID]; !exists {
		return fmt.Errorf("user with ID %s not found", user.ID)
	}

	rm.users[user.ID] = user
	return nil
}

// DeleteUser deletes a user
func (rm *RBACManager) DeleteUser(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.users[id]; !exists {
		return fmt.Errorf("user with ID %s not found", id)
	}

	delete(rm.users, id)
	return nil
}

// AssignRoleToUser assigns a role to a user
func (rm *RBACManager) AssignRoleToUser(userID, roleID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	user, exists := rm.users[userID]
	if !exists {
		return fmt.Errorf("user with ID %s not found", userID)
	}

	role, exists := rm.roles[roleID]
	if !exists {
		return fmt.Errorf("role with ID %s not found", roleID)
	}

	// Check if role is already assigned
	for _, r := range user.Roles {
		if r.ID == roleID {
			return nil // Role already assigned
		}
	}

	user.AssignRole(role)
	return nil
}

// UnassignRoleFromUser removes a role from a user
func (rm *RBACManager) UnassignRoleFromUser(userID, roleID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	user, exists := rm.users[userID]
	if !exists {
		return fmt.Errorf("user with ID %s not found", userID)
	}

	if !user.UnassignRole(roleID) {
		return fmt.Errorf("role with ID %s not assigned to user", roleID)
	}

	return nil
}

// GrantPolicyToUser directly grants a policy to a user
func (rm *RBACManager) GrantPolicyToUser(userID, policyID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	user, exists := rm.users[userID]
	if !exists {
		return fmt.Errorf("user with ID %s not found", userID)
	}

	policy, exists := rm.policies[policyID]
	if !exists {
		return fmt.Errorf("policy with ID %s not found", policyID)
	}

	// Check if policy is already granted
	for _, p := range user.DirectGrants {
		if p.ID == policyID {
			return nil // Policy already granted
		}
	}

	user.GrantPolicy(policy)
	return nil
}

// RevokePolicyFromUser removes a direct policy grant from a user
func (rm *RBACManager) RevokePolicyFromUser(userID, policyID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	user, exists := rm.users[userID]
	if !exists {
		return fmt.Errorf("user with ID %s not found", userID)
	}

	if !user.RevokePolicy(policyID) {
		return fmt.Errorf("policy with ID %s not directly granted to user", policyID)
	}

	return nil
}

// AuthorizationRequest contains the parameters for an authorization check
type AuthorizationRequest struct {
	UserID       string
	Action       Permission
	ResourceType ResourceType
	ResourceID   ResourceID
	TenantID     string
	Context      map[string]interface{}
}

// AuthorizationResult contains the result of an authorization check
type AuthorizationResult struct {
	Allowed        bool
	DenyingPolicy  *Policy
	AllowingPolicy *Policy
	Reason         string
}

// IsAuthorized checks if a user is authorized to perform an action
func (rm *RBACManager) IsAuthorized(req AuthorizationRequest) (*AuthorizationResult, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	user, exists := rm.users[req.UserID]
	if !exists {
		return &AuthorizationResult{
			Allowed: false,
			Reason:  fmt.Sprintf("user with ID %s not found", req.UserID),
		}, nil
	}

	// Default to deny
	result := &AuthorizationResult{
		Allowed: false,
		Reason:  "no applicable policies found",
	}

	// First check explicit denies (they override allows)
	if denyPolicy := rm.checkDenyPolicies(user, req); denyPolicy != nil {
		result.DenyingPolicy = denyPolicy
		result.Reason = fmt.Sprintf("explicitly denied by policy %s", denyPolicy.ID)
		return result, nil
	}

	// Then check for allows
	if allowPolicy := rm.checkAllowPolicies(user, req); allowPolicy != nil {
		result.Allowed = true
		result.AllowingPolicy = allowPolicy
		result.Reason = fmt.Sprintf("allowed by policy %s", allowPolicy.ID)
		return result, nil
	}

	return result, nil
}

// checkDenyPolicies checks for explicit deny policies
func (rm *RBACManager) checkDenyPolicies(user *User, req AuthorizationRequest) *Policy {
	// Check direct grants first
	for _, policy := range user.DirectGrants {
		if policy.Effect == EffectDeny && rm.policyMatches(policy, req) {
			return policy
		}
	}

	// Then check roles
	for _, role := range user.Roles {
		for _, policy := range role.Policies {
			if policy.Effect == EffectDeny && rm.policyMatches(policy, req) {
				return policy
			}
		}
	}

	return nil
}

// checkAllowPolicies checks for allow policies
func (rm *RBACManager) checkAllowPolicies(user *User, req AuthorizationRequest) *Policy {
	// Check direct grants first
	for _, policy := range user.DirectGrants {
		if policy.Effect == EffectAllow && rm.policyMatches(policy, req) {
			return policy
		}
	}

	// Then check roles
	for _, role := range user.Roles {
		for _, policy := range role.Policies {
			if policy.Effect == EffectAllow && rm.policyMatches(policy, req) {
				return policy
			}
		}
	}

	return nil
}

// policyMatches checks if a policy matches a request
func (rm *RBACManager) policyMatches(policy *Policy, req AuthorizationRequest) bool {
	// Check if action is in the policy
	actionMatch := false
	for _, action := range policy.Actions {
		if action == req.Action || action == "all" {
			actionMatch = true
			break
		}
	}
	if !actionMatch {
		return false
	}

	// Check if resource matches
	resourceMatch := false
	for _, resource := range policy.Resources {
		if (resource.Type == req.ResourceType || resource.Type == "*") &&
			(resource.ID == req.ResourceID || resource.ID == WildcardID) {
			resourceMatch = true
			break
		}
	}
	if !resourceMatch {
		return false
	}

	// Check conditions if present
	if len(policy.Conditions) > 0 {
		// Tenant check is a common condition
		if tenantID, ok := policy.Conditions["tenant_id"]; ok {
			if tenantStr, ok := tenantID.(string); ok {
				if tenantStr != req.TenantID && tenantStr != "*" {
					return false
				}
			}
		}

		// Additional condition checks would go here
		// This would be extended with a more sophisticated condition evaluator
	}

	return true
}

// CreateDefaultRoles creates default roles for the system
func (rm *RBACManager) CreateDefaultRoles() error {
	// Admin role - full access to everything
	adminRole := NewRole("admin", "Administrator", "Full system access")
	adminPolicy := NewPolicy(
		"admin-full-access",
		"Full access to all resources",
		EffectAllow,
		[]Permission{PermissionAdmin, PermissionRead, PermissionWrite, PermissionCreate, PermissionUpdate, PermissionDelete, PermissionExecute},
		[]Resource{NewResource("*", WildcardID)},
	)
	adminRole.AddPolicy(adminPolicy)
	
	if err := rm.RegisterPolicy(adminPolicy); err != nil {
		return err
	}
	if err := rm.RegisterRole(adminRole); err != nil {
		return err
	}
	
	// Operator role - can manage but not create/delete
	operatorRole := NewRole("operator", "Operator", "System operation access")
	operatorPolicy := NewPolicy(
		"operator-access",
		"Can operate the system but not create/delete resources",
		EffectAllow,
		[]Permission{PermissionRead, PermissionUpdate, PermissionExecute},
		[]Resource{
			NewResource(ResourceTask, WildcardID),
			NewResource(ResourceWorkflow, WildcardID),
			NewResource(ResourceWorker, WildcardID),
			NewResource(ResourceScheduler, WildcardID),
		},
	)
	operatorRole.AddPolicy(operatorPolicy)
	
	if err := rm.RegisterPolicy(operatorPolicy); err != nil {
		return err
	}
	if err := rm.RegisterRole(operatorRole); err != nil {
		return err
	}
	
	// User role - basic user access
	userRole := NewRole("user", "User", "Standard user access")
	userTaskPolicy := NewPolicy(
		"user-task-access",
		"Standard task access",
		EffectAllow,
		[]Permission{PermissionRead, PermissionCreate, PermissionUpdate, PermissionExecute},
		[]Resource{NewResource(ResourceTask, WildcardID)},
	)
	userWorkflowPolicy := NewPolicy(
		"user-workflow-access",
		"Standard workflow access",
		EffectAllow,
		[]Permission{PermissionRead, PermissionExecute},
		[]Resource{NewResource(ResourceWorkflow, WildcardID)},
	)
	userRole.AddPolicy(userTaskPolicy)
	userRole.AddPolicy(userWorkflowPolicy)
	
	if err := rm.RegisterPolicy(userTaskPolicy); err != nil {
		return err
	}
	if err := rm.RegisterPolicy(userWorkflowPolicy); err != nil {
		return err
	}
	if err := rm.RegisterRole(userRole); err != nil {
		return err
	}
	
	// Read-only role
	readOnlyRole := NewRole("readonly", "Read Only", "Read-only access")
	readOnlyPolicy := NewPolicy(
		"readonly-access",
		"Read-only access to all resources",
		EffectAllow,
		[]Permission{PermissionRead},
		[]Resource{NewResource("*", WildcardID)},
	)
	readOnlyRole.AddPolicy(readOnlyPolicy)
	
	if err := rm.RegisterPolicy(readOnlyPolicy); err != nil {
		return err
	}
	if err := rm.RegisterRole(readOnlyRole); err != nil {
		return err
	}
	
	return nil
} 