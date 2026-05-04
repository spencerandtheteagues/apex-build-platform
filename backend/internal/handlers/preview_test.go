package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"apex-build/internal/preview"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newPreviewHandlerTestFixture(t *testing.T, requireSandbox bool) (*PreviewHandler, uint) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Project{}, &models.File{}))

	user := models.User{
		Username:     strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_"),
		Email:        strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_") + "@example.com",
		PasswordHash: "hashed-password",
	}
	require.NoError(t, db.Create(&user).Error)

	project := models.Project{
		Name:     "Preview Fixture",
		Language: "typescript",
		OwnerID:  user.ID,
	}
	require.NoError(t, db.Create(&project).Error)

	handler := &PreviewHandler{
		db:             db,
		server:         preview.NewPreviewServer(db),
		serverRunner:   preview.NewServerRunner(db),
		requireSandbox: requireSandbox,
	}

	return handler, project.ID
}

func setPreviewFactoryDockerAvailable(t *testing.T, factory *preview.PreviewServerFactory, available bool) {
	t.Helper()
	field := reflect.ValueOf(factory).Elem().FieldByName("dockerAvailable")
	require.True(t, field.IsValid())
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().SetBool(available)
}

func setPreviewFactoryContainerServer(t *testing.T, factory *preview.PreviewServerFactory, server *preview.ContainerPreviewServer) {
	t.Helper()
	field := reflect.ValueOf(factory).Elem().FieldByName("containerServer")
	require.True(t, field.IsValid())
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(server))
}

type fakePreviewRuntime struct {
	name      string
	readyURL  string
	waitCh    chan struct{}
	waitError error
}

func newFakePreviewRuntime(name, readyURL string) *fakePreviewRuntime {
	return &fakePreviewRuntime{
		name:     name,
		readyURL: readyURL,
		waitCh:   make(chan struct{}),
	}
}

func (f *fakePreviewRuntime) Name() string { return f.name }

func (f *fakePreviewRuntime) RequiresLocalDependencyInstall() bool { return false }

func (f *fakePreviewRuntime) StartProcess(cfg *preview.ProcessStartConfig) (*preview.ProcessHandle, error) {
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			close(f.waitCh)
		})
	}

	return &preview.ProcessHandle{
		Pid:        4242,
		StdoutPipe: io.NopCloser(strings.NewReader("")),
		StderrPipe: io.NopCloser(strings.NewReader("")),
		ReadyURL:   f.readyURL,
		Wait: func() (int, error) {
			<-f.waitCh
			if f.waitError != nil {
				return 1, f.waitError
			}
			return 0, nil
		},
		SignalStop: stop,
		ForceKill:  stop,
	}, nil
}

type failingPreviewRuntime struct{}

func (f *failingPreviewRuntime) Name() string { return "failing" }

func (f *failingPreviewRuntime) StartProcess(*preview.ProcessStartConfig) (*preview.ProcessHandle, error) {
	return nil, fmt.Errorf("simulated runtime start failure")
}

func TestPreviewHandlerRejectsExplicitSandboxWithoutDocker(t *testing.T) {
	useSandbox, err := (&PreviewHandler{}).resolveRequestedPreviewSandbox(true)
	require.Error(t, err)
	require.False(t, useSandbox)
}

