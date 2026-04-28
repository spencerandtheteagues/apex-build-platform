// Package agents — build cleanup handlers.
//
// Provides force-delete and bulk-purge endpoints for build history. These are
// intentionally separate from handlers.go so build-history hygiene can evolve
// without colliding with the routine list/get/cancel/delete handlers there.
//
// Key behaviors:
//
//   - ForceDeleteBuild bypasses the active-status guard that DeleteBuild
//     enforces. If a build snapshot is left in an "active" state on disk but
//     no live build exists in memory (server crash, abandoned execution),
//     DeleteBuild returns 409. Force delete cancels the live build (best
//     effort), marks the snapshot terminal, and removes it.
//
//   - DeleteAllBuilds purges every build in a user's history in one call so
//     the user can start with a completely clean slate. Requires a
//     ?confirm=DELETE_ALL query string to make accidental purges impossible.
//
//   - Both endpoints also clean satellite rows that key on build_id
//     (PromptPackActivationRequest, PromptPackVersion, PromptPackActivationEvent)
//     so deleted builds leave no residue that could leak context into a
//     subsequent build with a similar prompt.
package agents

import (
	"log"
	"net/http"
	"strings"
	"time"

	appmiddleware "apex-build/internal/middleware"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
)

// RegisterCleanupRoutes wires the cleanup endpoints onto the protected route
// group. Called from main.go alongside RegisterRoutes.
func (h *BuildHandler) RegisterCleanupRoutes(rg *gin.RouterGroup) {
	rg.POST("/builds/:buildId/force-delete", h.ForceDeleteBuild)
	rg.DELETE("/builds", h.DeleteAllBuilds)
}

// ForceDeleteBuild deletes a build regardless of its snapshot status.
// POST /api/v1/builds/:buildId/force-delete
//
// Use this when DeleteBuild returns 409 because a snapshot looks active but
// the live build is gone (typical after a server restart or crash). It will:
//  1. Cancel any live build still in memory.
//  2. Mark the snapshot terminal so the active-status guard cannot re-fire.
//  3. Hard-delete the CompletedBuild row.
//  4. Clean up satellite rows keyed on build_id.
func (h *BuildHandler) ForceDeleteBuild(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "build history not available",
			"details": "Build history is temporarily unavailable because the primary database is offline.",
		})
		return
	}

	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}
	buildID := strings.TrimSpace(c.Param("buildId"))
	if buildID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "build id is required"})
		return
	}

	// Best-effort cancel any in-memory build so workers stop touching it.
	if liveBuild, liveErr := h.manager.GetBuild(buildID); liveErr == nil && liveBuild != nil {
		_ = h.manager.CancelBuild(buildID)
	}

	// Mark the snapshot terminal first. This cannot fail-soft — if the row is
	// genuinely missing we still want to return 404 instead of silently
	// pretending we deleted it.
	var snapshot models.CompletedBuild
	lookupErr := h.db.Where("build_id = ? AND user_id = ?", buildID, uid).First(&snapshot).Error
	if lookupErr != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}

	// Hard-delete the CompletedBuild row (Unscoped so it does not soft-delete).
	if err := h.db.Unscoped().Where("build_id = ? AND user_id = ?", buildID, uid).
		Delete(&models.CompletedBuild{}).Error; err != nil {
		log.Printf("ForceDeleteBuild: failed to delete CompletedBuild %s: %v", buildID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove build"})
		return
	}

	// Clean satellite rows so the deleted build leaves no residue.
	cleanupBuildSideTables(h, buildID)

	// Drop in-memory state so a stale active build can't be resurrected.
	if h.manager != nil {
		if h.manager.editStore != nil {
			h.manager.editStore.Clear(buildID)
		}
		h.manager.ForgetBuild(buildID)
	}
	if h.hub != nil {
		h.hub.CloseAllConnections(buildID)
	}

	log.Printf("ForceDeleteBuild: build %s force-deleted for user %d", buildID, uid)
	c.JSON(http.StatusOK, gin.H{
		"status":   "force_deleted",
		"build_id": buildID,
	})
}

