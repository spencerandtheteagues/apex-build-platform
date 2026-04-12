package deploy

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNeonDatabaseProvisionerEnsureDatabaseCreatesProjectAndInjectsEnv(t *testing.T) {
	t.Parallel()

	var createRequest map[string]any

	client, baseURL := newIPv4LoopbackTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/projects":
			if err := json.NewDecoder(r.Body).Decode(&createRequest); err != nil {
				t.Fatalf("decode create request: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"project": map[string]any{
					"id":   "proj_123",
					"name": "apex-project-7-db",
				},
				"branch": map[string]any{
					"id":   "br_123",
					"name": "main",
				},
				"roles": []map[string]any{
					{"name": "app_owner"},
				},
				"databases": []map[string]any{
					{"name": "app"},
				},
				"connection_uris": []map[string]any{
					{"connection_uri": "postgresql://app_owner:secret@ep-main.us-east-2.aws.neon.tech:5432/app?sslmode=require"},
				},
				"operations": []map[string]any{
					{"id": "op_123", "status": "finished"},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))

	provisioner := NewNeonDatabaseProvisioner("neon-token", "org_123")
	provisioner.baseURL = baseURL
	provisioner.httpClient = client

	result, err := provisioner.EnsureDatabase(context.Background(), &DeploymentConfig{
		ProjectID: 7,
		Branch:    "main",
		Database: &DatabaseConfig{
			Provider:     DatabaseProviderNeon,
			ProjectName:  "APEX Project 7 DB",
			DatabaseName: "app",
			RoleName:     "app_owner",
			RegionID:     "aws-us-east-2",
			PGVersion:    16,
		},
	})
	if err != nil {
		t.Fatalf("ensure database: %v", err)
	}

	projectPayload, ok := createRequest["project"].(map[string]any)
	if !ok {
		t.Fatalf("expected project payload, got %+v", createRequest)
	}
	if projectPayload["name"] != "apex-project-7-db" {
		t.Fatalf("expected sanitized project name, got %+v", projectPayload)
	}
	if projectPayload["region_id"] != "aws-us-east-2" || projectPayload["org_id"] != "org_123" {
		t.Fatalf("expected region and org persisted, got %+v", projectPayload)
	}
	if result.EnvVars["DATABASE_URL"] == "" || !strings.Contains(result.EnvVars["DATABASE_URL"], "ep-main.us-east-2.aws.neon.tech") {
		t.Fatalf("expected DATABASE_URL env var, got %+v", result.EnvVars)
	}
	if result.EnvVars["PGHOST"] != "ep-main.us-east-2.aws.neon.tech" || result.EnvVars["PGDATABASE"] != "app" {
		t.Fatalf("expected parsed PG env vars, got %+v", result.EnvVars)
	}
	if result.Metadata["neon_project_id"] != "proj_123" || result.Metadata["neon_branch_id"] != "br_123" {
		t.Fatalf("expected Neon metadata persisted, got %+v", result.Metadata)
	}
}

func TestNeonDatabaseProvisionerEnsureDatabaseReusesExistingProject(t *testing.T) {
	t.Parallel()

	var requestCount int
	client, baseURL := newIPv4LoopbackTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Method != http.MethodGet || r.URL.Path != "/projects/proj_123/connection_uri" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if r.URL.Query().Get("branch_id") != "br_123" {
			t.Fatalf("expected branch_id query param, got %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"uri": "postgresql://app_owner:secret@ep-main.us-east-2.aws.neon.tech:5432/app?sslmode=require",
		})
	}))

	provisioner := NewNeonDatabaseProvisioner("neon-token", "")
	provisioner.baseURL = baseURL
	provisioner.httpClient = client

	result, err := provisioner.EnsureDatabase(context.Background(), &DeploymentConfig{
		ProjectID: 7,
		Custom: map[string]interface{}{
			"neon_project_id":    "proj_123",
			"neon_project_name":  "apex-project-7-db",
			"neon_branch_id":     "br_123",
			"neon_branch_name":   "main",
			"neon_database_name": "app",
			"neon_role_name":     "app_owner",
		},
		Database: &DatabaseConfig{
			Provider: DatabaseProviderNeon,
		},
	})
	if err != nil {
		t.Fatalf("ensure database reuse: %v", err)
	}

	if requestCount != 1 {
		t.Fatalf("expected one connection URI request, got %d", requestCount)
	}
	if len(result.Logs) == 0 || !strings.Contains(result.Logs[0], "Reused Neon project") {
		t.Fatalf("expected reuse log, got %+v", result.Logs)
	}
	if result.Metadata["neon_project_id"] != "proj_123" {
		t.Fatalf("expected reused project metadata, got %+v", result.Metadata)
	}
}

func newIPv4LoopbackTestServer(t *testing.T, handler http.Handler) (*http.Client, string) {
	t.Helper()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "operation not permitted") {
			t.Skipf("sandbox does not permit loopback listeners: %v", err)
		}
		t.Fatalf("failed to open ipv4 loopback listener: %v", err)
	}
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	})

	client := &http.Client{Timeout: 5 * time.Second}
	return client, "http://" + listener.Addr().String()
}
