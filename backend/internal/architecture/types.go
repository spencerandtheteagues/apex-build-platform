package architecture

import "time"

const SchemaVersion = "1.0.0"

type CodeLocation struct {
	Path string `json:"path"`
}

type Node struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Layer         string         `json:"layer"`
	Type          string         `json:"type"`
	Description   string         `json:"description,omitempty"`
	RiskLevel     string         `json:"risk_level"`
	RiskScore     int            `json:"risk_score"`
	CodeLocations []CodeLocation `json:"code_locations"`
	FileCount     int            `json:"file_count"`
	TestCount     int            `json:"test_count"`
	References    int            `json:"references,omitempty"`
	Tags          []string       `json:"tags,omitempty"`
}

type Edge struct {
	ID          string `json:"id"`
	FromNode    string `json:"from_node"`
	ToNode      string `json:"to_node"`
	EdgeType    string `json:"edge_type"`
	Criticality string `json:"criticality"`
	Description string `json:"description,omitempty"`
}

type Contract struct {
	ID             string   `json:"id"`
	ContractType   string   `json:"contract_type"`
	Producer       string   `json:"producer"`
	Consumers      []string `json:"consumers"`
	SchemaLocation string   `json:"schema_location"`
	References     int      `json:"references,omitempty"`
	TestLocations  []string `json:"test_locations,omitempty"`
}

type DiagnosticPlaybook struct {
	ID             string   `json:"id"`
	Symptom        string   `json:"symptom"`
	EntryNodes     []string `json:"entry_nodes"`
	Checks         []string `json:"checks"`
	SafeRepairPlan []string `json:"safe_repair_plan"`
}

type QualityGate struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Checks      []string `json:"checks"`
}

type Map struct {
	SchemaVersion       string               `json:"schema_version"`
	GeneratedAt         time.Time            `json:"generated_at"`
	RepoRoot            string               `json:"repo_root,omitempty"`
	RepoSHA             string               `json:"repo_sha,omitempty"`
	Source              string               `json:"source"`
	Confidence          float64              `json:"confidence"`
	Summary             Summary              `json:"summary"`
	Nodes               []Node               `json:"nodes"`
	Edges               []Edge               `json:"edges"`
	Contracts           []Contract           `json:"contracts"`
	DiagnosticPlaybooks []DiagnosticPlaybook `json:"diagnostic_playbooks,omitempty"`
	QualityGates        []QualityGate        `json:"quality_gates,omitempty"`
	ReferenceTelemetry  *ReferenceTelemetry  `json:"reference_telemetry,omitempty"`
}

type Summary struct {
	NodeCount      int `json:"node_count"`
	EdgeCount      int `json:"edge_count"`
	ContractCount  int `json:"contract_count"`
	FileCount      int `json:"file_count"`
	TestFileCount  int `json:"test_file_count"`
	ReferenceCount int `json:"reference_count,omitempty"`
	HighRiskNodes  int `json:"high_risk_nodes"`
	CriticalNodes  int `json:"critical_nodes"`
}

type ReferenceTelemetry struct {
	TotalReferences int              `json:"total_references"`
	ByNode          map[string]int   `json:"by_node,omitempty"`
	ByDirectory     map[string]int   `json:"by_directory,omitempty"`
	ByContract      map[string]int   `json:"by_contract,omitempty"`
	ByDatabase      map[string]int   `json:"by_database,omitempty"`
	ByStructure     map[string]int   `json:"by_structure,omitempty"`
	ByAgentRole     map[string]int   `json:"by_agent_role,omitempty"`
	ByTaskType      map[string]int   `json:"by_task_type,omitempty"`
	RecentEvents    []ReferenceEvent `json:"recent_events,omitempty"`
	LastUpdatedAt   *time.Time       `json:"last_updated_at,omitempty"`
}

type ReferenceEvent struct {
	BuildID   string         `json:"build_id,omitempty"`
	TaskID    string         `json:"task_id,omitempty"`
	TaskType  string         `json:"task_type,omitempty"`
	AgentRole string         `json:"agent_role,omitempty"`
	Provider  string         `json:"provider,omitempty"`
	Model     string         `json:"model,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	Hits      []ReferenceHit `json:"hits,omitempty"`
}

type ReferenceHit struct {
	NodeID    string `json:"node_id,omitempty"`
	Directory string `json:"directory,omitempty"`
	Contract  string `json:"contract,omitempty"`
	Database  string `json:"database,omitempty"`
	Structure string `json:"structure,omitempty"`
	Count     int    `json:"count"`
}

type ReferenceInput struct {
	BuildID   string
	TaskID    string
	TaskType  string
	AgentRole string
	Provider  string
	Model     string
	Timestamp time.Time
	Texts     []string
}