// DeleteAllBuilds purges every build in a user's history. Requires
// ?confirm=DELETE_ALL to prevent accidental purges from misfired clients.
// DELETE /api/v1/builds?confirm=DELETE_ALL
func (h *BuildHandler) DeleteAllBuilds(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "build history not available",
			"details": "Build history is temporarily unavailable because the primary database is offline.",
		})
		return
	}

	uid, ok := appmiddleware.RequireUserID(c)
	if !ok {
		return
	}

	if confirm := strings.TrimSpace(c.Query("confirm")); confirm != "DELETE_ALL" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "confirmation required",
			"details": "Pass ?confirm=DELETE_ALL to purge every saved build for this user. This is intentionally awkward to prevent accidental purges.",
		})
		return
	}

	// Snapshot the user's build IDs before deletion so we can clean satellites.
	var rows []struct {
		BuildID string
	}
	if err := h.db.Table("completed_builds").
		Select("build_id").
		Where("user_id = ?", uid).
		Scan(&rows).Error; err != nil {
		log.Printf("DeleteAllBuilds: enumerate build IDs failed for user %d: %v", uid, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to enumerate builds"})
		return
	}
	buildIDs := make([]string, 0, len(rows))
	for _, r := range rows {
		if id := strings.TrimSpace(r.BuildID); id != "" {
			buildIDs = append(buildIDs, id)
		}
	}

	// Cancel any live builds first.
	for _, id := range buildIDs {
		if liveBuild, liveErr := h.manager.GetBuild(id); liveErr == nil && liveBuild != nil {
			_ = h.manager.CancelBuild(id)
		}
	}

	// Hard-delete all CompletedBuild rows for this user in one shot.
	res := h.db.Unscoped().Where("user_id = ?", uid).Delete(&models.CompletedBuild{})
	if res.Error != nil {
		log.Printf("DeleteAllBuilds: bulk delete failed for user %d: %v", uid, res.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove builds"})
		return
	}

	// Clean satellites and in-memory state for each removed build.
	for _, id := range buildIDs {
		cleanupBuildSideTables(h, id)
		if h.manager != nil {
			if h.manager.editStore != nil {
				h.manager.editStore.Clear(id)
			}
			h.manager.ForgetBuild(id)
		}
		if h.hub != nil {
			h.hub.CloseAllConnections(id)
		}
	}

	log.Printf("DeleteAllBuilds: user %d purged %d builds", uid, res.RowsAffected)
	c.JSON(http.StatusOK, gin.H{
		"status":         "all_deleted",
		"deleted_count":  res.RowsAffected,
		"deleted_at":     time.Now().UTC().Format(time.RFC3339),
		"deleted_builds": buildIDs,
	})
}

// cleanupBuildSideTables removes satellite rows that key on build_id. These
// tables are admin-gated registries that don't currently affect live prompt
// generation (LivePromptReadEnabled is false everywhere), but cleaning them on
// delete prevents future activation paths from re-introducing residue from
// builds the user has explicitly removed.
func cleanupBuildSideTables(h *BuildHandler, buildID string) {
	if h == nil || h.db == nil || strings.TrimSpace(buildID) == "" {
		return
	}

	tables := []struct {
		name  string
		model interface{}
		where string
	}{
		{"prompt_pack_activation_requests", &models.PromptPackActivationRequest{}, "build_id = ?"},
		{"prompt_pack_versions", &models.PromptPackVersion{}, "source_build_id = ?"},
		{"prompt_pack_activation_events", &models.PromptPackActivationEvent{}, "build_id = ?"},
	}

	for _, t := range tables {
		if err := h.db.Unscoped().Where(t.where, buildID).Delete(t.model).Error; err != nil {
			// Log but don't fail — these are best-effort cleanups. The primary
			// CompletedBuild row is already gone at this point.
			log.Printf("cleanupBuildSideTables: %s cleanup for build %s failed: %v", t.name, buildID, err)
		}
	}
}
