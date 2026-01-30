// APEX.BUILD RBAC Service
// Role-Based Access Control for enterprise organizations

package enterprise

import (
	"fmt"

	"gorm.io/gorm"
)

// RBACService handles role-based access control
type RBACService struct {
	db *gorm.DB
}

// NewRBACService creates a new RBAC service
func NewRBACService(db *gorm.DB) *RBACService {
	// Auto-migrate role and permission tables
	db.AutoMigrate(&Role{}, &Permission{}, &ProjectPermission{}, &OrganizationMember{})
	return &RBACService{db: db}
}

// GetOrganization retrieves an organization by ID
func (s *RBACService) GetOrganization(orgID uint) (*Organization, error) {
	var org Organization
	if err := s.db.First(&org, orgID).Error; err != nil {
		return nil, fmt.Errorf("organization not found: %w", err)
	}
	return &org, nil
}

// GetOrganizationBySlug retrieves an organization by slug
func (s *RBACService) GetOrganizationBySlug(slug string) (*Organization, error) {
	var org Organization
	if err := s.db.Where("slug = ?", slug).First(&org).Error; err != nil {
		return nil, fmt.Errorf("organization not found: %w", err)
	}
	return &org, nil
}

// GetUserOrganizations retrieves organizations a user belongs to
func (s *RBACService) GetUserOrganizations(userID uint) ([]Organization, error) {
	var members []OrganizationMember
	if err := s.db.Where("user_id = ? AND status = ?", userID, "active").
		Preload("Organization").
		Find(&members).Error; err != nil {
		return nil, fmt.Errorf("failed to get user organizations: %w", err)
	}

	orgs := make([]Organization, len(members))
	for i, m := range members {
		orgs[i] = m.Organization
	}
	return orgs, nil
}

// HasPermission checks if a user has a specific permission in an organization
func (s *RBACService) HasPermission(orgID, userID uint, resource, action string) bool {
	var member OrganizationMember
	if err := s.db.Where("organization_id = ? AND user_id = ? AND status = ?", orgID, userID, "active").
		Preload("Role.Permissions").
		First(&member).Error; err != nil {
		return false
	}

	for _, perm := range member.Role.Permissions {
		if perm.Resource == resource && perm.Action == action {
			return true
		}
		if perm.Resource == resource && perm.Action == "manage" {
			return true
		}
	}

	return false
}

// GetRoles returns all roles for an organization
func (s *RBACService) GetRoles(orgID uint) ([]Role, error) {
	var roles []Role
	if err := s.db.Where("organization_id = ? OR is_system = ?", orgID, true).
		Preload("Permissions").
		Find(&roles).Error; err != nil {
		return nil, fmt.Errorf("failed to get roles: %w", err)
	}
	return roles, nil
}
