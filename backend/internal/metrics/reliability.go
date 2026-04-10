package metrics

import (
	"regexp"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	reliabilityLabelSanitizer = regexp.MustCompile(`[^a-z0-9_]+`)

	buildFinalizationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "reliability",
			Name:      "build_finalizations_total",
			Help:      "Total number of build finalizations by status, mode, and reason",
		},
		[]string{"status", "mode", "reason"},
	)

	buildStallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "reliability",
			Name:      "build_stalls_total",
			Help:      "Total number of builds marked failed due to stall detection",
		},
		[]string{"status", "mode"},
	)

	previewStartsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "reliability",
			Name:      "preview_starts_total",
			Help:      "Total preview start attempts by kind, result, and sandbox mode",
		},
		[]string{"kind", "result", "sandbox"},
	)

	previewBackendStartsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "reliability",
			Name:      "preview_backend_starts_total",
			Help:      "Total backend preview server start attempts by result",
		},
		[]string{"result"},
	)

	previewBackendProcessExitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "reliability",
			Name:      "preview_backend_process_exits_total",
			Help:      "Total backend preview process exits by reason",
		},
		[]string{"reason"},
	)

	verificationReportsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "reliability",
			Name:      "verification_reports_total",
			Help:      "Total verification reports emitted by phase, surface, and status",
		},
		[]string{"phase", "surface", "status"},
	)

	verificationWarningsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "reliability",
			Name:      "verification_warnings_total",
			Help:      "Total advisory verification warnings by class and phase",
		},
		[]string{"class", "phase"},
	)

	buildCanaryCohortsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "reliability",
			Name:      "build_canary_cohorts_total",
			Help:      "Total finalized builds matching the platform canary cohorts",
		},
		[]string{"cohort", "status"},
	)
)

func RecordBuildFinalization(status, mode, reason string) {
	buildFinalizationsTotal.WithLabelValues(
		sanitizeReliabilityLabel(status, "unknown"),
		sanitizeReliabilityLabel(mode, "unknown"),
		sanitizeReliabilityLabel(reason, "unknown"),
	).Inc()
}

func RecordBuildStall(status, mode string) {
	buildStallsTotal.WithLabelValues(
		sanitizeReliabilityLabel(status, "unknown"),
		sanitizeReliabilityLabel(mode, "unknown"),
	).Inc()
}

func RecordPreviewStart(kind, result string, sandbox bool) {
	sandboxLabel := "false"
	if sandbox {
		sandboxLabel = "true"
	}
	previewStartsTotal.WithLabelValues(
		sanitizeReliabilityLabel(kind, "unknown"),
		sanitizeReliabilityLabel(result, "unknown"),
		sandboxLabel,
	).Inc()
}

func RecordPreviewBackendStart(result string) {
	previewBackendStartsTotal.WithLabelValues(
		sanitizeReliabilityLabel(result, "unknown"),
	).Inc()
}

func RecordPreviewBackendProcessExit(reason string) {
	previewBackendProcessExitsTotal.WithLabelValues(
		sanitizeReliabilityLabel(reason, "unknown"),
	).Inc()
}

func RecordVerificationReport(phase, surface, status string, warnings []string) {
	phaseLabel := sanitizeReliabilityLabel(phase, "unknown")
	verificationReportsTotal.WithLabelValues(
		phaseLabel,
		sanitizeReliabilityLabel(surface, "unknown"),
		sanitizeReliabilityLabel(status, "unknown"),
	).Inc()

	for _, warning := range warnings {
		class := classifyVerificationWarning(warning)
		if class == "" {
			continue
		}
		verificationWarningsTotal.WithLabelValues(class, phaseLabel).Inc()
	}
}

func RecordBuildCanaryCohort(cohort, status string) {
	buildCanaryCohortsTotal.WithLabelValues(
		sanitizeReliabilityLabel(cohort, "unknown"),
		sanitizeReliabilityLabel(status, "unknown"),
	).Inc()
}

func sanitizeReliabilityLabel(raw, fallback string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return fallback
	}
	s = reliabilityLabelSanitizer.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		return fallback
	}
	if len(s) > 63 {
		s = s[:63]
	}
	return s
}

func classifyVerificationWarning(raw string) string {
	warning := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.HasPrefix(warning, "visual:"):
		return "visual_layout"
	case strings.HasPrefix(warning, "interaction:"):
		return "interaction_canary"
	default:
		return ""
	}
}