func TestPreviewHandlerStartPreviewFallsBackWhenSandboxRequiredButDockerUnavailable(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, true)

	body, err := json.Marshal(map[string]any{
		"project_id": projectID,
		"sandbox":    true,
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/preview/start", bytes.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("user_id", uint(1))

	handler.StartPreview(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"sandbox_degraded":true`)
	require.Contains(t, recorder.Body.String(), `"sandbox":false`)
}

func TestPreviewHandlerStartPreviewRejectsProcessFallbackWhenSandboxContainerStartFails(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, true)
	factory, err := preview.NewPreviewServerFactory(handler.db, &preview.FactoryConfig{EnableContainerPreviews: false})
	require.NoError(t, err)
	setPreviewFactoryDockerAvailable(t, factory, true)
	setPreviewFactoryContainerServer(t, factory, &preview.ContainerPreviewServer{})
	handler.factory = factory
	handler.server = factory.GetProcessServer()

	body, err := json.Marshal(map[string]any{
		"project_id":  projectID,
		"entry_point": "index.html",
		"framework":   "react",
		"sandbox":     true,
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/preview/start", bytes.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Host = "apex-build.dev"
	context.Set("user_id", uint(1))

	handler.StartPreview(context)

	require.Equal(t, http.StatusInternalServerError, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `sandbox preview failed`)
	require.Contains(t, recorder.Body.String(), "Docker is not available")
	require.NotContains(t, recorder.Body.String(), `"success":true`)
	require.NotContains(t, recorder.Body.String(), `"degraded":true`)
	require.NotContains(t, recorder.Body.String(), `"frontend_fallback":true`)
}

func TestPreviewFrontendStartTimeoutAllowsContainerBuildWindow(t *testing.T) {
	t.Setenv("APEX_PREVIEW_FRONTEND_START_TIMEOUT_MS", "")

	require.GreaterOrEqual(t, previewFrontendStartTimeout(), 180*time.Second)
	require.LessOrEqual(t, previewFrontendStartTimeout(), 210*time.Second)
}

func TestPreviewHandlerFullStackNextFallsBackToFrontendPreviewWhenRuntimeFails(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, false)
	handler.serverRunner = preview.NewServerRunnerWithRuntime(handler.db, &failingPreviewRuntime{})

	files := []models.File{
		{
			ProjectID: projectID,
			Path:      "package.json",
			Name:      "package.json",
			Type:      "file",
			Content: `{
				"scripts": {"dev": "next dev", "build": "next build", "start": "next start"},
				"dependencies": {"next": "^15.3.2", "react": "^18.3.1", "react-dom": "^18.3.1"}
			}`,
		},
		{
			ProjectID: projectID,
			Path:      "app/page.tsx",
			Name:      "page.tsx",
			Type:      "file",
			Content:   `export default function Page() { return <main>Apex FieldOps AI</main> }`,
		},
	}
	require.NoError(t, handler.db.Create(&files).Error)

	body, err := json.Marshal(map[string]any{
		"project_id":      projectID,
		"framework":       "next",
		"entry_point":     "app/page.tsx",
		"start_backend":   true,
		"require_backend": false,
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/preview/fullstack/start", bytes.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Host = "apex-build.dev"
	context.Set("user_id", uint(1))
	context.Set("bypass_billing", true)

	handler.StartFullStackPreview(context)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.Contains(t, recorder.Body.String(), `"degraded":true`)
	require.Contains(t, recorder.Body.String(), `"frontend_fallback":true`)
	require.Contains(t, recorder.Body.String(), `/api/v1/preview/proxy/`+strconv.FormatUint(uint64(projectID), 10)+`/`)
}

func TestPreviewHandlerFullStackOptionalBackendTimeoutStillReturnsFrontendPreview(t *testing.T) {
	t.Setenv("APEX_PREVIEW_OPTIONAL_BACKEND_START_TIMEOUT_MS", "10")

	handler, projectID := newPreviewHandlerTestFixture(t, false)
	handler.serverRunner = preview.NewServerRunnerWithRuntime(handler.db, newFakePreviewRuntime("host", "http://127.0.0.1:1/__never_ready"))

	files := []models.File{
		{
			ProjectID: projectID,
			Path:      "package.json",
			Name:      "package.json",
			Type:      "file",
			Content: `{
				"scripts": {"start": "node server.js"},
				"dependencies": {"express": "^4.18.3"}
			}`,
		},
		{
			ProjectID: projectID,
			Path:      "server.js",
			Name:      "server.js",
			Type:      "file",
			Content:   `const express = require('express'); const app = express(); app.get('/api/health', (_req, res) => res.json({ok:true})); app.listen(process.env.PORT || 3000);`,
		},
		{
			ProjectID: projectID,
			Path:      "index.html",
			Name:      "index.html",
			Type:      "file",
			Content:   `<!doctype html><html><body><main>Apex preview still boots</main></body></html>`,
		},
	}
	require.NoError(t, handler.db.Create(&files).Error)

	body, err := json.Marshal(map[string]any{
		"project_id":      projectID,
		"framework":       "react",
		"entry_point":     "index.html",
		"start_backend":   true,
		"require_backend": false,
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/preview/fullstack/start", bytes.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Host = "apex-build.dev"
	context.Set("user_id", uint(1))
	context.Set("bypass_billing", true)

	handler.StartFullStackPreview(context)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.Contains(t, recorder.Body.String(), `"degraded":true`)
	require.Contains(t, recorder.Body.String(), `"backend_started":false`)
	require.Contains(t, recorder.Body.String(), `/api/v1/preview/proxy/`+strconv.FormatUint(uint64(projectID), 10)+`/`)
}

func TestPreviewHandlerFullStackOmitsBackendByDefaultForFrontendOnlyApps(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, false)

	files := []models.File{
		{
			ProjectID: projectID,
			Path:      "package.json",
			Name:      "package.json",
			Type:      "file",
			Content: `{
				"scripts": {"build": "vite build", "dev": "vite"},
				"dependencies": {"@vitejs/plugin-react": "^latest", "vite": "^latest", "react": "^latest", "react-dom": "^latest"}
			}`,
		},
		{
			ProjectID: projectID,
			Path:      "index.html",
			Name:      "index.html",
			Type:      "file",
			Content:   `<!doctype html><html><body><div id="root">Apex FieldOps AI</div></body></html>`,
		},
	}
	require.NoError(t, handler.db.Create(&files).Error)

	body, err := json.Marshal(map[string]any{
		"project_id":  projectID,
		"framework":   "react",
		"entry_point": "index.html",
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/preview/fullstack/start", bytes.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Host = "apex-build.dev"
	context.Set("user_id", uint(1))

	handler.StartFullStackPreview(context)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.Contains(t, recorder.Body.String(), `"backend_requested":false`)
	require.Contains(t, recorder.Body.String(), `"backend_skipped":true`)
	require.NotContains(t, recorder.Body.String(), `"backend_error"`)
	require.Contains(t, recorder.Body.String(), `/api/v1/preview/proxy/`+strconv.FormatUint(uint64(projectID), 10)+`/`)
}

func TestPreviewHandlerStartServerNotBlockedBySecureModeFallback(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, true)

	body, err := json.Marshal(map[string]any{
		"project_id": projectID,
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/preview/server/start", bytes.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("user_id", uint(1))

	handler.StartServer(context)

	require.NotEqual(t, http.StatusConflict, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "secure sandbox preview is enforced")
}

func TestPreviewHandlerGetServerStatusUsesFallbackModeWhenDockerUnavailable(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, true)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "projectId", Value: "1"}}
	context.Request = httptest.NewRequest(http.MethodGet, "/preview/server/status/1", nil)
	context.Set("user_id", uint(1))

	context.Params = gin.Params{{Key: "projectId", Value: strconv.FormatUint(uint64(projectID), 10)}}
	context.Request = httptest.NewRequest(http.MethodGet, "/preview/server/status/"+strconv.FormatUint(uint64(projectID), 10), nil)

	handler.GetServerStatus(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), `"available":false`)
	require.NotContains(t, recorder.Body.String(), "secure sandbox preview is enforced")
	require.Contains(t, recorder.Body.String(), `"success":true`)
}

func TestPreviewHandlerGetDockerStatusIncludesSecureModeFlags(t *testing.T) {
	handler, _ := newPreviewHandlerTestFixture(t, true)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/preview/docker/status", nil)

	handler.GetDockerStatus(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"sandbox_required":true`)
	require.Contains(t, recorder.Body.String(), `"sandbox_degraded":true`)
	require.Contains(t, recorder.Body.String(), `"backend_preview_available":true`)
	require.Contains(t, recorder.Body.String(), `"diagnostic":"preview container factory is disabled"`)
}

func TestPreviewHandlerGetDockerStatusTreatsE2BRuntimeAsBackendPreviewAvailable(t *testing.T) {
	handler, _ := newPreviewHandlerTestFixture(t, true)
	handler.serverRunner = preview.NewServerRunnerWithRuntime(handler.db, newFakePreviewRuntime("e2b", ""))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/preview/docker/status", nil)

	handler.GetDockerStatus(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"backend_preview_available":true`)
	require.Contains(t, recorder.Body.String(), `"backend_preview_runtime":"e2b"`)
}

func TestPreviewHandlerGetStatusFindsProcessFallbackWhenSandboxPreferred(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, true)
	factory, err := preview.NewPreviewServerFactory(handler.db, &preview.FactoryConfig{EnableContainerPreviews: false})
	require.NoError(t, err)
	setPreviewFactoryDockerAvailable(t, factory, true)
	handler.factory = factory
	handler.server = factory.GetProcessServer()

	_, err = handler.server.StartPreview(context.Background(), &preview.PreviewConfig{
		ProjectID:  projectID,
		EntryPoint: "index.html",
		Framework:  "react",
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "projectId", Value: strconv.FormatUint(uint64(projectID), 10)}}
	context.Request = httptest.NewRequest(http.MethodGet, "/preview/status/"+strconv.FormatUint(uint64(projectID), 10)+"?sandbox=1", nil)
	context.Set("user_id", uint(1))

	handler.GetPreviewStatus(context)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"active":true`)
	require.Contains(t, recorder.Body.String(), `"sandbox":false`)
	require.Contains(t, recorder.Body.String(), `"sandbox_degraded":true`)
}

func TestPreviewHandlerFactoryStatusPrefersActiveFrameworkRuntime(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, false)
	factory, err := preview.NewPreviewServerFactory(handler.db, &preview.FactoryConfig{EnableContainerPreviews: false})
	require.NoError(t, err)
	handler.factory = factory
	handler.server = factory.GetProcessServer()

	files := []models.File{
		{
			ProjectID: projectID,
			Path:      "package.json",
			Name:      "package.json",
			Type:      "file",
			Content:   `{"dependencies":{"next":"^15.3.2","react":"^18.3.1","react-dom":"^18.3.1"}}`,
		},
		{
			ProjectID: projectID,
			Path:      "app/page.tsx",
			Name:      "page.tsx",
			Type:      "file",
			Content:   `export default function Page() { return <main>Apex Preview Runtime</main> }`,
		},
	}
	require.NoError(t, handler.db.Create(&files).Error)

	readyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer readyServer.Close()

	handler.serverRunner = preview.NewServerRunnerWithRuntime(handler.db, newFakePreviewRuntime("e2b", readyServer.URL))
	proc, err := handler.serverRunner.Start(context.Background(), &preview.ServerConfig{
		ProjectID:    projectID,
		EntryFile:    "app/page.tsx",
		Command:      "npx next dev",
		ReadyTimeout: 250 * time.Millisecond,
	})
	require.NoError(t, err)
	defer proc.Stop()

	status, activeSandbox := handler.getPreviewStatus(projectID, false)

	require.False(t, activeSandbox)
	require.NotNil(t, status)
	require.True(t, status.Active)
	require.Equal(t, projectID, status.ProjectID)
	require.Equal(t, readyServer.URL, status.URL)
}

