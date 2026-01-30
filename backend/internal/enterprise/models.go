// APEX.BUILD Enterprise Models
// Database models for enterprise features

package enterprise

import (
	"time"

	"gorm.io/gorm"
)

// Organization represents a company or team using APEX.BUILD
type Organization struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Organization details
	Name         string `json:"name" gorm:"not null;uniqueIndex"`
	Slug         string `json:"slug" gorm:"not null;uniqueIndex"`
	Description  string `json:"description"`
	Website      string `json:"website"`
	LogoURL      string `json:"logo_url"`

	// Billing
	BillingEmail       string    `json:"billing_email"`
	StripeCustomerID   string    `json:"stripe_customer_id" gorm:"index"`
	SubscriptionID     string    `json:"subscription_id"`
	SubscriptionType   string    `json:"subscription_type" gorm:"default:'team'"`
	SubscriptionStatus string    `json:"subscription_status" gorm:"default:'active'"`
	SubscriptionEnd    time.Time `json:"subscription_end"`

	// Limits based on plan
	MaxMembers       int     `json:"max_members" gorm:"default:15"`
	MaxProjects      int     `json:"max_projects" gorm:"default:100"`
	MaxStorageGB     float64 `json:"max_storage_gb" gorm:"default:50"`
	MaxAIRequests    int     `json:"max_ai_requests" gorm:"default:10000"`
	UsedStorageBytes int64   `json:"used_storage_bytes" gorm:"default:0"`

	// SSO/SAML Configuration
	SSOEnabled     bool   `json:"sso_enabled" gorm:"default:false"`
	SAMLEntityID   string `json:"saml_entity_id"`
	SAMLSSOURL     string `json:"saml_sso_url"`
	SAMLCertificate string `json:"saml_certificate" gorm:"type:text"`
	SCIMEnabled    bool   `json:"scim_enabled" gorm:"default:false"`
	SCIMToken      string `json:"scim_token" gorm:"-"`
	SCIMTokenHash  string `json:"scim_token_hash"`

	// Features
	AuditLogsEnabled     bool `json:"audit_logs_enabled" gorm:"default:true"`
	AdvancedRBACEnabled  bool `json:"advanced_rbac_enabled" gorm:"default:false"`
	DataExportEnabled    bool `json:"data_export_enabled" gorm:"default:true"`
	CustomBrandingEnabled bool `json:"custom_branding_enabled" gorm:"default:false"`

	// Data retention
	AuditLogRetentionDays int `json:"audit_log_retention_days" gorm:"default:90"`
	DataRetentionDays     int `json:"data_retention_days" gorm:"default:365"`

	// Relationships
	Members      []OrganizationMember `json:"members" gorm:"foreignKey:OrganizationID"`
	Roles        []Role               `json:"roles" gorm:"foreignKey:OrganizationID"`
	AuditLogs    []AuditLog           `json:"audit_logs" gorm:"foreignKey:OrganizationID"`
	RateLimits   []RateLimit          `json:"rate_limits" gorm:"foreignKey:OrganizationID"`
	Invitations  []Invitation         `json:"invitations" gorm:"foreignKey:OrganizationID"`
}

// OrganizationMember represents a user's membership in an organization
type OrganizationMember struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	OrganizationID uint         `json:"organization_id" gorm:"not null;index"`
	Organization   Organization `json:"organization" gorm:"foreignKey:OrganizationID"`
	UserID         uint         `json:"user_id" gorm:"not null;index"`

	// Role assignment
	RoleID uint `json:"role_id" gorm:"not null"`
	Role   Role `json:"role" gorm:"foreignKey:RoleID"`

	// SSO metadata
	SAMLNameID     string                 `json:"saml_name_id"`
	SAMLAttributes map[string]interface{} `json:"saml_attributes" gorm:"serializer:json"`
	ProvisionedBy  string                 `json:"provisioned_by"` // manual, saml, scim

	// Status
	Status       string     `json:"status" gorm:"default:'active'"` // active, suspended, pending
	InvitedBy    *uint      `json:"invited_by"`
	InvitedAt    *time.Time `json:"invited_at"`
	JoinedAt     *time.Time `json:"joined_at"`
	LastActiveAt *time.Time `json:"last_active_at"`
}

// Role represents an RBAC role within an organization
type Role struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	OrganizationID *uint        `json:"organization_id" gorm:"index"` // nil for system roles
	Organization   *Organization `json:"organization" gorm:"foreignKey:OrganizationID"`

	Name        string `json:"name" gorm:"not null"`
	Description string `json:"description"`
	IsSystem    bool   `json:"is_system" gorm:"default:false"` // System roles can't be deleted
	IsDefault   bool   `json:"is_default" gorm:"default:false"` // Default role for new members

	// Permissions
	Permissions []Permission `json:"permissions" gorm:"many2many:role_permissions;"`
}

