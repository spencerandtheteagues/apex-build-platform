package agents

import (
	"net/http"

	"apex-build/internal/architecture"
	appmiddleware "apex-build/internal/middleware"

	"github.com/gin-gonic/gin"
)

func (h *BuildHandler) RegisterArchitectureAdminRoutes(rg *gin.RouterGroup) {
	arch := rg.Group("/architecture")
	{
		arch.GET("/map", h.GetAdminArchitectureMap)
	}
}

func (h *BuildHandler) GetAdminArchitectureMap(c *gin.Context) {
	var telemetry *architecture.ReferenceTelemetry
	if h != nil && h.manager != nil {
		telemetry = h.manager.ArchitectureReferenceTelemetrySnapshot()
	}
	m, err := architecture.GenerateMap("", telemetry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate architecture map", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"map": m})
}

func (h *BuildHandler) GetBuildArchitectureReferences(c *gin.Context) {
	buildID := c.Param("id")
	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	build, err := h.getLiveBuildForRead(buildID)
	if err == nil {
		build.mu.RLock()
		ownerID := build.UserID
		refs := architecture.CloneReferenceTelemetry(build.SnapshotState.ArchitectureReferences)
		build.mu.RUnlock()
		if ownerID != uid {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"references": firstNonNilTelemetry(refs)})
		return
	}

	snapshot, snapErr := h.getBuildSnapshot(uid, buildID)
	if snapErr != nil {
		if err != nil {
			writeBuildLookupError(c, err, err)
			return
		}
		writeBuildLookupError(c, snapErr, snapErr)
		return
	}
	if snapshot == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}
	if snapshot.UserID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}
	state := parseBuildSnapshotState(snapshot.StateJSON)
	c.JSON(http.StatusOK, gin.H{"references": firstNonNilTelemetry(state.ArchitectureReferences)})
}

func firstNonNilTelemetry(refs *architecture.ReferenceTelemetry) *architecture.ReferenceTelemetry {
	if refs != nil {
		return refs
	}
	return &architecture.ReferenceTelemetry{}
}