func TestPreviewHandlerBuildProxyURLUsesForwardedPublicOrigin(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, false)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/preview/proxy", nil)
	context.Request.Host = "internal-preview:8080"
	context.Request.Header.Set("X-Forwarded-Host", "preview.apex-build.dev, internal-preview:8080")
	context.Request.Header.Set("X-Forwarded-Proto", "https, http")

	url := handler.buildProxyURL(context, projectID)
	require.Equal(t, "https://preview.apex-build.dev/api/v1/preview/proxy/"+strconv.FormatUint(uint64(projectID), 10)+"/", url)
}

func TestPreviewHandlerBuildBackendProxyURLUsesForwardedPublicOrigin(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, false)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/preview/backend-proxy", nil)
	context.Request.Host = "internal-backend:8080"
	context.Request.Header.Set("X-Forwarded-Host", "preview.apex-build.dev, internal-backend:8080")
	context.Request.Header.Set("X-Forwarded-Proto", "https, http")

	url := handler.buildBackendProxyURL(context, projectID)
	require.Equal(t, "https://preview.apex-build.dev/api/v1/preview/backend-proxy/"+strconv.FormatUint(uint64(projectID), 10), url)
}

func TestPreviewProxyRewriteDisablesCompressionHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/_next/static/chunks/app.js", nil)
	req.Header.Set("Accept-Encoding", "gzip, br")

	disablePreviewProxyCompression(req)

	require.Empty(t, req.Header.Get("Accept-Encoding"))

	resp := &http.Response{
		Header: http.Header{
			"Content-Encoding": []string{"gzip"},
			"Content-Length":   []string{"999"},
			"ETag":             []string{`"old-etag"`},
			"Last-Modified":    []string{"Tue, 28 Apr 2026 00:00:00 GMT"},
		},
	}

	setRewrittenPreviewResponseBody(resp, "<html>Apex Preview</html>")

	require.Empty(t, resp.Header.Get("Content-Encoding"))
	require.Empty(t, resp.Header.Get("ETag"))
	require.Empty(t, resp.Header.Get("Last-Modified"))
	require.Equal(t, "25", resp.Header.Get("Content-Length"))
	require.Equal(t, int64(25), resp.ContentLength)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<html>Apex Preview</html>", string(body))
}

