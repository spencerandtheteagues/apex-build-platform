package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

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

func TestPreviewHandlerRejectsExplicitSandboxWithoutDocker(t *testing.T) {
	useSandbox, err := (&PreviewHandler{}).resolveRequestedPreviewSandbox(true)
	require.Error(t, err)
	require.False(t, useSandbox)
}

func TestPreviewHandlerStartPreviewFallsBackWhenSandboxRequiredButDockerUnavailable(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, true)

	body, err := json.Marshal(map[string]any{
		"project_id": projectID,
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

func TestPreviewHandlerBuildProxyURLUsesForwardedPublicOrigin(t *testing.T) {
	handler, projectID := newPreviewHandlerTestFixture(t, false)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/preview/proxy", nil)
	context.Request.Host = "internal-preview:8080"
	context.Request.Header.Set("X-Forwarded-Host", "preview.apex-build.dev, internal-preview:8080")
	context.Request.Header.Set("X-Forwarded-Proto", "https, http")

	url := handler.buildProxyURL(context, projectID)
	require.Equal(t, "https://preview.apex-build.dev/api/v1/preview/proxy/"+strconv.FormatUint(uint64(projectID), 10), url)
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
