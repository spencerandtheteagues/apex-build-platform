package db

import (
	"log"
	"os"
	"time"

	"apex-build/pkg/models"

	"golang.org/x/crypto/bcrypt"
)

// SECURITY: Get seed passwords from environment variables
func getSeedPassword(envVar, defaultDev string) string {
	password := os.Getenv(envVar)
	if password != "" {
		return password
	}
	// Only use default in non-production
	if os.Getenv("ENVIRONMENT") == "production" {
		log.Printf("⚠️  WARNING: %s not set in production - seed user will not be created", envVar)
		return ""
	}
	return defaultDev
}

// SeedAdminUser creates the default admin account if it doesn't exist
func (d *Database) SeedAdminUser() error {
	// Check if admin already exists
	var existingAdmin models.User
	result := d.DB.Where("username = ?", "admin").First(&existingAdmin)

	if result.Error == nil {
		log.Println("✅ Admin user already exists")
		// Update admin privileges in case they were changed
		d.DB.Model(&existingAdmin).Updates(map[string]interface{}{
			"is_admin":             true,
			"is_super_admin":       true,
			"has_unlimited_credits": true,
			"bypass_billing":       true,
			"bypass_rate_limits":   true,
			"subscription_type":   "owner",
			"is_verified":         true,
			"is_active":           true,
		})
		return nil
	}

	// SECURITY: Get password from environment variable
	password := getSeedPassword("ADMIN_SEED_PASSWORD", "admin-dev-password")
	if password == "" {
		log.Println("⚠️  Skipping admin user creation - ADMIN_SEED_PASSWORD not set")
		return nil
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Create admin user
	admin := models.User{
		Username:            "admin",
		Email:               "admin@apex.build",
		PasswordHash:        string(hashedPassword),
		FullName:            "APEX Admin",
		IsActive:            true,
		IsVerified:          true,
		IsAdmin:             true,
		IsSuperAdmin:        true,
		HasUnlimitedCredits: true,
		BypassBilling:       true,
		BypassRateLimits:    true,
		SubscriptionType:    "owner",
		SubscriptionEnd:     time.Now().AddDate(100, 0, 0), // 100 years from now
		CreditBalance:       999999999.0,
		PreferredTheme:      "cyberpunk",
		PreferredAI:         "auto",
	}

	if err := d.DB.Create(&admin).Error; err != nil {
		return err
	}

	log.Println("✅ Admin user created successfully")
	log.Println("   Username: admin")
	log.Println("   Privileges: Full unlimited access")

	return nil
}

// SeedSpencerUser creates Spencer's user account if it doesn't exist
func (d *Database) SeedSpencerUser() error {
	// Check if spencer already exists
	var existingUser models.User
	result := d.DB.Where("username = ?", "spencer").First(&existingUser)

	if result.Error == nil {
		log.Println("✅ Spencer user already exists")
		// Update to owner privileges
		d.DB.Model(&existingUser).Updates(map[string]interface{}{
			"is_admin":             true,
			"is_super_admin":       true,
			"has_unlimited_credits": true,
			"bypass_billing":       true,
			"bypass_rate_limits":   true,
			"subscription_type":   "owner",
			"is_verified":         true,
			"is_active":           true,
		})
		return nil
	}

	// SECURITY: Get password from environment variable
	password := getSeedPassword("SPENCER_SEED_PASSWORD", "spencer-dev-password")
	if password == "" {
		log.Println("⚠️  Skipping spencer user creation - SPENCER_SEED_PASSWORD not set")
		return nil
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Create Spencer's user
	spencer := models.User{
		Username:            "spencer",
		Email:               "spencerandtheteagues@gmail.com",
		PasswordHash:        string(hashedPassword),
		FullName:            "Spencer Teague",
		IsActive:            true,
		IsVerified:          true,
		IsAdmin:             true,
		IsSuperAdmin:        true,
		HasUnlimitedCredits: true,
		BypassBilling:       true,
		BypassRateLimits:    true,
		SubscriptionType:    "owner",
		SubscriptionEnd:     time.Now().AddDate(100, 0, 0),
		CreditBalance:       999999999.0,
		PreferredTheme:      "cyberpunk",
		PreferredAI:         "auto",
	}

	if err := d.DB.Create(&spencer).Error; err != nil {
		return err
	}

	log.Println("✅ Spencer user created successfully")
	return nil
}

// RunSeeds runs all database seeds
func (d *Database) RunSeeds() error {
	log.Println("🌱 Running database seeds...")

	if err := d.SeedAdminUser(); err != nil {
		log.Printf("⚠️ Failed to seed admin user: %v", err)
	}

	if err := d.SeedSpencerUser(); err != nil {
		log.Printf("⚠️ Failed to seed spencer user: %v", err)
	}

	log.Println("🌱 Database seeding complete")
	return nil
}