// Permission represents a specific capability
type Permission struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Permission identity
	Resource string `json:"resource" gorm:"not null"` // projects, files, users, billing, etc.
	Action   string `json:"action" gorm:"not null"`   // create, read, update, delete, manage
	Scope    string `json:"scope" gorm:"default:'organization'"` // organization, project, own

	// Human-readable info
	Name        string `json:"name" gorm:"not null;uniqueIndex"`
	Description string `json:"description"`

	// Grouping
	Category string `json:"category"` // core, admin, billing, security
}

// ProjectPermission represents project-level permission overrides
type ProjectPermission struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	ProjectID uint `json:"project_id" gorm:"not null;index"`
	UserID    uint `json:"user_id" gorm:"not null;index"`
	RoleID    uint `json:"role_id" gorm:"not null"`
	Role      Role `json:"role" gorm:"foreignKey:RoleID"`

	// Custom permissions can be added/removed from role
	AdditionalPermissions []Permission `json:"additional_permissions" gorm:"many2many:project_permission_additions;"`
	RevokedPermissions    []Permission `json:"revoked_permissions" gorm:"many2many:project_permission_revocations;"`
}

// AuditLog records all actions in the system
type AuditLog struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`

	// Who
	OrganizationID *uint  `json:"organization_id" gorm:"index"`
	UserID         *uint  `json:"user_id" gorm:"index"`
	Username       string `json:"username"`
	Email          string `json:"email"`
	IPAddress      string `json:"ip_address"`
	UserAgent      string `json:"user_agent"`

	// What
	Action       string `json:"action" gorm:"not null;index"` // create, read, update, delete, login, etc.
	ResourceType string `json:"resource_type" gorm:"index"`   // project, file, user, etc.
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name"`

	// Details
	Description string                 `json:"description"`
	OldValue    map[string]interface{} `json:"old_value" gorm:"serializer:json"`
	NewValue    map[string]interface{} `json:"new_value" gorm:"serializer:json"`
	Metadata    map[string]interface{} `json:"metadata" gorm:"serializer:json"`

	// Context
	RequestID   string `json:"request_id"`
	SessionID   string `json:"session_id"`
	Environment string `json:"environment"` // production, staging, development

	// Classification
	Severity  string `json:"severity" gorm:"default:'info'"` // info, warning, error, critical
	Category  string `json:"category"`                       // authentication, authorization, data, system
	Outcome   string `json:"outcome" gorm:"default:'success'"` // success, failure, error

	// Compliance
	RetainUntil *time.Time `json:"retain_until" gorm:"index"`
	Exported    bool       `json:"exported" gorm:"default:false"`
}

// RateLimit defines per-user or per-endpoint rate limits
type RateLimit struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Scope
	OrganizationID *uint  `json:"organization_id" gorm:"index"`
	UserID         *uint  `json:"user_id" gorm:"index"`
	Endpoint       string `json:"endpoint"`       // Specific endpoint or "*" for all
	Plan           string `json:"plan"`           // free, pro, team, enterprise

	// Limits
	RequestsPerMinute int `json:"requests_per_minute" gorm:"default:60"`
	RequestsPerHour   int `json:"requests_per_hour" gorm:"default:1000"`
	RequestsPerDay    int `json:"requests_per_day" gorm:"default:10000"`

	// AI-specific limits
	AIRequestsPerMinute int `json:"ai_requests_per_minute" gorm:"default:10"`
	AIRequestsPerHour   int `json:"ai_requests_per_hour" gorm:"default:100"`
	AIRequestsPerDay    int `json:"ai_requests_per_day" gorm:"default:1000"`
	AITokensPerDay      int `json:"ai_tokens_per_day" gorm:"default:100000"`

	// Build limits
	ConcurrentBuilds int `json:"concurrent_builds" gorm:"default:3"`
	BuildsPerDay     int `json:"builds_per_day" gorm:"default:100"`

	// Override
	IsOverride bool   `json:"is_override" gorm:"default:false"`
	Reason     string `json:"reason"`
	ExpiresAt  *time.Time `json:"expires_at"`
}

// RateLimitUsage tracks current rate limit consumption
type RateLimitUsage struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at"`

	// Scope
	UserID    uint   `json:"user_id" gorm:"not null;index"`
	Endpoint  string `json:"endpoint" gorm:"index"`
	Window    string `json:"window"` // minute, hour, day

	// Counts
	RequestCount  int `json:"request_count" gorm:"default:0"`
	AIRequestCount int `json:"ai_request_count" gorm:"default:0"`
	TokenCount    int `json:"token_count" gorm:"default:0"`
	BuildCount    int `json:"build_count" gorm:"default:0"`

	// Window timing
	WindowStart time.Time `json:"window_start" gorm:"index"`
	WindowEnd   time.Time `json:"window_end"`
}

