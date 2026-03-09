package db

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"apex-build/internal/config"
	"apex-build/pkg/models"

	"golang.org/x/crypto/bcrypt"
)

// SECURITY: Get seed passwords from environment variables
func getSeedPassword(envVar, defaultDev string) string {
	password := strings.TrimSpace(os.Getenv(envVar))
	if password != "" {
		return password
	}

	switch {
	case config.IsProductionEnvironment() || config.IsStagingEnvironment():
		log.Printf("WARNING: %s not set in %s - seed user will not be created", envVar, config.GetEnvironment())
		return ""
	case config.GetEnvironment() == config.EnvTest:
		return defaultDev
	case strings.EqualFold(strings.TrimSpace(os.Getenv("ALLOW_DEFAULT_SEED_PASSWORDS")), "true"):
		log.Printf("WARNING: Using built-in development seed password for %s because ALLOW_DEFAULT_SEED_PASSWORDS=true", envVar)
		return defaultDev
	default:
		log.Printf("INFO: %s not set and ALLOW_DEFAULT_SEED_PASSWORDS is false - skipping seed user creation", envVar)
		return ""
	}
}

// SeedAdminUser creates the default admin account if it doesn't exist
func (d *Database) SeedAdminUser() error {
	// Check if admin already exists
	var existingAdmin models.User
	result := d.DB.Where("username = ?", "admin").First(&existingAdmin)

	// SECURITY: Get password from environment variable
	password := getSeedPassword("ADMIN_SEED_PASSWORD", "admin-dev-password")
	if password == "" {
		log.Println("⚠️  Skipping admin user creation - ADMIN_SEED_PASSWORD not set")
		return nil
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	if result.Error == nil {
		log.Println("✅ Admin user already exists - updating privileges and password")
		d.DB.Model(&existingAdmin).Updates(map[string]interface{}{
			"password_hash":         string(hashedPassword),
			"is_admin":              true,
			"is_super_admin":        true,
			"has_unlimited_credits": true,
			"bypass_billing":        true,
			"bypass_rate_limits":    true,
			"subscription_type":     "owner",
			"is_verified":           true,
			"is_active":             true,
		})
		return nil
	}
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

	// SECURITY: Get password from environment variable
	password := getSeedPassword("SPENCER_SEED_PASSWORD", "spencer-dev-password")
	if password == "" {
		log.Println("⚠️  Skipping spencer user creation - SPENCER_SEED_PASSWORD not set")
		return nil
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	if result.Error == nil {
		log.Println("✅ Spencer user already exists - updating privileges and password")
		d.DB.Model(&existingUser).Updates(map[string]interface{}{
			"password_hash":         string(hashedPassword),
			"is_admin":              true,
			"is_super_admin":        true,
			"has_unlimited_credits": true,
			"bypass_billing":        true,
			"bypass_rate_limits":    true,
			"subscription_type":     "owner",
			"is_verified":           true,
			"is_active":             true,
		})
		return nil
	}
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
	var errs []string

	if err := d.SeedAdminUser(); err != nil {
		log.Printf("⚠️ Failed to seed admin user: %v", err)
		errs = append(errs, fmt.Sprintf("admin seed: %v", err))
	}

	if err := d.SeedSpencerUser(); err != nil {
		log.Printf("⚠️ Failed to seed spencer user: %v", err)
		errs = append(errs, fmt.Sprintf("spencer seed: %v", err))
	}

	log.Println("🌱 Database seeding complete")
	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	return nil
}
