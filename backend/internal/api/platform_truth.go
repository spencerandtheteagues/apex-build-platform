package api

import (
	"net/http"

	"apex-build/internal/payments"

	"github.com/gin-gonic/gin"
)

type PlatformTruth struct {
	Version   string          `json:"version"`
	Stack     StackTruth      `json:"stack"`
	Plans     []payments.Plan `json:"plans"`
	Features  []FeatureTruth  `json:"features"`
	Readiness any             `json:"readiness"`
}

type StackTruth struct {
	BackendGo string `json:"backend_go"`
	Node      string `json:"node"`
	Frontend  string `json:"frontend"`
	Database  string `json:"database"`
	Cache     string `json:"cache"`
}

type FeatureTruth struct {
	Key    string `json:"key"`
	Status string `json:"status"`
	Source string `json:"source"`
}

func (s *Server) PlatformTruth(c *gin.Context) {
	c.JSON(http.StatusOK, PlatformTruth{
		Version: "1.0.0",
		Stack: StackTruth{
			BackendGo: "1.26+",
			Node:      "20+",
			Frontend:  "React 18, TypeScript 4.9, Vite 4",
			Database:  "PostgreSQL 15",
			Cache:     "Redis 7 with memory fallback",
		},
		Plans: payments.GetAllPlans(),
		Features: []FeatureTruth{
			{Key: "feature_readiness", Status: "live", Source: "/api/v1/health/features"},
			{Key: "billing_plans", Status: "live", Source: "/api/v1/billing/plans"},
			{Key: "mobile_source_generation", Status: "flagged_beta", Source: "MOBILE_BUILDER_ENABLED"},
			{Key: "mobile_eas_builds", Status: "gated", Source: "MOBILE_EAS_BUILD_ENABLED"},
			{Key: "mobile_store_submission", Status: "gated", Source: "MOBILE_EAS_SUBMIT_ENABLED"},
		},
		Readiness: s.runtimeReadinessSummary(true),
	})
}