func TestRewritePreviewHTMLForProxyWithBackendAppendsPreviewTokenToAssets(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, false)

	html := `
<html>
  <head>
    <link rel="stylesheet" href="/__apex_bundle.css">
  </head>
  <body>
    <img src="/logo.svg">
    <script src="/__apex_bundle.js"></script>
  </body>
</html>`

	rewritten := handler.rewritePreviewHTMLForProxyWithBackend(html, projectID, "", "preview-token-123")
	prefix := "/api/v1/preview/proxy/" + strconv.FormatUint(uint64(projectID), 10)

	require.Contains(t, rewritten, `href="`+prefix+`/__apex_bundle.css?preview_token=preview-token-123"`)
	require.Contains(t, rewritten, `src="`+prefix+`/__apex_bundle.js?preview_token=preview-token-123"`)
	require.Contains(t, rewritten, `src="`+prefix+`/logo.svg?preview_token=preview-token-123"`)
	require.Contains(t, rewritten, `window.__APEX_IMPORT_META_ENV__=`)
	require.Contains(t, rewritten, `window.import.meta={env:window.__APEX_IMPORT_META_ENV__};`)
	require.NotContains(t, rewritten, "history.replaceState")
}

func TestRewritePreviewHTMLForBackendRuntimeProxyAppendsPreviewTokenToNextAssets(t *testing.T) {
	handler, _ := newPreviewHandlerTestFixture(t, false)

	html := `
<!doctype html>
<html>
  <head>
    <link rel="preload" href="/_next/static/css/app.css" as="style">
    <link rel="stylesheet" href="/_next/static/css/app.css">
  </head>
  <body>
    <script src="/_next/static/chunks/webpack.js"></script>
    <script src="/_next/static/chunks/app/page.js"></script>
  </body>
</html>`

	prefix := "/api/v1/preview/backend-proxy/99"
	rewritten := handler.rewritePreviewHTMLForProxyWithPrefix(
		html,
		prefix,
		"https://apex-build.dev/api/v1/preview/backend-proxy/99",
		"runtime-token",
	)

	require.Contains(t, rewritten, `href="`+prefix+`/_next/static/css/app.css?preview_token=runtime-token"`)
	require.Contains(t, rewritten, `src="`+prefix+`/_next/static/chunks/webpack.js?preview_token=runtime-token"`)
	require.Contains(t, rewritten, `src="`+prefix+`/_next/static/chunks/app/page.js?preview_token=runtime-token"`)
	require.Contains(t, rewritten, `var _bp="https://apex-build.dev/api/v1/preview/backend-proxy/99";`)
	require.Contains(t, rewritten, `window.__APEX_BACKEND_URL__=_bp;`)
	require.Contains(t, rewritten, `window.__APEX_IMPORT_META_ENV__=`)
}

