package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// EnterpriseConfig represents the full enterprise configuration
type EnterpriseConfig struct {
	General        GeneralConfig        `yaml:"general"`
	Security       SecurityConfig       `yaml:"security"`
	Tenancy        TenancyConfig        `yaml:"tenancy"`
	HighAvailability HAConfig           `yaml:"high_availability"`
	Integrations   IntegrationsConfig   `yaml:"integrations"`
	Compliance     ComplianceConfig     `yaml:"compliance"`
	Observability  ObservabilityConfig  `yaml:"observability"`
	Recovery       RecoveryConfig       `yaml:"recovery"`
}

// GeneralConfig contains general configuration settings
type GeneralConfig struct {
	Environment string `yaml:"environment"`
	InstanceID  string `yaml:"instance_id"`
	Version     string `yaml:"version"`
	LogLevel    string `yaml:"log_level"`
	DataDir     string `yaml:"data_dir"`
	TempDir     string `yaml:"temp_dir"`
}

// SecurityConfig contains security-related configuration
type SecurityConfig struct {
	OAuth            OAuthConfig         `yaml:"oauth"`
	RBAC             RBACConfig          `yaml:"rbac"`
	DataEncryption   EncryptionConfig    `yaml:"data_encryption"`
	AuditLogging     AuditConfig         `yaml:"audit_logging"`
	NetworkSecurity  NetworkSecurityConfig `yaml:"network_security"`
	Secrets          SecretsConfig       `yaml:"secrets"`
}

// OAuthConfig configures OAuth integration
type OAuthConfig struct {
	Enabled      bool              `yaml:"enabled"`
	Providers    []ProviderConfig  `yaml:"providers"`
	DefaultProvider string         `yaml:"default_provider"`
	SessionTimeout time.Duration   `yaml:"session_timeout"`
	JWTSigningKey  string          `yaml:"jwt_signing_key"`
	CookieSecure   bool            `yaml:"cookie_secure"`
	CookieDomain   string          `yaml:"cookie_domain"`
}

