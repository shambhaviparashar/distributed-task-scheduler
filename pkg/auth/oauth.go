package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OAuthProvider represents an OAuth 2.0 provider configuration
type OAuthProvider struct {
	ProviderName  string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	Scopes        []string
	DiscoveryURL  string
	provider      *oidc.Provider
	verifier      *oidc.IDTokenVerifier
	oauthConfig   *oauth2.Config
	initialized   bool
	stateStore    *StateStore
	mu            sync.RWMutex
}

// StateStore is used to store and validate OAuth state parameters
type StateStore struct {
	states map[string]time.Time
	mu     sync.RWMutex
}

// NewStateStore creates a new state store for OAuth flow
func NewStateStore() *StateStore {
	return &StateStore{
		states: make(map[string]time.Time),
	}
}

// GenerateState generates a new state parameter and stores it
func (s *StateStore) GenerateState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	state := base64.StdEncoding.EncodeToString(b)
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Store state with expiration time (10 minutes)
	s.states[state] = time.Now().Add(10 * time.Minute)
	
	// Clean up expired states
	for k, v := range s.states {
		if time.Now().After(v) {
			delete(s.states, k)
		}
	}
	
	return state, nil
}

// ValidateState validates a state parameter
func (s *StateStore) ValidateState(state string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	expiry, exists := s.states[state]
	if !exists {
		return false
	}
	
	// Check if state is still valid
	if time.Now().After(expiry) {
		return false
	}
	
	return true
}

// RemoveState removes a state from the store once used
func (s *StateStore) RemoveState(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	delete(s.states, state)
}

// NewOAuthProvider creates a new OAuth provider
func NewOAuthProvider(providerName, clientID, clientSecret, redirectURL, discoveryURL string, scopes []string) *OAuthProvider {
	if scopes == nil {
		scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}
	
	return &OAuthProvider{
		ProviderName:  providerName,
		ClientID:      clientID,
		ClientSecret:  clientSecret,
		RedirectURL:   redirectURL,
		Scopes:        scopes,
		DiscoveryURL:  discoveryURL,
		stateStore:    NewStateStore(),
	}
}

// Initialize initializes the OAuth provider
func (p *OAuthProvider) Initialize(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.initialized {
		return nil
	}
	
	provider, err := oidc.NewProvider(ctx, p.DiscoveryURL)
	if err != nil {
		return fmt.Errorf("failed to initialize OIDC provider: %w", err)
	}
	
	p.provider = provider
	p.verifier = provider.Verifier(&oidc.Config{ClientID: p.ClientID})
	
	p.oauthConfig = &oauth2.Config{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
		RedirectURL:  p.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       p.Scopes,
	}
	
	p.initialized = true
	return nil
}

// GetAuthURL returns the URL to redirect the user to for authentication
func (p *OAuthProvider) GetAuthURL(ctx context.Context) (string, string, error) {
	if !p.initialized {
		return "", "", errors.New("provider not initialized")
	}
	
	state, err := p.stateStore.GenerateState()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate state: %w", err)
	}
	
	authURL := p.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)
	return authURL, state, nil
}

