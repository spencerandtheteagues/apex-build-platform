package websocket

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestHandleWebSocketRejectsMissingUserContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hub := NewHub()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/ws?room_id=test-room", nil)

	hub.HandleWebSocket(context)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "AUTH_REQUIRED")
}

func TestHandleWebSocketRejectsInvalidUserContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hub := NewHub()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/ws?room_id=test-room", nil)
	context.Set("user_id", "not-a-uint")

	hub.HandleWebSocket(context)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "AUTH_REQUIRED")
}