// ProviderConfig configures an OAuth provider
type ProviderConfig struct {
	Name         string   `yaml:"name"`
	Type         string   `yaml:"type"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	RedirectURL  string   `yaml:"redirect_url"`
	AuthURL      string   `yaml:"auth_url"`
	TokenURL     string   `yaml:"token_url"`
	UserInfoURL  string   `yaml:"userinfo_url"`
	Scopes       []string `yaml:"scopes"`
	DiscoveryURL string   `yaml:"discovery_url"`
}

// RBACConfig configures RBAC settings
type RBACConfig struct {
	Enabled        bool   `yaml:"enabled"`
	DefaultUserRole string `yaml:"default_user_role"`
	RolesFile      string `yaml:"roles_file"`
	PoliciesFile   string `yaml:"policies_file"`
	CacheTimeout   time.Duration `yaml:"cache_timeout"`
}

// EncryptionConfig configures data encryption
type EncryptionConfig struct {
	Enabled      bool   `yaml:"enabled"`
	KeyEnvVar    string `yaml:"key_env_var"`
	KeyFile      string `yaml:"key_file"`
	KeyRotationInterval time.Duration `yaml:"key_rotation_interval"`
	Algorithm    string `yaml:"algorithm"`
}

// AuditConfig configures audit logging
type AuditConfig struct {
	Enabled      bool   `yaml:"enabled"`
	LogFile      string `yaml:"log_file"`
	MaxFileSize  int    `yaml:"max_file_size_mb"`
	MaxBackups   int    `yaml:"max_backups"`
	MaxAge       int    `yaml:"max_age_days"`
	Compress     bool   `yaml:"compress"`
	ExternalSink string `yaml:"external_sink"`
}

// NetworkSecurityConfig contains network security settings
type NetworkSecurityConfig struct {
	TLS              TLSConfig `yaml:"tls"`
	EnableCORS       bool      `yaml:"enable_cors"`
	CORSAllowedOrigins []string `yaml:"cors_allowed_origins"`
	RateLimiting     bool      `yaml:"rate_limiting"`
	RequestsPerMinute int       `yaml:"requests_per_minute"`
	IPAllowlist      []string  `yaml:"ip_allowlist"`
	IPBlocklist      []string  `yaml:"ip_blocklist"`
}

// TLSConfig configures TLS settings
type TLSConfig struct {
	Enabled      bool   `yaml:"enabled"`
	CertFile     string `yaml:"cert_file"`
	KeyFile      string `yaml:"key_file"`
	MinVersion   string `yaml:"min_version"`
	MaxVersion   string `yaml:"max_version"`
	CipherSuites []string `yaml:"cipher_suites"`
}

// SecretsConfig configures secrets management
type SecretsConfig struct {
	Provider      string `yaml:"provider"`
	VaultAddress  string `yaml:"vault_address"`
	VaultToken    string `yaml:"vault_token"`
	AWSRegion     string `yaml:"aws_region"`
	GCPProjectID  string `yaml:"gcp_project_id"`
	AzureVaultURL string `yaml:"azure_vault_url"`
	KeyPrefix     string `yaml:"key_prefix"`
}

// TenancyConfig configures multi-tenancy support
type TenancyConfig struct {
	Enabled              bool   `yaml:"enabled"`
	IsolationMode        string `yaml:"isolation_mode"` // shared, dedicated, hybrid
	DatabaseMode         string `yaml:"database_mode"`  // shared, isolated, schema
	DefaultPlan          string `yaml:"default_plan"`
	TenantOnboarding     bool   `yaml:"tenant_onboarding"`
	TenantOffboarding    bool   `yaml:"tenant_offboarding"`
	AutoProvision        bool   `yaml:"auto_provision"`
	ResourceQuotaEnabled bool   `yaml:"resource_quota_enabled"`
	CachePerTenant       bool   `yaml:"cache_per_tenant"`
	PlansFile            string `yaml:"plans_file"`
}

// HAConfig configures high availability settings
type HAConfig struct {
	Enabled          bool   `yaml:"enabled"`
	Mode             string `yaml:"mode"`              // active-passive, active-active
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
	ElectionTimeout  time.Duration `yaml:"election_timeout"`
	ClusterID        string `yaml:"cluster_id"`
	ClusterName      string `yaml:"cluster_name"`
	StorageType      string `yaml:"storage_type"`      // redis, etcd, consul, postgres
	StorageEndpoint  string `yaml:"storage_endpoint"`
	LeadershipLock   string `yaml:"leadership_lock"`
	NodeTypes        []string `yaml:"node_types"`
	ZoneAwareness    bool   `yaml:"zone_awareness"`
	RegionAwareness  bool   `yaml:"region_awareness"`
}

// IntegrationsConfig configures integration connectors
type IntegrationsConfig struct {
	Enabled      bool                   `yaml:"enabled"`
	ConnectorsDir string                `yaml:"connectors_dir"`
	Connectors   []ConnectorConfig      `yaml:"connectors"`
	RateLimits   GlobalRateLimitConfig  `yaml:"rate_limits"`
	Timeouts     ConnectorTimeoutConfig `yaml:"timeouts"`
}

// ConnectorConfig configures a specific connector
type ConnectorConfig struct {
	ID          string                 `yaml:"id"`
	Name        string                 `yaml:"name"`
	Type        string                 `yaml:"type"`
	Enabled     bool                   `yaml:"enabled"`
	Settings    map[string]interface{} `yaml:"settings"`
	CredentialsFile string             `yaml:"credentials_file"`
}

// GlobalRateLimitConfig configures global rate limits for connectors
type GlobalRateLimitConfig struct {
	Enabled           bool  `yaml:"enabled"`
	DefaultRPM        int   `yaml:"default_rpm"`
	DefaultBurstSize  int   `yaml:"default_burst_size"`
	MaxRetries        int   `yaml:"max_retries"`
	RetryBaseInterval time.Duration `yaml:"retry_base_interval"`
}

// ConnectorTimeoutConfig configures timeouts for connectors
type ConnectorTimeoutConfig struct {
	Connect     time.Duration `yaml:"connect"`
	Read        time.Duration `yaml:"read"`
	Write       time.Duration `yaml:"write"`
	IdleTimeout time.Duration `yaml:"idle_timeout"`
}

// ComplianceConfig configures compliance features
type ComplianceConfig struct {
	DataRetention  DataRetentionConfig `yaml:"data_retention"`
	Anonymization  bool                `yaml:"anonymization"`
	PIIProtection  bool                `yaml:"pii_protection"`
	PIIPatterns    []string            `yaml:"pii_patterns"`
	AccessReporting bool               `yaml:"access_reporting"`
	ComplianceMode  string             `yaml:"compliance_mode"` // strict, normal, relaxed
}

// DataRetentionConfig configures data retention policies
type DataRetentionConfig struct {
	Enabled        bool  `yaml:"enabled"`
	TaskLogDays    int   `yaml:"task_log_days"`
	AuditLogDays   int   `yaml:"audit_log_days"`
	MetricsDays    int   `yaml:"metrics_days"`
	SystemEventDays int  `yaml:"system_event_days"`
	UserDataDays   int   `yaml:"user_data_days"`
}

// ObservabilityConfig configures observability features
type ObservabilityConfig struct {
	Metrics         MetricsConfig     `yaml:"metrics"`
	Tracing         TracingConfig     `yaml:"tracing"`
	Logging         LoggingConfig     `yaml:"logging"`
	Alerting        AlertingConfig    `yaml:"alerting"`
	Dashboards      DashboardsConfig  `yaml:"dashboards"`
	HealthChecks    HealthChecksConfig `yaml:"health_checks"`
}

// MetricsConfig configures metrics collection
type MetricsConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Endpoint      string `yaml:"endpoint"`
	ExportFormat  string `yaml:"export_format"` // prometheus, statsd, influxdb
	Interval      time.Duration `yaml:"interval"`
	IncludeLabels []string `yaml:"include_labels"`
}

// TracingConfig configures distributed tracing
type TracingConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Provider        string `yaml:"provider"` // jaeger, zipkin, datadog, otlp
	Endpoint        string `yaml:"endpoint"`
	ServiceName     string `yaml:"service_name"`
	SamplingRate    float64 `yaml:"sampling_rate"`
	PropagationFormat string `yaml:"propagation_format"`
}

// LoggingConfig configures logging
type LoggingConfig struct {
	Format          string `yaml:"format"` // json, text
	Level           string `yaml:"level"`
	Output          string `yaml:"output"` // file, stdout, syslog
	FilePath        string `yaml:"file_path"`
	RotationMaxSize int    `yaml:"rotation_max_size_mb"`
	StructuredLogging bool  `yaml:"structured_logging"`
}

// AlertingConfig configures alerting
type AlertingConfig struct {
	Enabled      bool     `yaml:"enabled"`
	Endpoints    []string `yaml:"endpoints"`
	AlertManager AlertManagerConfig `yaml:"alert_manager"`
}

// AlertManagerConfig configures the alert manager
type AlertManagerConfig struct {
	URL          string        `yaml:"url"`
	AuthType     string        `yaml:"auth_type"`
	Username     string        `yaml:"username"`
	PasswordFile string        `yaml:"password_file"`
	Timeout      time.Duration `yaml:"timeout"`
}

// DashboardsConfig configures dashboards
type DashboardsConfig struct {
	BuiltIn        bool   `yaml:"built_in"`
	GrafanaDashboardIDs []string `yaml:"grafana_dashboard_ids"`
	GrafanaURL     string `yaml:"grafana_url"`
}

// HealthChecksConfig configures health checks
type HealthChecksConfig struct {
	Enabled           bool          `yaml:"enabled"`
	Interval          time.Duration `yaml:"interval"`
	Timeout           time.Duration `yaml:"timeout"`
	HealthEndpoint    string        `yaml:"health_endpoint"`
	ReadinessEndpoint string        `yaml:"readiness_endpoint"`
	LivenessEndpoint  string        `yaml:"liveness_endpoint"`
}

// RecoveryConfig configures disaster recovery
type RecoveryConfig struct {
	Backup          BackupConfig     `yaml:"backup"`
	FailoverTimeout time.Duration    `yaml:"failover_timeout"`
	AutoRecovery    bool             `yaml:"auto_recovery"`
	RecoveryPlan    RecoveryPlanConfig `yaml:"recovery_plan"`
}

// BackupConfig configures backups
type BackupConfig struct {
	Enabled         bool          `yaml:"enabled"`
	Schedule        string        `yaml:"schedule"` // cron expression
	Type            string        `yaml:"type"`     // full, incremental
	Destination     string        `yaml:"destination"`
	RetentionCount  int           `yaml:"retention_count"`
	Timeout         time.Duration `yaml:"timeout"`
	IncludeData     []string      `yaml:"include_data"`
	ExcludeData     []string      `yaml:"exclude_data"`
	PreBackupScript string        `yaml:"pre_backup_script"`
	PostBackupScript string       `yaml:"post_backup_script"`
}

// RecoveryPlanConfig configures the recovery plan
type RecoveryPlanConfig struct {
	StepsFile    string        `yaml:"steps_file"`
	TimeoutTotal time.Duration `yaml:"timeout_total"`
	NotifyOnFail bool          `yaml:"notify_on_fail"`
	NotifyEmail  string        `yaml:"notify_email"`
}

// LoadEnterpriseConfig loads the enterprise configuration from a YAML file
func LoadEnterpriseConfig(configPath string) (*EnterpriseConfig, error) {
	// Make sure the file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file does not exist: %s", configPath)
	}

	// Read the file
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	// Parse the YAML
	var config EnterpriseConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse configuration file: %w", err)
	}

	return &config, nil
}

// SaveEnterpriseConfig saves the enterprise configuration to a YAML file
func SaveEnterpriseConfig(config *EnterpriseConfig, configPath string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	// Write to file
	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	return nil
}

// DefaultEnterpriseConfig returns the default enterprise configuration
func DefaultEnterpriseConfig() *EnterpriseConfig {
	return &EnterpriseConfig{
		General: GeneralConfig{
			Environment: "development",
			LogLevel:    "info",
			DataDir:     "./data",
			TempDir:     "./tmp",
		},
		Security: SecurityConfig{
			OAuth: OAuthConfig{
				Enabled:         true,
				DefaultProvider: "oidc",
				SessionTimeout:  24 * time.Hour,
				CookieSecure:    true,
				Providers: []ProviderConfig{
					{
						Name:        "oidc",
						Type:        "oidc",
						RedirectURL: "http://localhost:8080/auth/callback",
						Scopes:      []string{"openid", "profile", "email"},
					},
				},
			},
			RBAC: RBACConfig{
				Enabled:         true,
				DefaultUserRole: "user",
				CacheTimeout:    15 * time.Minute,
			},
			DataEncryption: EncryptionConfig{
				Enabled:      true,
				KeyEnvVar:    "ENCRYPTION_KEY",
				Algorithm:    "aes-256-gcm",
				KeyRotationInterval: 30 * 24 * time.Hour, // 30 days
			},
			AuditLogging: AuditConfig{
				Enabled:     true,
				MaxFileSize: 100,
				MaxBackups:  10,
				MaxAge:      30,
				Compress:    true,
			},
			NetworkSecurity: NetworkSecurityConfig{
				TLS: TLSConfig{
					Enabled:    true,
					MinVersion: "TLS1.2",
					MaxVersion: "TLS1.3",
				},
				EnableCORS:         true,
				RateLimiting:       true,
				RequestsPerMinute:  60,
			},
			Secrets: SecretsConfig{
				Provider:  "env",
				KeyPrefix: "TS_",
			},
		},
		Tenancy: TenancyConfig{
			Enabled:              true,
			IsolationMode:        "shared",
			DatabaseMode:         "shared",
			DefaultPlan:          "standard",
			TenantOnboarding:     true,
			TenantOffboarding:    true,
			AutoProvision:        true,
			ResourceQuotaEnabled: true,
			CachePerTenant:       false,
		},
		HighAvailability: HAConfig{
			Enabled:           true,
			Mode:              "active-active",
			HeartbeatInterval: 5 * time.Second,
			ElectionTimeout:   15 * time.Second,
			StorageType:       "redis",
			StorageEndpoint:   "localhost:6379",
			LeadershipLock:    "task-scheduler-leader",
			NodeTypes:         []string{"scheduler", "worker", "api"},
			ZoneAwareness:     true,
			RegionAwareness:   false,
		},
		Integrations: IntegrationsConfig{
			Enabled:      true,
			ConnectorsDir: "./connectors",
			RateLimits: GlobalRateLimitConfig{
				Enabled:           true,
				DefaultRPM:        60,
				DefaultBurstSize:  10,
				MaxRetries:        3,
				RetryBaseInterval: 1 * time.Second,
			},
			Timeouts: ConnectorTimeoutConfig{
				Connect:     5 * time.Second,
				Read:        30 * time.Second,
				Write:       30 * time.Second,
				IdleTimeout: 90 * time.Second,
			},
			Connectors: []ConnectorConfig{
				{
					ID:      "github",
					Name:    "GitHub",
					Type:    "github",
					Enabled: true,
					Settings: map[string]interface{}{
						"api_url": "https://api.github.com",
					},
					CredentialsFile: "credentials/github.json",
				},
				{
					ID:      "slack",
					Name:    "Slack",
					Type:    "slack",
					Enabled: true,
					Settings: map[string]interface{}{
						"api_url": "https://slack.com/api",
					},
					CredentialsFile: "credentials/slack.json",
				},
			},
		},
		Compliance: ComplianceConfig{
			DataRetention: DataRetentionConfig{
				Enabled:         true,
				TaskLogDays:     90,
				AuditLogDays:    365,
				MetricsDays:     30,
				SystemEventDays: 90,
				UserDataDays:    90,
			},
			Anonymization:   true,
			PIIProtection:   true,
			AccessReporting: true,
			ComplianceMode:  "normal",
		},
		Observability: ObservabilityConfig{
			Metrics: MetricsConfig{
				Enabled:      true,
				Endpoint:     "/metrics",
				ExportFormat: "prometheus",
				Interval:     15 * time.Second,
			},
			Tracing: TracingConfig{
				Enabled:     true,
				Provider:    "jaeger",
				ServiceName: "task-scheduler",
				SamplingRate: 0.1,
			},
			Logging: LoggingConfig{
				Format:            "json",
				Level:             "info",
				Output:            "file",
				StructuredLogging: true,
			},
			Alerting: AlertingConfig{
				Enabled: true,
				AlertManager: AlertManagerConfig{
					Timeout: 5 * time.Second,
				},
			},
			HealthChecks: HealthChecksConfig{
				Enabled:           true,
				Interval:          30 * time.Second,
				Timeout:           5 * time.Second,
				HealthEndpoint:    "/health",
				ReadinessEndpoint: "/ready",
				LivenessEndpoint:  "/alive",
			},
		},
		Recovery: RecoveryConfig{
			Backup: BackupConfig{
				Enabled:        true,
				Schedule:       "0 0 * * *", // Daily at midnight
				Type:           "full",
				RetentionCount: 7,
				Timeout:        30 * time.Minute,
			},
			FailoverTimeout: 60 * time.Second,
			AutoRecovery:    true,
			RecoveryPlan: RecoveryPlanConfig{
				TimeoutTotal: 4 * time.Hour,
				NotifyOnFail: true,
			},
		},
	}
} 