// UserInfo contains basic user information from the ID token
type UserInfo struct {
	Subject     string   `json:"sub"`
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	GivenName   string   `json:"given_name"`
	FamilyName  string   `json:"family_name"`
	Picture     string   `json:"picture"`
	Groups      []string `json:"groups"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	Tenant      string   `json:"tenant"`
	Raw         map[string]interface{}
}

// ExchangeCode exchanges an authorization code for tokens and validates the ID token
func (p *OAuthProvider) ExchangeCode(ctx context.Context, code, state string) (*oauth2.Token, *UserInfo, error) {
	if !p.initialized {
		return nil, nil, errors.New("provider not initialized")
	}
	
	// Validate state parameter
	if !p.stateStore.ValidateState(state) {
		return nil, nil, errors.New("invalid OAuth state parameter")
	}
	
	// Remove the state now that it's been used
	p.stateStore.RemoveState(state)
	
	// Exchange code for token
	oauth2Token, err := p.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	
	// Extract and verify the ID token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, nil, errors.New("no id_token in token response")
	}
	
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to verify ID token: %w", err)
	}
	
	// Parse claims from ID token
	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, nil, fmt.Errorf("failed to parse claims: %w", err)
	}
	
	// Build user info from claims
	userInfo := &UserInfo{
		Subject: idToken.Subject,
		Raw:     claims,
	}
	
	// Extract standard claims
	if email, ok := claims["email"].(string); ok {
		userInfo.Email = email
	}
	if name, ok := claims["name"].(string); ok {
		userInfo.Name = name
	}
	if givenName, ok := claims["given_name"].(string); ok {
		userInfo.GivenName = givenName
	}
	if familyName, ok := claims["family_name"].(string); ok {
		userInfo.FamilyName = familyName
	}
	if picture, ok := claims["picture"].(string); ok {
		userInfo.Picture = picture
	}
	
	// Extract groups/roles if available (provider-specific)
	if groups, ok := claims["groups"].([]interface{}); ok {
		for _, g := range groups {
			if group, ok := g.(string); ok {
				userInfo.Groups = append(userInfo.Groups, group)
			}
		}
	}
	
	if roles, ok := claims["roles"].([]interface{}); ok {
		for _, r := range roles {
			if role, ok := r.(string); ok {
				userInfo.Roles = append(userInfo.Roles, role)
			}
		}
	}
	
	if permissions, ok := claims["permissions"].([]interface{}); ok {
		for _, p := range permissions {
			if permission, ok := p.(string); ok {
				userInfo.Permissions = append(userInfo.Permissions, permission)
			}
		}
	}
	
	// Extract tenant information for multi-tenancy
	if tenant, ok := claims["tenant"].(string); ok {
		userInfo.Tenant = tenant
	}
	
	return oauth2Token, userInfo, nil
}

// ValidateToken validates an ID token and returns the user info
func (p *OAuthProvider) ValidateToken(ctx context.Context, tokenString string) (*UserInfo, error) {
	if !p.initialized {
		return nil, errors.New("provider not initialized")
	}
	
	idToken, err := p.verifier.Verify(ctx, tokenString)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}
	
	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}
	
	// Build user info from claims
	userInfo := &UserInfo{
		Subject: idToken.Subject,
		Raw:     claims,
	}
	
	// Extract standard claims
	if email, ok := claims["email"].(string); ok {
		userInfo.Email = email
	}
	if name, ok := claims["name"].(string); ok {
		userInfo.Name = name
	}
	if givenName, ok := claims["given_name"].(string); ok {
		userInfo.GivenName = givenName
	}
	if familyName, ok := claims["family_name"].(string); ok {
		userInfo.FamilyName = familyName
	}
	if picture, ok := claims["picture"].(string); ok {
		userInfo.Picture = picture
	}
	
	// Extract groups/roles
	if groups, ok := claims["groups"].([]interface{}); ok {
		for _, g := range groups {
			if group, ok := g.(string); ok {
				userInfo.Groups = append(userInfo.Groups, group)
			}
		}
	}
	
	if roles, ok := claims["roles"].([]interface{}); ok {
		for _, r := range roles {
			if role, ok := r.(string); ok {
				userInfo.Roles = append(userInfo.Roles, role)
			}
		}
	}
	
	if permissions, ok := claims["permissions"].([]interface{}); ok {
		for _, p := range permissions {
			if permission, ok := p.(string); ok {
				userInfo.Permissions = append(userInfo.Permissions, permission)
			}
		}
	}
	
	if tenant, ok := claims["tenant"].(string); ok {
		userInfo.Tenant = tenant
	}
	
	return userInfo, nil
}

// UserInfoFromToken extracts user info from token using userinfo endpoint
func (p *OAuthProvider) UserInfoFromToken(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	if !p.initialized {
		return nil, errors.New("provider not initialized")
	}
	
	// Create client with token
	client := p.oauthConfig.Client(ctx, token)
	
	// Get user info from the provider's userinfo endpoint
	userInfoURL := p.provider.Endpoint().UserInfoURL
	resp, err := client.Get(userInfoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get userinfo: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo request failed: %s", resp.Status)
	}
	
	var userInfo UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse userinfo response: %w", err)
	}
	
	return &userInfo, nil
}

// OAuthManager manages multiple OAuth providers
type OAuthManager struct {
	providers map[string]*OAuthProvider
	mu        sync.RWMutex
}

// NewOAuthManager creates a new OAuth manager
func NewOAuthManager() *OAuthManager {
	return &OAuthManager{
		providers: make(map[string]*OAuthProvider),
	}
}

// RegisterProvider registers an OAuth provider
func (m *OAuthManager) RegisterProvider(provider *OAuthProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.providers[provider.ProviderName] = provider
}

// GetProvider returns a provider by name
func (m *OAuthManager) GetProvider(name string) (*OAuthProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	provider, ok := m.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", name)
	}
	
	return provider, nil
}

// InitializeAll initializes all registered providers
func (m *OAuthManager) InitializeAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for _, provider := range m.providers {
		if err := provider.Initialize(ctx); err != nil {
			return fmt.Errorf("failed to initialize provider %s: %w", provider.ProviderName, err)
		}
	}
	
	return nil
} 