// Package hosting - Native Hosting Models for APEX.BUILD
// Provides .apex.app subdomain hosting like Replit's .replit.app
package hosting

import (
	"time"

	"gorm.io/gorm"
)

// DeploymentStatus represents the current state of a native deployment
type DeploymentStatus string

const (
	StatusPending      DeploymentStatus = "pending"
	StatusProvisioning DeploymentStatus = "provisioning"
	StatusBuilding     DeploymentStatus = "building"
	StatusDeploying    DeploymentStatus = "deploying"
	StatusRunning      DeploymentStatus = "running"
	StatusStopped      DeploymentStatus = "stopped"
	StatusFailed       DeploymentStatus = "failed"
	StatusDeleted      DeploymentStatus = "deleted"
)

// ContainerStatus represents the container health state
type ContainerStatus string

const (
	ContainerHealthy   ContainerStatus = "healthy"
	ContainerUnhealthy ContainerStatus = "unhealthy"
	ContainerStarting  ContainerStatus = "starting"
	ContainerStopped   ContainerStatus = "stopped"
)

// NativeDeployment represents a deployment to .apex.app native hosting
type NativeDeployment struct {
	ID        string           `json:"id" gorm:"primarykey;type:varchar(36)"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	DeletedAt gorm.DeletedAt   `json:"-" gorm:"index"`

	// Project and user association
	ProjectID uint   `json:"project_id" gorm:"not null;index"`
	UserID    uint   `json:"user_id" gorm:"not null;index"`

	// Subdomain configuration
	Subdomain        string `json:"subdomain" gorm:"uniqueIndex;not null;type:varchar(63)"` // [subdomain].apex.app
	CustomSubdomain  string `json:"custom_subdomain,omitempty" gorm:"type:varchar(63)"`     // User-requested subdomain if different
	SubdomainStatus  string `json:"subdomain_status" gorm:"default:'pending'"`              // pending, active, failed

	// Full URLs
	URL        string `json:"url"`                                     // https://[subdomain].apex.app
	PreviewURL string `json:"preview_url,omitempty"`                   // https://preview-[subdomain].apex.app

	// Deployment status
	Status          DeploymentStatus `json:"status" gorm:"not null;type:varchar(50);default:'pending'"`
	ContainerStatus ContainerStatus  `json:"container_status" gorm:"type:varchar(50);default:'stopped'"`
	ErrorMessage    string           `json:"error_message,omitempty" gorm:"type:text"`

	// Container configuration
	ContainerID   string `json:"container_id,omitempty" gorm:"type:varchar(64)"`
	ContainerPort int    `json:"container_port" gorm:"default:3000"`
	ExternalPort  int    `json:"external_port,omitempty"`

	// Build configuration
	BuildCommand   string `json:"build_command,omitempty" gorm:"type:varchar(500)"`
	StartCommand   string `json:"start_command,omitempty" gorm:"type:varchar(500)"`
	InstallCommand string `json:"install_command,omitempty" gorm:"type:varchar(500)"`
	Framework      string `json:"framework,omitempty" gorm:"type:varchar(50)"`
	NodeVersion    string `json:"node_version,omitempty" gorm:"default:'18'"`
	PythonVersion  string `json:"python_version,omitempty" gorm:"default:'3.11'"`
	GoVersion      string `json:"go_version,omitempty" gorm:"default:'1.21'"`

	// Resource limits
	MemoryLimit   int64 `json:"memory_limit" gorm:"default:512"`    // MB
	CPULimit      int64 `json:"cpu_limit" gorm:"default:500"`       // millicores (1000 = 1 CPU)
	StorageLimit  int64 `json:"storage_limit" gorm:"default:1024"`  // MB
	BandwidthUsed int64 `json:"bandwidth_used" gorm:"default:0"`    // MB

	// Scaling configuration
	AutoScale       bool  `json:"auto_scale" gorm:"default:false"`
	MinInstances    int   `json:"min_instances" gorm:"default:1"`
	MaxInstances    int   `json:"max_instances" gorm:"default:3"`
	CurrentInstances int  `json:"current_instances" gorm:"default:0"`

	// Health check configuration
	HealthCheckPath     string        `json:"health_check_path" gorm:"default:'/health'"`
	HealthCheckInterval int           `json:"health_check_interval" gorm:"default:30"` // seconds
	HealthCheckTimeout  int           `json:"health_check_timeout" gorm:"default:5"`   // seconds
	RestartOnFailure    bool          `json:"restart_on_failure" gorm:"default:true"`
	MaxRestarts         int           `json:"max_restarts" gorm:"default:3"`
	RestartCount        int           `json:"restart_count" gorm:"default:0"`

	// Always-On configuration (Replit parity feature)
	// When enabled, deployment stays running 24/7 with automatic restart on crash
	AlwaysOn          bool       `json:"always_on" gorm:"default:false"`
	AlwaysOnEnabled   *time.Time `json:"always_on_enabled_at,omitempty"`  // When always-on was enabled
	LastKeepAlive     *time.Time `json:"last_keep_alive,omitempty"`       // Last keep-alive ping timestamp
	KeepAliveInterval int        `json:"keep_alive_interval" gorm:"default:60"` // Keep-alive interval in seconds
	SleepAfterMinutes int        `json:"sleep_after_minutes" gorm:"default:0"` // 0 = never sleep (always-on)

	// DNS configuration
	DNSRecordID     string `json:"dns_record_id,omitempty" gorm:"type:varchar(50)"`
	DNSZoneID       string `json:"dns_zone_id,omitempty" gorm:"type:varchar(50)"`
	SSLCertificateID string `json:"ssl_certificate_id,omitempty" gorm:"type:varchar(50)"`
	SSLStatus       string `json:"ssl_status" gorm:"default:'pending'"` // pending, active, expired, error

	// Metrics
	TotalRequests   int64     `json:"total_requests" gorm:"default:0"`
	AvgResponseTime int64     `json:"avg_response_time" gorm:"default:0"` // ms
	LastRequestAt   *time.Time `json:"last_request_at,omitempty"`
	UptimeSeconds   int64     `json:"uptime_seconds" gorm:"default:0"`

	// Timestamps
	BuildStartedAt   *time.Time `json:"build_started_at,omitempty"`
	BuildCompletedAt *time.Time `json:"build_completed_at,omitempty"`
	DeployedAt       *time.Time `json:"deployed_at,omitempty"`
	LastHealthCheck  *time.Time `json:"last_health_check,omitempty"`

	// Build timing
	BuildDuration  int64 `json:"build_duration,omitempty"`  // ms
	DeployDuration int64 `json:"deploy_duration,omitempty"` // ms
}

// DeploymentLog represents a log entry for a native deployment
type DeploymentLog struct {
	ID           uint      `json:"id" gorm:"primarykey"`
	CreatedAt    time.Time `json:"created_at"`
	DeploymentID string    `json:"deployment_id" gorm:"not null;index;type:varchar(36)"`

	// Log details
	Timestamp time.Time `json:"timestamp" gorm:"not null;index"`
	Level     string    `json:"level" gorm:"not null;type:varchar(20)"` // debug, info, warn, error
	Source    string    `json:"source" gorm:"type:varchar(50)"`         // build, deploy, runtime, health
	Message   string    `json:"message" gorm:"type:text"`
	Metadata  string    `json:"metadata,omitempty" gorm:"type:text"`    // JSON metadata
}

// DeploymentEnvVar represents an environment variable for a deployment
type DeploymentEnvVar struct {
	ID           uint           `json:"id" gorm:"primarykey"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`

	DeploymentID string `json:"deployment_id" gorm:"not null;index;type:varchar(36)"`
	ProjectID    uint   `json:"project_id" gorm:"not null;index"`
	UserID       uint   `json:"user_id" gorm:"not null"`

	// Variable details
	Key         string `json:"key" gorm:"not null;type:varchar(255)"`
	Value       string `json:"-" gorm:"type:text"`                  // Encrypted, hidden from JSON
	Description string `json:"description,omitempty" gorm:"type:varchar(500)"`
	IsSecret    bool   `json:"is_secret" gorm:"default:false"`      // If true, never expose value
	IsSystem    bool   `json:"is_system" gorm:"default:false"`      // System-managed env vars

	// Scope
	Environment string `json:"environment" gorm:"default:'production'"` // production, preview, development
}

// DeploymentEnvVarMasked returns a masked version for API responses
type DeploymentEnvVarMasked struct {
	ID          uint   `json:"id"`
	Key         string `json:"key"`
	Value       string `json:"value"` // Masked if is_secret
	Description string `json:"description,omitempty"`
	IsSecret    bool   `json:"is_secret"`
	IsSystem    bool   `json:"is_system"`
	Environment string `json:"environment"`
}

// Mask creates a masked version of the env var
func (e *DeploymentEnvVar) Mask() DeploymentEnvVarMasked {
	value := e.Value
	if e.IsSecret {
		if len(value) > 4 {
			value = value[:4] + "********"
		} else {
			value = "********"
		}
	}

	return DeploymentEnvVarMasked{
		ID:          e.ID,
		Key:         e.Key,
		Value:       value,
		Description: e.Description,
		IsSecret:    e.IsSecret,
		IsSystem:    e.IsSystem,
		Environment: e.Environment,
	}
}

// DeploymentHistory tracks deployment history for rollbacks
type DeploymentHistory struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	DeploymentID string `json:"deployment_id" gorm:"not null;index;type:varchar(36)"`
	ProjectID    uint   `json:"project_id" gorm:"not null;index"`

	// Deployment snapshot
	Version       int    `json:"version" gorm:"not null"`
	CommitSHA     string `json:"commit_sha,omitempty" gorm:"type:varchar(40)"`
	CommitMessage string `json:"commit_message,omitempty" gorm:"type:varchar(500)"`
	ImageTag      string `json:"image_tag,omitempty" gorm:"type:varchar(100)"`

	// Status at time of snapshot
	Status       DeploymentStatus `json:"status" gorm:"type:varchar(50)"`
	BuildLogs    string           `json:"build_logs,omitempty" gorm:"type:text"`

	// Metrics at deployment time
	BuildDuration  int64 `json:"build_duration,omitempty"`
	DeployDuration int64 `json:"deploy_duration,omitempty"`
}

// Subdomain represents a claimed subdomain for conflict resolution
type Subdomain struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Subdomain info
	Name      string `json:"name" gorm:"uniqueIndex;not null;type:varchar(63)"`
	ProjectID uint   `json:"project_id" gorm:"not null;index"`
	UserID    uint   `json:"user_id" gorm:"not null;index"`

	// Status
	Status       string     `json:"status" gorm:"default:'active'"` // active, reserved, released
	ReservedUntil *time.Time `json:"reserved_until,omitempty"`       // For temporary reservations

	// DNS
	DNSConfigured bool `json:"dns_configured" gorm:"default:false"`
	SSLConfigured bool `json:"ssl_configured" gorm:"default:false"`
}

// TableName specifies the table name for NativeDeployment
func (NativeDeployment) TableName() string {
	return "native_deployments"
}

// TableName specifies the table name for DeploymentLog
func (DeploymentLog) TableName() string {
	return "deployment_logs"
}

// TableName specifies the table name for DeploymentEnvVar
func (DeploymentEnvVar) TableName() string {
	return "deployment_env_vars"
}

// TableName specifies the table name for DeploymentHistory
func (DeploymentHistory) TableName() string {
	return "deployment_history"
}

// TableName specifies the table name for Subdomain
func (Subdomain) TableName() string {
	return "subdomains"
}

// IsActive returns true if the deployment is running and healthy
func (d *NativeDeployment) IsActive() bool {
	return d.Status == StatusRunning && d.ContainerStatus == ContainerHealthy
}

// ShouldStayAwake returns true if the deployment should never sleep (always-on enabled)
func (d *NativeDeployment) ShouldStayAwake() bool {
	return d.AlwaysOn && d.SleepAfterMinutes == 0
}

// NeedsCrashRecovery returns true if the deployment crashed and should be auto-restarted
func (d *NativeDeployment) NeedsCrashRecovery() bool {
	return d.AlwaysOn && d.Status == StatusFailed && d.RestartCount < d.MaxRestarts
}

// GetFullURL returns the full URL for the deployment
func (d *NativeDeployment) GetFullURL() string {
	if d.URL != "" {
		return d.URL
	}
	return "https://" + d.Subdomain + ".apex.app"
}
