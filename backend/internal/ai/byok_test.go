package ai

import (
	"testing"

	"apex-build/internal/secrets"
	"apex-build/pkg/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestParseOllamaCredentialAcceptsCloudKeyWithBaseURL(t *testing.T) {
	t.Setenv("OLLAMA_BASE_URL", "")

	baseURL, apiKey := parseOllamaCredential("sk-test / OLLAMA_BASE_URL:https://ollama.com")
	if baseURL != "https://ollama.com" {
		t.Fatalf("baseURL = %q, want https://ollama.com", baseURL)
	}
	if apiKey != "sk-test" {
		t.Fatalf("apiKey = %q, want sk-test", apiKey)
	}
}

func TestParseOllamaCredentialKeepsURLOnlyLocalMode(t *testing.T) {
	t.Setenv("OLLAMA_BASE_URL", "https://ollama.com")

	baseURL, apiKey := parseOllamaCredential("http://localhost:11434")
	if baseURL != "http://localhost:11434" {
		t.Fatalf("baseURL = %q, want http://localhost:11434", baseURL)
	}
	if apiKey != "" {
		t.Fatalf("apiKey = %q, want empty local key", apiKey)
	}
}

func TestBYOKRouterWithOllamaKeyDoesNotInheritPlatformProviders(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.UserAPIKey{}, &models.AIUsageLog{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	user := models.User{
		Username:           "byok-ollama-user",
		Email:              "byok-ollama@example.com",
		SubscriptionType:   "builder",
		SubscriptionStatus: "active",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	secretsManager, err := secrets.NewSecretsManager("test-master-key-for-byok-ollama")
	if err != nil {
		t.Fatalf("new secrets manager: %v", err)
	}

	platformRouter := &AIRouter{
		clients: map[AIProvider]AIClient{
			ProviderClaude:   &stubProviderClient{provider: ProviderClaude},
			ProviderGPT4:     &stubProviderClient{provider: ProviderGPT4},
			ProviderGemini:   &stubProviderClient{provider: ProviderGemini},
			ProviderGrok:     &stubProviderClient{provider: ProviderGrok},
			ProviderDeepSeek: &stubProviderClient{provider: ProviderDeepSeek},
			ProviderGLM:      &stubProviderClient{provider: ProviderGLM},
			ProviderOllama:   &stubProviderClient{provider: ProviderOllama},
		},
		config: DefaultRouterConfig(),
	}

	manager := NewBYOKManager(db, secretsManager, platformRouter)
	if err := manager.SaveKey(user.ID, string(ProviderOllama), "http://localhost:11434", "kimi-k2.6"); err != nil {
		t.Fatalf("save byok ollama key: %v", err)
	}

	router, hasBYOK, err := manager.GetRouterForUser(user.ID)
	if err != nil {
		t.Fatalf("GetRouterForUser: %v", err)
	}
	if !hasBYOK {
		t.Fatal("expected hasBYOK=true")
	}
	if _, ok := router.clients[ProviderOllama]; !ok {
		t.Fatal("expected user's BYOK Ollama client to be available")
	}
	for _, provider := range []AIProvider{ProviderClaude, ProviderGPT4, ProviderGemini, ProviderGrok, ProviderDeepSeek, ProviderGLM} {
		if _, ok := router.clients[provider]; ok {
			t.Fatalf("strict BYOK router inherited platform provider %s", provider)
		}
	}

	selected, err := router.selectProvider(&AIRequest{Capability: CapabilityArchitecture, PowerMode: "max"})
	if err != nil {
		t.Fatalf("selectProvider: %v", err)
	}
	if selected != ProviderOllama {
		t.Fatalf("selectProvider = %s, want %s", selected, ProviderOllama)
	}
}
