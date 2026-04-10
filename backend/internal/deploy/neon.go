package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type neonHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type NeonDatabaseProvisioner struct {
	token      string
	orgID      string
	baseURL    string
	httpClient neonHTTPClient
}

func NewNeonDatabaseProvisioner(token, orgID string) *NeonDatabaseProvisioner {
	return &NeonDatabaseProvisioner{
		token:   strings.TrimSpace(token),
		orgID:   strings.TrimSpace(orgID),
		baseURL: "https://console.neon.tech/api/v2",
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (p *NeonDatabaseProvisioner) Name() DatabaseProvider {
	return DatabaseProviderNeon
}

func (p *NeonDatabaseProvisioner) ValidateConfig(config *DatabaseConfig) error {
	if config == nil {
		return fmt.Errorf("database config is required")
	}
	if strings.TrimSpace(p.token) == "" {
		return fmt.Errorf("NEON_API_KEY is not configured")
	}
	if config.Provider != "" && config.Provider != DatabaseProviderNeon {
		return fmt.Errorf("unsupported database provider %q", config.Provider)
	}
	if config.PGVersion != 0 {
		switch config.PGVersion {
		case 14, 15, 16, 17:
		default:
			return fmt.Errorf("unsupported Neon PostgreSQL version %d", config.PGVersion)
		}
	}
	return nil
}

func (p *NeonDatabaseProvisioner) EnsureDatabase(ctx context.Context, config *DeploymentConfig) (*ProvisionedDatabaseResult, error) {
	if config == nil || config.Database == nil {
		return nil, fmt.Errorf("database config is required")
	}
	desired := p.resolveDesiredState(config)

	if desired.reuseProjectID != "" {
		uri, err := p.getConnectionURI(ctx, desired.reuseProjectID, desired.reuseBranchID, desired.databaseName, desired.roleName, desired.pooled)
		if err == nil {
			return p.provisionedResult(uri, desired.metadata("reused")), nil
		}
		var apiErr *neonAPIError
		if !isNeonNotFound(err, &apiErr) {
			return nil, err
		}
	}

	createResp, err := p.createProject(ctx, desired)
	if err != nil {
		return nil, err
	}

	projectID := strings.TrimSpace(createResp.Project.ID)
	branchID := strings.TrimSpace(createResp.Branch.ID)
	projectName := firstNonEmpty(strings.TrimSpace(createResp.Project.Name), desired.projectName)
	branchName := firstNonEmpty(strings.TrimSpace(createResp.Branch.Name), desired.branchName)
	databaseName := desired.databaseName
	if len(createResp.Databases) > 0 && strings.TrimSpace(createResp.Databases[0].Name) != "" {
		databaseName = strings.TrimSpace(createResp.Databases[0].Name)
	}
	roleName := desired.roleName
	if len(createResp.Roles) > 0 && strings.TrimSpace(createResp.Roles[0].Name) != "" {
		roleName = strings.TrimSpace(createResp.Roles[0].Name)
	}

	for _, op := range createResp.Operations {
		if err := p.waitForOperation(ctx, projectID, op.ID, op.Status); err != nil {
			return nil, err
		}
	}

	uri := strings.TrimSpace(firstNonEmpty(createResp.firstConnectionURI(), ""))
	if uri == "" {
		uri, err = p.getConnectionURI(ctx, projectID, branchID, databaseName, roleName, desired.pooled)
		if err != nil {
			return nil, err
		}
	}

	result := p.provisionedResult(uri, map[string]any{
		"database_provider":  "neon",
		"neon_project_id":    projectID,
		"neon_project_name":  projectName,
		"neon_branch_id":     branchID,
		"neon_branch_name":   branchName,
		"neon_database_name": databaseName,
		"neon_role_name":     roleName,
		"neon_pooled":        desired.pooled,
	})
	if desired.regionID != "" {
		result.Metadata["neon_region_id"] = desired.regionID
	}
	if desired.orgID != "" {
		result.Metadata["neon_org_id"] = desired.orgID
	}
	if desired.pgVersion != 0 {
		result.Metadata["neon_pg_version"] = desired.pgVersion
	}
	result.Logs = []string{
		fmt.Sprintf("Provisioned Neon project %s (%s)", projectName, projectID),
		fmt.Sprintf("Using Neon branch %s with database %s", branchName, databaseName),
	}
	return result, nil
}

type neonDesiredState struct {
	projectName    string
	branchName     string
	databaseName   string
	roleName       string
	regionID       string
	orgID          string
	pgVersion      int
	pooled         bool
	reuseProjectID string
	reuseBranchID  string
}

func (p *NeonDatabaseProvisioner) resolveDesiredState(config *DeploymentConfig) neonDesiredState {
	database := config.Database
	custom := config.Custom

	customBranchName := stringFromMap(custom, "neon_branch_name")
	customDatabaseName := stringFromMap(custom, "neon_database_name")
	customRoleName := stringFromMap(custom, "neon_role_name")

	desired := neonDesiredState{
		projectName:    sanitizeNeonProjectName(firstNonEmpty(database.ProjectName, stringFromMap(custom, "neon_project_name"), fmt.Sprintf("apex-project-%d-db", config.ProjectID))),
		branchName:     sanitizeNeonBranchName(firstNonEmpty(database.BranchName, customBranchName, config.Branch, "main")),
		databaseName:   sanitizeNeonIdentifier(firstNonEmpty(database.DatabaseName, customDatabaseName, "app"), "app"),
		roleName:       sanitizeNeonIdentifier(firstNonEmpty(database.RoleName, customRoleName, "app_owner"), "app_owner"),
		regionID:       firstNonEmpty(strings.TrimSpace(database.RegionID), stringFromMap(custom, "neon_region_id")),
		orgID:          firstNonEmpty(strings.TrimSpace(database.OrgID), stringFromMap(custom, "neon_org_id"), p.orgID),
		pgVersion:      firstNonZero(database.PGVersion, intFromMap(custom, "neon_pg_version")),
		pooled:         database.Pooled,
		reuseProjectID: stringFromMap(custom, "neon_project_id"),
		reuseBranchID:  stringFromMap(custom, "neon_branch_id"),
	}
	if desired.reuseProjectID != "" {
		branchCompatible := strings.TrimSpace(database.BranchName) == "" || desired.branchName == sanitizeNeonBranchName(firstNonEmpty(customBranchName, desired.branchName))
		databaseCompatible := strings.TrimSpace(database.DatabaseName) == "" || desired.databaseName == sanitizeNeonIdentifier(firstNonEmpty(customDatabaseName, desired.databaseName), desired.databaseName)
		roleCompatible := strings.TrimSpace(database.RoleName) == "" || desired.roleName == sanitizeNeonIdentifier(firstNonEmpty(customRoleName, desired.roleName), desired.roleName)
		if !(branchCompatible && databaseCompatible && roleCompatible) {
			desired.reuseProjectID = ""
			desired.reuseBranchID = ""
		}
	}
	return desired
}

func (desired neonDesiredState) metadata(mode string) map[string]any {
	metadata := map[string]any{
		"database_provider":  "neon",
		"neon_project_id":    desired.reuseProjectID,
		"neon_branch_id":     desired.reuseBranchID,
		"neon_branch_name":   desired.branchName,
		"neon_database_name": desired.databaseName,
		"neon_role_name":     desired.roleName,
		"neon_pooled":        desired.pooled,
		"neon_state":         mode,
	}
	if desired.projectName != "" {
		metadata["neon_project_name"] = desired.projectName
	}
	if desired.regionID != "" {
		metadata["neon_region_id"] = desired.regionID
	}
	if desired.orgID != "" {
		metadata["neon_org_id"] = desired.orgID
	}
	if desired.pgVersion != 0 {
		metadata["neon_pg_version"] = desired.pgVersion
	}
	return metadata
}

type neonProjectCreateResponse struct {
	Project        neonProject            `json:"project"`
	Branch         neonBranch             `json:"branch"`
	Roles          []neonRole             `json:"roles"`
	Databases      []neonDatabase         `json:"databases"`
	ConnectionURIs []neonConnectionDetail `json:"connection_uris"`
	Operations     []neonOperation        `json:"operations"`
}

func (r neonProjectCreateResponse) firstConnectionURI() string {
	if len(r.ConnectionURIs) == 0 {
		return ""
	}
	return strings.TrimSpace(r.ConnectionURIs[0].ConnectionURI)
}

type neonConnectionDetail struct {
	ConnectionURI string `json:"connection_uri"`
}

type neonProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type neonBranch struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type neonRole struct {
	Name string `json:"name"`
}

type neonDatabase struct {
	Name string `json:"name"`
}

type neonOperation struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

type neonOperationResponse struct {
	Operation neonOperation `json:"operation"`
}

type neonConnectionURIResponse struct {
	URI string `json:"uri"`
}

type neonAPIError struct {
	StatusCode int
	Body       string
}

func (e *neonAPIError) Error() string {
	return fmt.Sprintf("neon API returned %d: %s", e.StatusCode, strings.TrimSpace(e.Body))
}

func isNeonNotFound(err error, target **neonAPIError) bool {
	if err == nil {
		return false
	}
	var apiErr *neonAPIError
	if errors.As(err, &apiErr) {
		if target != nil {
			*target = apiErr
		}
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

func (p *NeonDatabaseProvisioner) createProject(ctx context.Context, desired neonDesiredState) (*neonProjectCreateResponse, error) {
	project := map[string]any{
		"name":            desired.projectName,
		"store_passwords": true,
		"branch": map[string]any{
			"name":          desired.branchName,
			"role_name":     desired.roleName,
			"database_name": desired.databaseName,
		},
	}
	if desired.regionID != "" {
		project["region_id"] = desired.regionID
	}
	if desired.orgID != "" {
		project["org_id"] = desired.orgID
	}
	if desired.pgVersion != 0 {
		project["pg_version"] = desired.pgVersion
	}

	body := map[string]any{"project": project}
	var response neonProjectCreateResponse
	if err := p.doJSON(ctx, http.MethodPost, "/projects", nil, body, &response); err != nil {
		return nil, err
	}
	if response.Project.ID == "" {
		return nil, fmt.Errorf("Neon project creation response did not include a project ID")
	}
	return &response, nil
}

func (p *NeonDatabaseProvisioner) waitForOperation(ctx context.Context, projectID, operationID, initialStatus string) error {
	if strings.TrimSpace(operationID) == "" {
		return nil
	}
	status := strings.ToLower(strings.TrimSpace(initialStatus))
	if neonOperationSucceeded(status) {
		return nil
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			var response neonOperationResponse
			path := fmt.Sprintf("/projects/%s/operations/%s", projectID, operationID)
			if err := p.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
				return err
			}
			status = strings.ToLower(strings.TrimSpace(response.Operation.Status))
			switch {
			case neonOperationSucceeded(status):
				return nil
			case neonOperationFailed(status):
				return fmt.Errorf("Neon operation %s failed: %s", operationID, firstNonEmpty(response.Operation.Error, "unknown error"))
			}
		}
	}
}

func neonOperationSucceeded(status string) bool {
	return status == "finished" || status == "succeeded" || status == "success"
}

func neonOperationFailed(status string) bool {
	switch status {
	case "failed", "error", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func (p *NeonDatabaseProvisioner) getConnectionURI(ctx context.Context, projectID, branchID, databaseName, roleName string, pooled bool) (string, error) {
	query := neturl.Values{}
	if branchID != "" {
		query.Set("branch_id", branchID)
	}
	query.Set("database_name", databaseName)
	query.Set("role_name", roleName)
	if pooled {
		query.Set("pooled", "true")
	}
	var response neonConnectionURIResponse
	path := fmt.Sprintf("/projects/%s/connection_uri", projectID)
	if err := p.doJSON(ctx, http.MethodGet, path, query, nil, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.URI) == "" {
		return "", fmt.Errorf("Neon did not return a connection URI")
	}
	return response.URI, nil
}

func (p *NeonDatabaseProvisioner) provisionedResult(connectionURI string, metadata map[string]any) *ProvisionedDatabaseResult {
	env := databaseEnvVarsFromURI(connectionURI)
	result := &ProvisionedDatabaseResult{
		EnvVars:  env,
		Metadata: metadata,
	}
	if state, ok := metadata["neon_state"].(string); ok && state == "reused" {
		result.Logs = []string{
			fmt.Sprintf("Reused Neon project %s", firstNonEmpty(stringFromAny(metadata["neon_project_name"]), stringFromAny(metadata["neon_project_id"]))),
			fmt.Sprintf("Reusing Neon database %s owned by %s", stringFromAny(metadata["neon_database_name"]), stringFromAny(metadata["neon_role_name"])),
		}
	}
	return result
}

func databaseEnvVarsFromURI(connectionURI string) map[string]string {
	env := map[string]string{
		"DATABASE_URL": connectionURI,
		"POSTGRES_URL": connectionURI,
	}
	parsed, err := neturl.Parse(connectionURI)
	if err != nil {
		return env
	}
	username := ""
	password := ""
	if parsed.User != nil {
		username = parsed.User.Username()
		password, _ = parsed.User.Password()
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		port = "5432"
	}
	databaseName := strings.TrimPrefix(parsed.Path, "/")
	env["PGHOST"] = host
	env["PGPORT"] = port
	env["PGDATABASE"] = databaseName
	env["PGUSER"] = username
	env["PGPASSWORD"] = password
	return env
}

func (p *NeonDatabaseProvisioner) doJSON(ctx context.Context, method, path string, query neturl.Values, body any, dest any) error {
	endpoint := strings.TrimRight(p.baseURL, "/") + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return &neonAPIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	if dest == nil || len(respBody) == 0 {
		return nil
	}
	return json.Unmarshal(respBody, dest)
}

func sanitizeNeonProjectName(value string) string {
	name := sanitizeNeonSlug(value, "apex-db")
	if name == "" {
		return "apex-db"
	}
	return name
}

func sanitizeNeonBranchName(value string) string {
	name := sanitizeNeonSlug(value, "main")
	if name == "" {
		return "main"
	}
	return name
}

func sanitizeNeonSlug(value, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = fallback
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		name = fallback
	}
	if len(name) > 63 {
		name = strings.Trim(name[:63], "-")
	}
	if name == "" {
		return fallback
	}
	return name
}

func sanitizeNeonIdentifier(value, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = fallback
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	name := strings.Trim(b.String(), "_")
	if name == "" {
		name = fallback
	}
	if name == "" {
		name = "app"
	}
	if name[0] >= '0' && name[0] <= '9' {
		name = "db_" + name
	}
	if len(name) > 63 {
		name = strings.Trim(name[:63], "_")
	}
	if name == "" {
		return "app"
	}
	return name
}

func stringFromMap(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	return stringFromAny(values[key])
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func intFromMap(values map[string]interface{}, key string) int {
	if values == nil {
		return 0
	}
	switch typed := values[key].(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		value, _ := typed.Int64()
		return int(value)
	case string:
		value, _ := strconv.Atoi(strings.TrimSpace(typed))
		return value
	default:
		return 0
	}
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
