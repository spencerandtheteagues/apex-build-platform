package api

import (
	"net/http"
	"sync"
	"time"

	"apex-build/internal/ai"
	appmiddleware "apex-build/internal/middleware"

	"github.com/gin-gonic/gin"
)

var modelCache struct {
	sync.RWMutex
	models    []ai.LiveModelInfo
	fetchedAt time.Time
}

const modelCacheTTL = 5 * time.Minute

// GetOpenRouterModels returns all OpenRouter models merged with internal quality scores.
// Cached for 5 minutes. Falls back to the curated catalog if the live API is unreachable.
func (s *Server) GetOpenRouterModels(c *gin.Context) {
	_, exists := appmiddleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "not authenticated"})
		return
	}

	// Serve from cache if fresh
	modelCache.RLock()
	if time.Since(modelCache.fetchedAt) < modelCacheTTL && len(modelCache.models) > 0 {
		models := modelCache.models
		modelCache.RUnlock()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    gin.H{"models": models, "total": len(models), "source": "cache"},
		})
		return
	}
	modelCache.RUnlock()

	client, ok := s.aiRouter.GetClient(ai.ProviderOpenRouter)
	if !ok {
		// OpenRouter not configured — return catalog only
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    gin.H{"models": catalogAsLiveInfo(), "total": len(ai.OpenRouterCatalog()), "source": "catalog"},
		})
		return
	}

	orClient, ok := client.(*ai.OpenRouterClient)
	if !ok {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    gin.H{"models": catalogAsLiveInfo(), "total": len(ai.OpenRouterCatalog()), "source": "catalog"},
		})
		return
	}

	models, err := orClient.FetchLiveModels(c.Request.Context())
	if err != nil {
		// API unreachable — serve catalog
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    gin.H{"models": catalogAsLiveInfo(), "total": len(ai.OpenRouterCatalog()), "source": "catalog_fallback"},
		})
		return
	}

	modelCache.Lock()
	modelCache.models = models
	modelCache.fetchedAt = time.Now()
	modelCache.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"models": models, "total": len(models), "source": "live"},
	})
}

func catalogAsLiveInfo() []ai.LiveModelInfo {
	catalog := ai.OpenRouterCatalog()
	out := make([]ai.LiveModelInfo, 0, len(catalog))
	for _, m := range catalog {
		out = append(out, ai.LiveModelInfo{
			ID:            m.ID,
			Name:          m.Name,
			ContextWindow: m.ContextWindow,
			InputPer1M:    m.InputPer1M,
			OutputPer1M:   m.OutputPer1M,
			IsFree:        m.IsFree,
			QualityCode:   m.QualityCode,
			QualityReason: m.QualityReason,
			SpeedRating:   m.SpeedRating,
			Tier:          m.Tier,
			Tags:          m.Tags,
		})
	}
	return out
}
