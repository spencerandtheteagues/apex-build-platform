package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openAPISubscriptionGuardTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}

func TestHasPaidBackendPlanRequiresActiveStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAPISubscriptionGuardTestDB(t)
	user := models.User{
		Username:           "past-due-api-builder",
		Email:              "past-due-api-builder@example.com",
		PasswordHash:       "hash",
		SubscriptionType:   "builder",
		SubscriptionStatus: "past_due",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("subscription_type", "builder")
	c.Set("subscription_status", "active")

	if hasPaidBackendPlan(c, db, user.ID) {
		t.Fatal("past-due DB subscription must not unlock paid backend features even if JWT context is stale")
	}

	if err := db.Model(&user).Update("subscription_status", "active").Error; err != nil {
		t.Fatalf("update subscription status: %v", err)
	}
	if !hasPaidBackendPlan(c, db, user.ID) {
		t.Fatal("active builder subscription should unlock paid backend features")
	}
}
