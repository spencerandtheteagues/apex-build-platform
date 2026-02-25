package handlers

import (
	"net/http"
	"strconv"
	"time"

	"apex-build/internal/spend"

	"github.com/gin-gonic/gin"
)

// SpendHandler exposes spend tracking endpoints.
type SpendHandler struct {
	tracker *spend.SpendTracker
}

// NewSpendHandler creates a new SpendHandler.
func NewSpendHandler(tracker *spend.SpendTracker) *SpendHandler {
	return &SpendHandler{tracker: tracker}
}

// GetSummary returns the daily and monthly spend summary for the authenticated user.
// GET /spend/summary
func (h *SpendHandler) GetSummary(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	summary, err := h.tracker.GetSummary(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get spend summary",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    summary,
	})
}

// GetBreakdown returns spend broken down by a grouping dimension.
// GET /spend/breakdown?group_by=provider&day=2026-02-25&month=2026-02
func (h *SpendHandler) GetBreakdown(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	opts := spend.BreakdownOpts{
		GroupBy:  c.DefaultQuery("group_by", "provider"),
		UserID:   userID,
		DayKey:   c.Query("day"),
		MonthKey: c.Query("month"),
		BuildID:  c.Query("build_id"),
	}

	if pidStr := c.Query("project_id"); pidStr != "" {
		if pid, err := strconv.ParseUint(pidStr, 10, 64); err == nil {
			pidUint := uint(pid)
			opts.ProjectID = &pidUint
		}
	}

	items, err := h.tracker.GetBreakdown(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get spend breakdown",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    items,
	})
}

// GetHistory returns paginated spend history for the authenticated user.
// GET /spend/history?page=1&limit=20
func (h *SpendHandler) GetHistory(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	page := 1
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := (page - 1) * limit

	events, total, err := h.tracker.GetHistory(userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get spend history",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"events":      events,
			"page":        page,
			"limit":       limit,
			"total_count": total,
		},
	})
}

// GetBuildSpend returns spend details for a specific build.
// GET /spend/build/:id
func (h *SpendHandler) GetBuildSpend(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	buildID := c.Param("id")
	if buildID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Build ID is required",
		})
		return
	}

	total, events, err := h.tracker.GetBuildSpend(buildID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get build spend",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"build_id":    buildID,
			"total_spend": total,
			"events":      events,
		},
	})
}

// ExportCSV returns a CSV file of spend events for a date range.
// GET /spend/export/csv?from=2026-01-01&to=2026-02-25
func (h *SpendHandler) ExportCSV(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	fromStr := c.Query("from")
	toStr := c.Query("to")

	now := time.Now().UTC()

	// Default: last 30 days
	from := now.AddDate(0, 0, -30)
	to := now

	if fromStr != "" {
		if parsed, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = parsed
		}
	}
	if toStr != "" {
		if parsed, err := time.Parse("2006-01-02", toStr); err == nil {
			// Include the entire "to" day
			to = parsed.Add(24*time.Hour - time.Nanosecond)
		}
	}

	csvData, err := h.tracker.ExportCSV(userID, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to export spend data",
		})
		return
	}

	filename := "apex-spend-" + from.Format("20060102") + "-" + to.Format("20060102") + ".csv"
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", csvData)
}

// RegisterRoutes registers all spend endpoints under the given router group.
func (h *SpendHandler) RegisterRoutes(rg *gin.RouterGroup) {
	sg := rg.Group("/spend")
	{
		sg.GET("/summary", h.GetSummary)
		sg.GET("/breakdown", h.GetBreakdown)
		sg.GET("/history", h.GetHistory)
		sg.GET("/build/:id", h.GetBuildSpend)
		sg.GET("/export/csv", h.ExportCSV)
	}
}
