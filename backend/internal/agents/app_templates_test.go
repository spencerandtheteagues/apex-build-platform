package agents

import "testing"

func TestDetectAppTemplateDoesNotTreatEmbeddedMockAgentsAsAISaaS(t *testing.T) {
	t.Parallel()

	description := `Build Apex FieldOps AI, a contractor field operations platform with dashboard metrics,
job pipeline, estimate builder, crew management, settings, and an Estimate Swarm modal with simulated
Kimi K2.6, GLM-5.1, and DeepSeek V4 agent panels. All data must be in memory. No database, no external
APIs, and no real API keys are needed.`

	tmpl := DetectAppTemplate(description)
	if tmpl != nil && tmpl.ID == "ai-saas" {
		t.Fatalf("embedded/mock agent feature incorrectly selected AI SaaS template: %+v", tmpl)
	}
}

func TestDetectAppTemplateKeepsExplicitAISaaSIntent(t *testing.T) {
	t.Parallel()

	description := `Build an AI SaaS app for document analysis with BYOK, provider routing,
token usage tracking, generation history, and a model selector.`

	tmpl := DetectAppTemplate(description)
	if tmpl == nil || tmpl.ID != "ai-saas" {
		t.Fatalf("expected explicit AI SaaS prompt to select ai-saas template, got %+v", tmpl)
	}
}