func TestRewritePreviewJavaScriptForProxyRewritesViteDynamicAssetImports(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, false)
	relativePrefix := "/api/v1/preview/proxy/" + strconv.FormatUint(uint64(projectID), 10)

	js := `const deps=["assets/Dashboard-CVEE8rpR.js","/assets/api-CM3qBsrA.js","https://cdn.example.com/assets/external.js","` + relativePrefix + `/assets/existing.js?preview_token=old-token"];`

	rewritten := handler.rewritePreviewJavaScriptForProxy(js, projectID, "token value")

	require.Contains(t, rewritten, `"`+relativePrefix+`/assets/Dashboard-CVEE8rpR.js?preview_token=token+value"`)
	require.Contains(t, rewritten, `"`+relativePrefix+`/assets/api-CM3qBsrA.js?preview_token=token+value"`)
	require.Contains(t, rewritten, `"https://cdn.example.com/assets/external.js"`)
	require.Contains(t, rewritten, `"`+relativePrefix+`/assets/existing.js?preview_token=old-token"`)
	require.NotContains(t, rewritten, `"assets/Dashboard-CVEE8rpR.js"`)
	require.NotContains(t, rewritten, `"/assets/api-CM3qBsrA.js"`)
}

func TestRewritePreviewJavaScriptForBackendRuntimeProxyRewritesNextChunks(t *testing.T) {
	handler, _ := newPreviewHandlerTestFixture(t, false)
	prefix := "/api/v1/preview/backend-proxy/99"

	js := `self.__next_f.push(["/_next/static/chunks/app/page.js"]);__webpack_require__.p="/_next/";const css="/_next/static/css/app.css";`

	rewritten := handler.rewritePreviewJavaScriptForProxyWithPrefix(js, prefix, "runtime token")

	require.Contains(t, rewritten, `"`+prefix+`/_next/static/chunks/app/page.js?preview_token=runtime+token"`)
	require.Contains(t, rewritten, `__webpack_require__.p="`+prefix+`/_next/";`)
	require.Contains(t, rewritten, `"`+prefix+`/_next/static/css/app.css?preview_token=runtime+token"`)
	require.NotContains(t, rewritten, `"/_next/static/chunks/app/page.js"`)
}

