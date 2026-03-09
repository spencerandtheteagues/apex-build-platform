package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"apex-build/internal/collaboration"
	"apex-build/internal/middleware"

	"github.com/gin-gonic/gin"
)

type CollaborationHandler struct {
	hub            *collaboration.CollabHub
	accessResolver collaboration.AccessResolver
}

func NewCollaborationHandler(hub *collaboration.CollabHub, accessResolver collaboration.AccessResolver) *CollaborationHandler {
	return &CollaborationHandler{
		hub:            hub,
		accessResolver: accessResolver,
	}
}

func (h *CollaborationHandler) JoinRoom(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectID, err := strconv.ParseUint(c.Param("projectId"), 10, 32)
	if err != nil || projectID == 0 {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	access, err := h.resolveProjectAccess(userID, uint(projectID))
	if err != nil {
		switch {
		case errors.Is(err, collaboration.ErrProjectNotFound):
			c.JSON(http.StatusNotFound, StandardResponse{Success: false, Error: "Project not found", Code: "PROJECT_NOT_FOUND"})
		case errors.Is(err, collaboration.ErrProjectAccessDenied):
			c.JSON(http.StatusForbidden, StandardResponse{Success: false, Error: "Access denied", Code: "ACCESS_DENIED"})
		default:
			c.JSON(http.StatusInternalServerError, StandardResponse{Success: false, Error: "Failed to prepare collaboration room", Code: "COLLAB_ROOM_ERROR"})
		}
		return
	}

	users := h.hub.GetRoomUsers(access.RoomID)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]any{
			"room_id":    access.RoomID,
			"project_id": access.ProjectID,
			"permission": access.Permission,
			"users":      users,
			"user_count": len(users),
		},
		Message: "Ready to join collaboration room",
	})
}

func (h *CollaborationHandler) LeaveRoom(c *gin.Context) {
	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Left collaboration room",
	})
}

func (h *CollaborationHandler) GetUsers(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	roomID := c.Param("roomId")
	projectID, err := collaboration.ProjectIDFromRoomID(roomID)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid collaboration room",
			Code:    "INVALID_ROOM_ID",
		})
		return
	}

	if _, err := h.resolveProjectAccess(userID, projectID); err != nil {
		switch {
		case errors.Is(err, collaboration.ErrProjectNotFound):
			c.JSON(http.StatusNotFound, StandardResponse{Success: false, Error: "Project not found", Code: "PROJECT_NOT_FOUND"})
		case errors.Is(err, collaboration.ErrProjectAccessDenied):
			c.JSON(http.StatusForbidden, StandardResponse{Success: false, Error: "Access denied", Code: "ACCESS_DENIED"})
		default:
			c.JSON(http.StatusInternalServerError, StandardResponse{Success: false, Error: "Failed to load collaboration users", Code: "COLLAB_USERS_ERROR"})
		}
		return
	}

	users := h.hub.GetRoomUsers(roomID)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]any{
			"room_id":    roomID,
			"project_id": projectID,
			"users":      users,
			"user_count": len(users),
		},
	})
}

func (h *CollaborationHandler) resolveProjectAccess(userID, projectID uint) (*collaboration.ProjectAccess, error) {
	if h.accessResolver == nil {
		return nil, errors.New("collaboration access resolver is not configured")
	}
	return h.accessResolver(userID, projectID)
}
