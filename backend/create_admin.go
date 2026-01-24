//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"apex-build/internal/auth"
	"apex-build/pkg/models"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	fmt.Println("🚀 APEX.BUILD Admin Account Creation")
	fmt.Println("=====================================")

	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
		log.Println("Continuing with system environment variables...")
	}

	// Get database URL from environment
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	// Connect to database
	fmt.Println("📡 Connecting to database...")
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto-migrate the User model if needed
	fmt.Println("🔄 Ensuring database schema is up to date...")
	err = db.AutoMigrate(&models.User{})
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Admin account details
	adminEmail := "spencerandtheteagues@gmail.com"
	adminPassword := "THE$T@R$H1PKEY!"
	adminUsername := "apex_admin"
	adminFullName := "APEX.BUILD System Administrator"

	// Check if admin already exists
	fmt.Printf("🔍 Checking if admin user %s already exists...\n", adminEmail)
	var existingUser models.User
	result := db.Where("email = ? OR username = ?", adminEmail, adminUsername).First(&existingUser)

	if result.Error == nil {
		fmt.Printf("⚠️  User with email %s or username %s already exists!\n", adminEmail, adminUsername)
		fmt.Printf("User ID: %d\n", existingUser.ID)
		fmt.Printf("Username: %s\n", existingUser.Username)
		fmt.Printf("Email: %s\n", existingUser.Email)
		fmt.Printf("Subscription: %s\n", existingUser.SubscriptionType)

		// Update existing user to admin privileges
		fmt.Println("🔧 Updating existing user to admin privileges...")
		updates := map[string]interface{}{
			"subscription_type": "enterprise",
			"is_verified":       true,
			"is_active":         true,
			"subscription_end":  time.Now().AddDate(10, 0, 0), // 10 years from now
			"full_name":         adminFullName,
		}

		result = db.Model(&existingUser).Updates(updates)
		if result.Error != nil {
			log.Fatalf("Failed to update user to admin: %v", result.Error)
		}

		fmt.Println("✅ Existing user updated to admin privileges successfully!")
		fmt.Printf("Admin User ID: %d\n", existingUser.ID)
		fmt.Printf("Email: %s\n", existingUser.Email)
		fmt.Printf("Username: %s\n", existingUser.Username)
		fmt.Printf("Subscription: %s\n", existingUser.SubscriptionType)
		return
	}

	// Create new admin user
	fmt.Printf("👤 Creating new admin user: %s\n", adminEmail)

	// Initialize auth service
	authService := auth.NewAuthService("apex_build_jwt_secret_key_2024_ultra_secure_production")

	// Hash the password
	fmt.Println("🔐 Hashing admin password...")
	hashedPassword, err := authService.HashPassword(adminPassword)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Create admin user
	adminUser := models.User{
		Username:          adminUsername,
		Email:             adminEmail,
		PasswordHash:      hashedPassword,
		FullName:          adminFullName,
		IsActive:          true,
		IsVerified:        true,
		SubscriptionType:  "enterprise", // Highest tier
		SubscriptionEnd:   time.Now().AddDate(10, 0, 0), // 10 years from now
		PreferredTheme:    "cyberpunk",
		PreferredAI:       "auto",
	}

	// Save to database
	fmt.Println("💾 Saving admin user to database...")
	result = db.Create(&adminUser)
	if result.Error != nil {
		log.Fatalf("Failed to create admin user: %v", result.Error)
	}

	fmt.Println("\n✅ ADMIN ACCOUNT CREATED SUCCESSFULLY!")
	fmt.Println("=====================================")
	fmt.Printf("User ID: %d\n", adminUser.ID)
	fmt.Printf("Username: %s\n", adminUser.Username)
	fmt.Printf("Email: %s\n", adminUser.Email)
	fmt.Printf("Full Name: %s\n", adminUser.FullName)
	fmt.Printf("Subscription: %s\n", adminUser.SubscriptionType)
	fmt.Printf("Verified: %v\n", adminUser.IsVerified)
	fmt.Printf("Active: %v\n", adminUser.IsActive)
	fmt.Printf("Subscription End: %v\n", adminUser.SubscriptionEnd.Format("2006-01-02"))
	fmt.Printf("Created: %v\n", adminUser.CreatedAt.Format("2006-01-02 15:04:05"))

	fmt.Println("\n🔑 LOGIN CREDENTIALS:")
	fmt.Println("=====================")
	fmt.Printf("Email: %s\n", adminEmail)
	fmt.Printf("Password: %s\n", adminPassword)

	fmt.Println("\n🎯 ADMIN CAPABILITIES:")
	fmt.Println("======================")
	fmt.Println("✅ Enterprise subscription (highest tier)")
	fmt.Println("✅ Unlimited projects")
	fmt.Println("✅ Unlimited AI requests")
	fmt.Println("✅ Access to all AI models")
	fmt.Println("✅ Full platform access")
	fmt.Println("✅ 10-year subscription (effectively permanent)")

	fmt.Println("\n🚀 PLATFORM ACCESS:")
	fmt.Println("===================")
	fmt.Println("Frontend: http://localhost:3001")
	fmt.Println("Backend API: http://localhost:8080")
	fmt.Println("Health Check: http://localhost:8080/health")

	fmt.Println("\n🎉 APEX.BUILD platform is ready for admin access!")
}