func TestRewritePreviewJavaScriptForProxyNormalizesRelativeViteChunksToPublicProxyURL(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, false)
	prefix := "https://api.apex-build.dev/api/v1/preview/proxy/" + strconv.FormatUint(uint64(projectID), 10)

	js := `import{r as React}from"./index-DZPHYNTJ.js";const View=lazy(()=>import("./Dashboard-CVEE8rpR.js"));const deps=["./api-CM3qBsrA.js","./style.css"];`

	rewritten := handler.rewritePreviewJavaScriptForProxyWithPrefix(js, prefix, "token value")

	require.Contains(t, rewritten, `from"`+prefix+`/assets/index-DZPHYNTJ.js?preview_token=token+value"`)
	require.Contains(t, rewritten, `import("`+prefix+`/assets/Dashboard-CVEE8rpR.js?preview_token=token+value")`)
	require.Contains(t, rewritten, `"`+prefix+`/assets/api-CM3qBsrA.js?preview_token=token+value"`)
	require.Contains(t, rewritten, `"`+prefix+`/assets/style.css?preview_token=token+value"`)
	require.NotContains(t, rewritten, `"./Dashboard-CVEE8rpR.js"`)
	require.NotContains(t, rewritten, `from"./index-DZPHYNTJ.js"`)
}

func TestRewritePreviewJavaScriptForProxyKeepsVitePreloadDepsOriginRelative(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, false)
	prefix := "https://api.apex-build.dev/api/v1/preview/proxy/" + strconv.FormatUint(uint64(projectID), 10)
	preloadPrefix := "api/v1/preview/proxy/" + strconv.FormatUint(uint64(projectID), 10)

	js := `const __vite__mapDeps=(i,m=__vite__mapDeps,d=(m.f||(m.f=["assets/Dashboard-CVEE8rpR.js","assets/api-CM3qBsrA.js"])))=>i.map(i=>d[i]);const View=lazy(()=>import("./Dashboard-CVEE8rpR.js"),__vite__mapDeps([0,1]));`

	rewritten := handler.rewritePreviewJavaScriptForProxyWithPrefix(js, prefix, "token value")

	require.Contains(t, rewritten, `m.f=["`+preloadPrefix+`/assets/Dashboard-CVEE8rpR.js?preview_token=token+value","`+preloadPrefix+`/assets/api-CM3qBsrA.js?preview_token=token+value"]`)
	require.Contains(t, rewritten, `import("`+prefix+`/assets/Dashboard-CVEE8rpR.js?preview_token=token+value")`)
	require.NotContains(t, rewritten, `"/https://api.apex-build.dev`)
	require.NotContains(t, rewritten, `"https://api.apex-build.dev/api/v1/preview/proxy/`+strconv.FormatUint(uint64(projectID), 10)+`/assets/api-CM3qBsrA.js`)
}