// UsageRecord tracks billable usage
type UsageRecord struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`

	// Who
	OrganizationID *uint `json:"organization_id" gorm:"index"`
	UserID         uint  `json:"user_id" gorm:"not null;index"`
	ProjectID      *uint `json:"project_id" gorm:"index"`

	// What
	ResourceType string `json:"resource_type" gorm:"not null"` // ai_request, build, storage, bandwidth
	Quantity     int    `json:"quantity" gorm:"not null"`
	Unit         string `json:"unit"` // requests, tokens, bytes, minutes

	// Cost calculation
	UnitCost float64 `json:"unit_cost"`
	TotalCost float64 `json:"total_cost"`
	Currency string  `json:"currency" gorm:"default:'USD'"`

	// Billing period
	BillingPeriodStart time.Time `json:"billing_period_start"`
	BillingPeriodEnd   time.Time `json:"billing_period_end"`

	// Metadata
	Description string                 `json:"description"`
	Metadata    map[string]interface{} `json:"metadata" gorm:"serializer:json"`

	// Status
	Billed   bool       `json:"billed" gorm:"default:false"`
	BilledAt *time.Time `json:"billed_at"`
	InvoiceID *string   `json:"invoice_id"`
}

// Invoice represents a billing invoice
type Invoice struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Invoice details
	InvoiceNumber string `json:"invoice_number" gorm:"uniqueIndex;not null"`
	OrganizationID *uint `json:"organization_id" gorm:"index"`
	UserID         uint  `json:"user_id" gorm:"index"`

	// Billing period
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	DueDate     time.Time `json:"due_date"`

	// Amounts
	Subtotal     float64 `json:"subtotal"`
	TaxAmount    float64 `json:"tax_amount"`
	TaxRate      float64 `json:"tax_rate"`
	Discount     float64 `json:"discount"`
	DiscountCode string  `json:"discount_code"`
	Total        float64 `json:"total"`
	Currency     string  `json:"currency" gorm:"default:'USD'"`

	// Status
	Status         string     `json:"status" gorm:"default:'draft'"` // draft, pending, paid, void, overdue
	PaidAt         *time.Time `json:"paid_at"`
	PaymentMethod  string     `json:"payment_method"`
	TransactionID  string     `json:"transaction_id"`

	// Stripe
	StripeInvoiceID string `json:"stripe_invoice_id"`
	StripePDFURL    string `json:"stripe_pdf_url"`

	// Line items stored as JSON
	LineItems []InvoiceLineItem `json:"line_items" gorm:"serializer:json"`

	// Notes
	Notes string `json:"notes"`
}

// InvoiceLineItem represents a line item on an invoice
type InvoiceLineItem struct {
	Description string  `json:"description"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Amount      float64 `json:"amount"`
	ResourceType string `json:"resource_type"`
}

// Invitation for organization members
type Invitation struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	OrganizationID uint         `json:"organization_id" gorm:"not null;index"`
	Organization   Organization `json:"organization" gorm:"foreignKey:OrganizationID"`

	Email     string `json:"email" gorm:"not null"`
	Token     string `json:"token" gorm:"uniqueIndex;not null"`
	RoleID    uint   `json:"role_id" gorm:"not null"`
	Role      Role   `json:"role" gorm:"foreignKey:RoleID"`

	InvitedByID uint `json:"invited_by_id" gorm:"not null"`

	ExpiresAt *time.Time `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at"`
	Status    string     `json:"status" gorm:"default:'pending'"` // pending, accepted, expired, revoked
}

// DataExportRequest for GDPR compliance
type DataExportRequest struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	UserID uint `json:"user_id" gorm:"not null;index"`

	// Request details
	RequestType string `json:"request_type" gorm:"not null"` // export, delete
	Scope       string `json:"scope"`                        // all, profile, projects, activity

	// Status
	Status        string     `json:"status" gorm:"default:'pending'"` // pending, processing, completed, failed
	ProcessedAt   *time.Time `json:"processed_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	ExpiresAt     *time.Time `json:"expires_at"`

	// Output
	DownloadURL   string `json:"download_url"`
	DownloadToken string `json:"download_token"`
	FileSize      int64  `json:"file_size"`

	// Audit
	ProcessedByID *uint  `json:"processed_by_id"`
	Notes         string `json:"notes"`
	ErrorMessage  string `json:"error_message"`
}

// AccountDeletionRequest for GDPR right to be forgotten
type AccountDeletionRequest struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	UserID uint `json:"user_id" gorm:"not null;index"`

	// Reason
	Reason      string `json:"reason"`
	Feedback    string `json:"feedback"`

	// Verification
	VerificationToken string     `json:"verification_token"`
	VerifiedAt        *time.Time `json:"verified_at"`

	// Grace period
	ScheduledDeletionAt time.Time  `json:"scheduled_deletion_at"` // Usually 30 days from request
	CancelledAt         *time.Time `json:"cancelled_at"`

	// Execution
	Status          string     `json:"status" gorm:"default:'pending'"` // pending, verified, scheduled, completed, cancelled
	DeletionStartedAt *time.Time `json:"deletion_started_at"`
	DeletionCompletedAt *time.Time `json:"deletion_completed_at"`

	// What was deleted
	DeletedResources map[string]int `json:"deleted_resources" gorm:"serializer:json"`
	ErrorMessage     string         `json:"error_message"`
}
