package spend

import "time"

// SpendEvent is a GORM model for the spend_events table
type SpendEvent struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UserID       uint      `gorm:"not null;index:idx_spend_user_day;index:idx_spend_user_month" json:"user_id"`
	ProjectID    *uint     `json:"project_id,omitempty"`
	BuildID      string    `gorm:"index:idx_spend_build" json:"build_id,omitempty"`
	AgentID      string    `json:"agent_id,omitempty"`
	AgentRole    string    `json:"agent_role,omitempty"`
	Provider     string    `gorm:"not null" json:"provider"`
	Model        string    `gorm:"not null" json:"model"`
	Capability   string    `json:"capability,omitempty"`
	IsBYOK       bool      `gorm:"default:false" json:"is_byok"`
	InputTokens  int       `gorm:"not null;default:0" json:"input_tokens"`
	OutputTokens int       `gorm:"not null;default:0" json:"output_tokens"`
	RawCost      float64   `gorm:"not null;default:0;type:numeric(12,6)" json:"raw_cost"`
	BilledCost   float64   `gorm:"not null;default:0;type:numeric(12,6)" json:"billed_cost"`
	PowerMode    string    `json:"power_mode,omitempty"`
	DurationMs   int       `gorm:"default:0" json:"duration_ms"`
	Status       string    `gorm:"default:success" json:"status"`
	TargetFile   string    `json:"target_file,omitempty"`
	DayKey       string    `gorm:"not null;index:idx_spend_user_day;type:date" json:"day_key"`
	MonthKey     string    `gorm:"not null;index:idx_spend_user_month" json:"month_key"`
}

func (SpendEvent) TableName() string { return "spend_events" }

// SpendSummary is returned by summary endpoints
type SpendSummary struct {
	DailySpend   float64 `json:"daily_spend"`
	MonthlySpend float64 `json:"monthly_spend"`
	BuildSpend   float64 `json:"build_spend,omitempty"`
	DailyCount   int     `json:"daily_count"`
	MonthlyCount int     `json:"monthly_count"`
}

// SpendBreakdownItem represents a row in a breakdown query
type SpendBreakdownItem struct {
	Key          string  `json:"key"`
	BilledCost   float64 `json:"billed_cost"`
	RawCost      float64 `json:"raw_cost"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	Count        int     `json:"count"`
}

// BreakdownOpts controls how breakdowns are grouped
type BreakdownOpts struct {
	GroupBy   string // "provider", "model", "agent_role", "build_id"
	UserID    uint
	DayKey    string // YYYY-MM-DD
	MonthKey  string // YYYY-MM
	BuildID   string
	ProjectID *uint
}

// RecordSpendInput contains all data needed to record a spend event
type RecordSpendInput struct {
	UserID       uint
	ProjectID    *uint
	BuildID      string
	AgentID      string
	AgentRole    string
	Provider     string
	Model        string
	Capability   string
	IsBYOK       bool
	InputTokens  int
	OutputTokens int
	PowerMode    string
	DurationMs   int
	Status       string
	TargetFile   string
}