func TestPreviewHandlerBuildProxyBaseURLExcludesPreviewToken(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, false)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/preview/proxy?preview_token=token-value", nil)
	context.Request.Host = "api.apex-build.dev"
	context.Request.Header.Set("X-Forwarded-Proto", "https")

	baseURL := handler.buildProxyBaseURL(context, projectID)

	require.Equal(t, "https://api.apex-build.dev/api/v1/preview/proxy/"+strconv.FormatUint(uint64(projectID), 10), baseURL)
	require.NotContains(t, baseURL, "preview_token")
}

func TestApplyPreviewResponseHeadersAllowsSameOriginStorageForHTML(t *testing.T) {
	handler, _ := newPreviewHandlerTestFixture(t, false)

	headers := make(http.Header)
	handler.applyPreviewResponseHeaders(headers, "null", true)

	require.Equal(t, "null", headers.Get("Access-Control-Allow-Origin"))
	require.Contains(t, headers.Get("Content-Security-Policy"), "sandbox")
	require.Contains(t, headers.Get("Content-Security-Policy"), "allow-same-origin")
}

func TestBackendProxyTargetURLPrefersRemoteRuntimeURL(t *testing.T) {
	target, err := backendProxyTargetURL(&preview.ServerStatus{
		Running: true,
		Port:    9100,
		URL:     "https://abc123.preview.e2b.dev",
	})

	require.NoError(t, err)
	require.Equal(t, "https://abc123.preview.e2b.dev", target.String())
}

func TestBackendProxyTargetURLFallsBackToLocalhostPort(t *testing.T) {
	target, err := backendProxyTargetURL(&preview.ServerStatus{
		Running: true,
		Port:    9100,
	})

	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:9100", target.String())
}

func TestPreviewProxyTargetURLPrefersRemoteContainerURL(t *testing.T) {
	target, err := previewProxyTargetURL(&preview.PreviewStatus{
		Active: true,
		Port:   10000,
		URL:    "http://177.7.36.223:10000",
	})

	require.NoError(t, err)
	require.Equal(t, "http://177.7.36.223:10000", target.String())
}

func TestPreviewProxyTargetURLFallsBackToLocalhostPort(t *testing.T) {
	target, err := previewProxyTargetURL(&preview.PreviewStatus{
		Active: true,
		Port:   10000,
	})

	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:10000", target.String())
}

func TestPreviewHandlerDetectFrameworkPrefersNextOverReact(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, false)

	require.NoError(t, handler.db.Create(&models.File{
		ProjectID: projectID,
		Path:      "package.json",
		Type:      "file",
		Content: `{
  "dependencies": {
    "next": "^15.3.2",
    "react": "^18.3.0",
    "react-dom": "^18.3.0"
  }
}`,
	}).Error)

	require.Equal(t, "next", handler.detectFramework(projectID))
}

func TestPreviewHandlerDetectEntryPointFindsNextAppRouterPage(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, false)

	require.NoError(t, handler.db.Create(&models.File{
		ProjectID: projectID,
		Path:      "app/page.tsx",
		Type:      "file",
		Content:   `export default function Page() { return <main>Hello</main> }`,
	}).Error)

	require.Equal(t, "app/page.tsx", handler.detectEntryPoint(projectID))
	require.Equal(t, "app/page.tsx", handler.detectBundleEntryPoint(projectID))
}

func TestMergePreviewEnvVarsOverlaysBackendValues(t *testing.T) {
	merged := mergePreviewEnvVars(
		map[string]string{
			"NEXT_PUBLIC_API_URL": "https://frontend.example/api",
			"SHARED":              "frontend",
		},
		map[string]string{
			"API_SECRET": "secret",
			"SHARED":     "backend",
		},
	)

	require.Equal(t, "https://frontend.example/api", merged["NEXT_PUBLIC_API_URL"])
	require.Equal(t, "secret", merged["API_SECRET"])
	require.Equal(t, "backend", merged["SHARED"])
}
