package agents

import (
	"testing"

	"apex-build/pkg/models"
)

func TestBuildSubscriptionRequirement(t *testing.T) {
	tests := []struct {
		name   string
		req    *BuildRequest
		want   bool
		reason string
	}{
		{
			name: "frontend only stack stays free eligible",
			req: &BuildRequest{
				Description: "Build a responsive landing page for a design studio with a pricing grid and contact section.",
				TechStack:   &TechStack{Frontend: "React"},
			},
			want: false,
		},
		{
			name: "explicit backend stack requires paid",
			req: &BuildRequest{
				Description: "Build a product dashboard.",
				TechStack:   &TechStack{Frontend: "React", Backend: "Go"},
			},
			want:   true,
			reason: "backend services",
		},
		{
			name: "auth and billing requirements require paid",
			req: &BuildRequest{
				Description: "Build a landing page with login, subscriptions, and Stripe checkout.",
			},
			want:   true,
			reason: "authentication flows",
		},
		{
			name: "database stack requires paid",
			req: &BuildRequest{
				Description: "Build a CRM.",
				TechStack:   &TechStack{Frontend: "React", Database: "PostgreSQL"},
			},
			want:   true,
			reason: "database-backed apps",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := buildSubscriptionRequirement(tc.req)
			if got != tc.want {
				t.Fatalf("buildSubscriptionRequirement() = %v, want %v", got, tc.want)
			}
			if tc.reason != "" && reason != tc.reason {
				t.Fatalf("buildSubscriptionRequirement() reason = %q, want %q", reason, tc.reason)
			}
		})
	}
}

func TestIsPaidBuildPlan(t *testing.T) {
	for _, planType := range []string{"builder", "pro", "team", "enterprise", "owner"} {
		if !isPaidBuildPlan(planType) {
			t.Fatalf("expected %s to unlock backend builds", planType)
		}
	}
	if isPaidBuildPlan("free") {
		t.Fatal("free plan must not unlock backend builds")
	}
}

func TestUserHasActiveBYOKKeyRequiresPaidPlan(t *testing.T) {
	db := openBuildTestDB(t)

	freeUser := models.User{
		Username:         "free-byok-user",
		Email:            "free-byok@example.com",
		PasswordHash:     "hash",
		SubscriptionType: "free",
	}
	if err := db.Create(&freeUser).Error; err != nil {
		t.Fatalf("create free user: %v", err)
	}
	if err := db.Create(&models.UserAPIKey{
		UserID:   freeUser.ID,
		Provider: "claude",
		IsActive: true,
	}).Error; err != nil {
		t.Fatalf("create free key: %v", err)
	}

	builderUser := models.User{
		Username:         "builder-byok-user",
		Email:            "builder-byok@example.com",
		PasswordHash:     "hash",
		SubscriptionType: "builder",
	}
	if err := db.Create(&builderUser).Error; err != nil {
		t.Fatalf("create builder user: %v", err)
	}
	if err := db.Create(&models.UserAPIKey{
		UserID:   builderUser.ID,
		Provider: "claude",
		IsActive: true,
	}).Error; err != nil {
		t.Fatalf("create builder key: %v", err)
	}

	manager := &AgentManager{db: db}

	if manager.userHasActiveBYOKKey(freeUser.ID) {
		t.Fatal("free users must not be treated as having active BYOK access")
	}
	if !manager.userHasActiveBYOKKey(builderUser.ID) {
		t.Fatal("builder users with active keys should retain BYOK access")
	}
}
