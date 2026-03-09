package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"apex-build/internal/collaboration"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCollaborationHandlerJoinRoomDeniesUnauthorizedUsers(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	handler := NewCollaborationHandler(
		collaboration.NewCollabHub(),
		func(userID, projectID uint) (*collaboration.ProjectAccess, error) {
			return nil, collaboration.ErrProjectAccessDenied
		},
	)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "projectId", Value: "12"}}
	context.Request = httptest.NewRequest(http.MethodPost, "/collab/join/12", nil)
	context.Set("user_id", uint(42))

	handler.JoinRoom(context)

	require.Equal(t, http.StatusForbidden, recorder.Code)

	var payload StandardResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.False(t, payload.Success)
	require.Equal(t, "ACCESS_DENIED", payload.Code)
}

func TestCollaborationHandlerJoinRoomReturnsRoomBootstrap(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	handler := NewCollaborationHandler(
		collaboration.NewCollabHub(),
		func(userID, projectID uint) (*collaboration.ProjectAccess, error) {
			return &collaboration.ProjectAccess{
				ProjectID:  projectID,
				RoomID:     collaboration.ProjectRoomID(projectID),
				Permission: collaboration.PermissionOwner,
			}, nil
		},
	)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "projectId", Value: "7"}}
	context.Request = httptest.NewRequest(http.MethodPost, "/collab/join/7", nil)
	context.Set("user_id", uint(42))

	handler.JoinRoom(context)

	require.Equal(t, http.StatusOK, recorder.Code)

	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			RoomID    string `json:"room_id"`
			ProjectID uint   `json:"project_id"`
			Users     []any  `json:"users"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, "project_7", payload.Data.RoomID)
	require.Equal(t, uint(7), payload.Data.ProjectID)
	require.Empty(t, payload.Data.Users)
}

func TestCollaborationHandlerGetUsersRejectsInvalidRoomIDs(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	handler := NewCollaborationHandler(
		collaboration.NewCollabHub(),
		func(userID, projectID uint) (*collaboration.ProjectAccess, error) {
			return nil, errors.New("should not be called")
		},
	)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "roomId", Value: "bad-room"}}
	context.Request = httptest.NewRequest(http.MethodGet, "/collab/users/bad-room", nil)
	context.Set("user_id", uint(42))

	handler.GetUsers(context)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
