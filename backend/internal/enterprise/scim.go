// APEX.BUILD SCIM Service
// SCIM 2.0 for user provisioning and deprovisioning

package enterprise

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SCIMService handles SCIM 2.0 provisioning
type SCIMService struct {
	db           *gorm.DB
	auditService *AuditService
	rbacService  *RBACService
}

// NewSCIMService creates a new SCIM service
func NewSCIMService(db *gorm.DB, auditService *AuditService, rbacService *RBACService) *SCIMService {
	return &SCIMService{
		db:           db,
		auditService: auditService,
		rbacService:  rbacService,
	}
}

// SCIM Resource Types and Schemas
const (
	SCIMSchemaUser       = "urn:ietf:params:scim:schemas:core:2.0:User"
	SCIMSchemaGroup      = "urn:ietf:params:scim:schemas:core:2.0:Group"
	SCIMSchemaListResp   = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	SCIMSchemaError      = "urn:ietf:params:scim:api:messages:2.0:Error"
	SCIMSchemaPatchOp    = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	SCIMSchemaEnterprise = "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"
)

// SCIMUser represents a SCIM User resource
type SCIMUser struct {
	Schemas           []string                 `json:"schemas"`
	ID                string                   `json:"id"`
	ExternalID        string                   `json:"externalId,omitempty"`
	UserName          string                   `json:"userName"`
	Name              *SCIMName                `json:"name,omitempty"`
	DisplayName       string                   `json:"displayName,omitempty"`
	Emails            []SCIMEmail              `json:"emails,omitempty"`
	Active            bool                     `json:"active"`
	Groups            []SCIMGroupRef           `json:"groups,omitempty"`
	Meta              *SCIMMeta                `json:"meta,omitempty"`
	EnterpriseUser    *SCIMEnterpriseUser      `json:"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User,omitempty"`
}

// SCIMName represents a SCIM Name
type SCIMName struct {
	Formatted       string `json:"formatted,omitempty"`
	FamilyName      string `json:"familyName,omitempty"`
	GivenName       string `json:"givenName,omitempty"`
	MiddleName      string `json:"middleName,omitempty"`
	HonorificPrefix string `json:"honorificPrefix,omitempty"`
	HonorificSuffix string `json:"honorificSuffix,omitempty"`
}

// SCIMEmail represents a SCIM Email
type SCIMEmail struct {
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Primary bool   `json:"primary,omitempty"`
}

// SCIMGroupRef represents a reference to a SCIM Group
type SCIMGroupRef struct {
	Value   string `json:"value"`
	Ref     string `json:"$ref,omitempty"`
	Display string `json:"display,omitempty"`
}

// SCIMMeta represents SCIM Meta
type SCIMMeta struct {
	ResourceType string `json:"resourceType"`
	Created      string `json:"created"`
	LastModified string `json:"lastModified"`
	Location     string `json:"location,omitempty"`
	Version      string `json:"version,omitempty"`
}

// SCIMEnterpriseUser represents enterprise user extension
type SCIMEnterpriseUser struct {
	EmployeeNumber string      `json:"employeeNumber,omitempty"`
	CostCenter     string      `json:"costCenter,omitempty"`
	Organization   string      `json:"organization,omitempty"`
	Division       string      `json:"division,omitempty"`
	Department     string      `json:"department,omitempty"`
	Manager        *SCIMManager `json:"manager,omitempty"`
}

// SCIMManager represents a manager reference
type SCIMManager struct {
	Value       string `json:"value,omitempty"`
	Ref         string `json:"$ref,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

// SCIMGroup represents a SCIM Group resource
type SCIMGroup struct {
	Schemas     []string      `json:"schemas"`
	ID          string        `json:"id"`
	ExternalID  string        `json:"externalId,omitempty"`
	DisplayName string        `json:"displayName"`
	Members     []SCIMMember  `json:"members,omitempty"`
	Meta        *SCIMMeta     `json:"meta,omitempty"`
}

// SCIMMember represents a SCIM Group member
type SCIMMember struct {
	Value   string `json:"value"`
	Ref     string `json:"$ref,omitempty"`
	Display string `json:"display,omitempty"`
	Type    string `json:"type,omitempty"`
}

// SCIMListResponse represents a SCIM list response
type SCIMListResponse struct {
	Schemas      []string      `json:"schemas"`
	TotalResults int           `json:"totalResults"`
	StartIndex   int           `json:"startIndex"`
	ItemsPerPage int           `json:"itemsPerPage"`
	Resources    []interface{} `json:"Resources"`
}

// SCIMError represents a SCIM error response
type SCIMError struct {
	Schemas  []string `json:"schemas"`
	Status   string   `json:"status"`
	ScimType string   `json:"scimType,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

// SCIMPatchOp represents a SCIM PATCH operation
type SCIMPatchOp struct {
	Schemas    []string           `json:"schemas"`
	Operations []SCIMPatchOpItem  `json:"Operations"`
}

// SCIMPatchOpItem represents a single PATCH operation
type SCIMPatchOpItem struct {
	Op    string      `json:"op"`
	Path  string      `json:"path,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

// GenerateSCIMToken generates a new SCIM token for an organization
func (s *SCIMService) GenerateSCIMToken(org *Organization) (string, error) {
	// Generate random token
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	token := hex.EncodeToString(bytes)

	// Hash for storage
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash token: %w", err)
	}

	org.SCIMEnabled = true
	org.SCIMTokenHash = string(hash)

	if err := s.db.Save(org).Error; err != nil {
		return "", fmt.Errorf("failed to save SCIM token: %w", err)
	}

	// Log the action
	if s.auditService != nil {
		s.auditService.LogEvent(&AuditLog{
			OrganizationID: &org.ID,
			Action:         "scim_token_generated",
			ResourceType:   "organization",
			ResourceID:     fmt.Sprintf("%d", org.ID),
			ResourceName:   org.Name,
			Category:       "security",
			Description:    "SCIM provisioning token generated",
		})
	}

	return token, nil
}

// ValidateSCIMToken validates a SCIM bearer token
func (s *SCIMService) ValidateSCIMToken(org *Organization, token string) bool {
	if !org.SCIMEnabled || org.SCIMTokenHash == "" {
		return false
	}

	err := bcrypt.CompareHashAndPassword([]byte(org.SCIMTokenHash), []byte(token))
	return err == nil
}

// GetUsers retrieves users for SCIM
func (s *SCIMService) GetUsers(org *Organization, filter string, startIndex, count int) (*SCIMListResponse, error) {
	var members []OrganizationMember
	query := s.db.Where("organization_id = ?", org.ID).
		Preload("Role")

	// Apply filter if provided
	if filter != "" {
		// Parse SCIM filter - simplified implementation
		if strings.Contains(filter, "userName eq") {
			// Extract username from filter
			parts := strings.Split(filter, `"`)
			if len(parts) >= 2 {
				username := parts[1]
				query = query.Joins("JOIN users ON users.id = organization_members.user_id").
					Where("users.username = ?", username)
			}
		}
	}

	// Get total count
	var total int64
	query.Model(&OrganizationMember{}).Count(&total)

	// Apply pagination
	if startIndex > 0 {
		query = query.Offset(startIndex - 1) // SCIM is 1-indexed
	}
	if count > 0 {
		query = query.Limit(count)
	}

	if err := query.Find(&members).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch users: %w", err)
	}

	// Convert to SCIM users
	var resources []interface{}
	for _, member := range members {
		var user models.User
		if err := s.db.First(&user, member.UserID).Error; err != nil {
			continue
		}

		scimUser := s.userToSCIM(&user, &member, org)
		resources = append(resources, scimUser)
	}

	return &SCIMListResponse{
		Schemas:      []string{SCIMSchemaListResp},
		TotalResults: int(total),
		StartIndex:   startIndex,
		ItemsPerPage: len(resources),
		Resources:    resources,
	}, nil
}

// GetUser retrieves a single user by ID
func (s *SCIMService) GetUser(org *Organization, userID string) (*SCIMUser, error) {
	id, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID")
	}

	var member OrganizationMember
	if err := s.db.Where("organization_id = ? AND user_id = ?", org.ID, id).
		Preload("Role").
		First(&member).Error; err != nil {
		return nil, fmt.Errorf("user not found")
	}

	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, fmt.Errorf("user not found")
	}

	return s.userToSCIM(&user, &member, org), nil
}

// CreateUser creates a new user via SCIM
func (s *SCIMService) CreateUser(org *Organization, scimUser *SCIMUser) (*SCIMUser, error) {
	// Check if user already exists
	var existingUser models.User
	if err := s.db.Where("email = ?", s.getPrimaryEmail(scimUser)).First(&existingUser).Error; err == nil {
		// User exists, just add to organization
		return s.addUserToOrg(&existingUser, org, scimUser)
	}

	// Create new user
	user := &models.User{
		Username:     scimUser.UserName,
		Email:        s.getPrimaryEmail(scimUser),
		PasswordHash: "", // SSO users don't have passwords
		FullName:     s.getDisplayName(scimUser),
		IsActive:     scimUser.Active,
		IsVerified:   true, // SCIM-provisioned users are verified
		SubscriptionType: "team",
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return s.addUserToOrg(user, org, scimUser)
}

// addUserToOrg adds a user to an organization
func (s *SCIMService) addUserToOrg(user *models.User, org *Organization, scimUser *SCIMUser) (*SCIMUser, error) {
	// Get default role
	var defaultRole Role
	if err := s.db.Where("organization_id = ? AND is_default = ?", org.ID, true).First(&defaultRole).Error; err != nil {
		// Fall back to "Developer" role or first role
		if err := s.db.Where("organization_id = ?", org.ID).First(&defaultRole).Error; err != nil {
			return nil, fmt.Errorf("no default role found")
		}
	}

	// Create organization member
	now := time.Now()
	member := &OrganizationMember{
		OrganizationID: org.ID,
		UserID:         user.ID,
		RoleID:         defaultRole.ID,
		ProvisionedBy:  "scim",
		Status:         "active",
		JoinedAt:       &now,
		SAMLNameID:     scimUser.ExternalID,
	}

	if !scimUser.Active {
		member.Status = "suspended"
	}

	// Store enterprise attributes
	if scimUser.EnterpriseUser != nil {
		member.SAMLAttributes = map[string]interface{}{
			"employeeNumber": scimUser.EnterpriseUser.EmployeeNumber,
			"department":     scimUser.EnterpriseUser.Department,
			"organization":   scimUser.EnterpriseUser.Organization,
		}
	}

	if err := s.db.Create(member).Error; err != nil {
		return nil, fmt.Errorf("failed to add user to organization: %w", err)
	}

	// Log the action
	if s.auditService != nil {
		s.auditService.LogEvent(&AuditLog{
			OrganizationID: &org.ID,
			UserID:         &user.ID,
			Username:       user.Username,
			Email:          user.Email,
			Action:         "user_provisioned",
			ResourceType:   "user",
			ResourceID:     fmt.Sprintf("%d", user.ID),
			ResourceName:   user.Username,
			Category:       "provisioning",
			Description:    "User provisioned via SCIM",
			Metadata: map[string]interface{}{
				"external_id": scimUser.ExternalID,
				"role":        defaultRole.Name,
			},
		})
	}

	return s.userToSCIM(user, member, org), nil
}

// UpdateUser updates a user via SCIM
func (s *SCIMService) UpdateUser(org *Organization, userID string, scimUser *SCIMUser) (*SCIMUser, error) {
	id, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID")
	}

	var member OrganizationMember
	if err := s.db.Where("organization_id = ? AND user_id = ?", org.ID, id).First(&member).Error; err != nil {
		return nil, fmt.Errorf("user not found in organization")
	}

	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, fmt.Errorf("user not found")
	}

	// Update user fields
	user.Username = scimUser.UserName
	user.Email = s.getPrimaryEmail(scimUser)
	user.FullName = s.getDisplayName(scimUser)
	user.IsActive = scimUser.Active

	if err := s.db.Save(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Update member status
	if scimUser.Active {
		member.Status = "active"
	} else {
		member.Status = "suspended"
	}

	if err := s.db.Save(&member).Error; err != nil {
		return nil, fmt.Errorf("failed to update member: %w", err)
	}

	// Log the action
	if s.auditService != nil {
		s.auditService.LogEvent(&AuditLog{
			OrganizationID: &org.ID,
			UserID:         &user.ID,
			Username:       user.Username,
			Email:          user.Email,
			Action:         "user_updated",
			ResourceType:   "user",
			ResourceID:     fmt.Sprintf("%d", user.ID),
			ResourceName:   user.Username,
			Category:       "provisioning",
			Description:    "User updated via SCIM",
		})
	}

	return s.userToSCIM(&user, &member, org), nil
}

// PatchUser partially updates a user via SCIM PATCH
func (s *SCIMService) PatchUser(org *Organization, userID string, patchOp *SCIMPatchOp) (*SCIMUser, error) {
	id, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID")
	}

	var member OrganizationMember
	if err := s.db.Where("organization_id = ? AND user_id = ?", org.ID, id).First(&member).Error; err != nil {
		return nil, fmt.Errorf("user not found in organization")
	}

	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, fmt.Errorf("user not found")
	}

	// Apply patch operations
	for _, op := range patchOp.Operations {
		switch strings.ToLower(op.Op) {
		case "replace":
			if err := s.applyPatchReplace(&user, &member, op.Path, op.Value); err != nil {
				return nil, err
			}
		case "add":
			if err := s.applyPatchAdd(&user, &member, op.Path, op.Value); err != nil {
				return nil, err
			}
		case "remove":
			if err := s.applyPatchRemove(&user, &member, op.Path); err != nil {
				return nil, err
			}
		}
	}

	if err := s.db.Save(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to save user: %w", err)
	}

	if err := s.db.Save(&member).Error; err != nil {
		return nil, fmt.Errorf("failed to save member: %w", err)
	}

	// Log the action
	if s.auditService != nil {
		s.auditService.LogEvent(&AuditLog{
			OrganizationID: &org.ID,
			UserID:         &user.ID,
			Username:       user.Username,
			Email:          user.Email,
			Action:         "user_patched",
			ResourceType:   "user",
			ResourceID:     fmt.Sprintf("%d", user.ID),
			ResourceName:   user.Username,
			Category:       "provisioning",
			Description:    "User patched via SCIM",
		})
	}

	return s.userToSCIM(&user, &member, org), nil
}

// DeleteUser removes a user from the organization via SCIM
func (s *SCIMService) DeleteUser(org *Organization, userID string) error {
	id, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid user ID")
	}

	var member OrganizationMember
	if err := s.db.Where("organization_id = ? AND user_id = ?", org.ID, id).First(&member).Error; err != nil {
		return fmt.Errorf("user not found in organization")
	}

	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return fmt.Errorf("user not found")
	}

	// Remove from organization (soft delete)
	if err := s.db.Delete(&member).Error; err != nil {
		return fmt.Errorf("failed to remove user from organization: %w", err)
	}

	// Log the action
	if s.auditService != nil {
		s.auditService.LogEvent(&AuditLog{
			OrganizationID: &org.ID,
			UserID:         &user.ID,
			Username:       user.Username,
			Email:          user.Email,
			Action:         "user_deprovisioned",
			ResourceType:   "user",
			ResourceID:     fmt.Sprintf("%d", user.ID),
			ResourceName:   user.Username,
			Category:       "provisioning",
			Description:    "User deprovisioned via SCIM",
		})
	}

	return nil
}

// Helper functions

func (s *SCIMService) userToSCIM(user *models.User, member *OrganizationMember, org *Organization) *SCIMUser {
	scimUser := &SCIMUser{
		Schemas:     []string{SCIMSchemaUser},
		ID:          fmt.Sprintf("%d", user.ID),
		ExternalID:  member.SAMLNameID,
		UserName:    user.Username,
		DisplayName: user.FullName,
		Active:      user.IsActive && member.Status == "active",
		Meta: &SCIMMeta{
			ResourceType: "User",
			Created:      user.CreatedAt.Format(time.RFC3339),
			LastModified: user.UpdatedAt.Format(time.RFC3339),
		},
	}

	// Parse name
	names := strings.SplitN(user.FullName, " ", 2)
	if len(names) > 0 {
		scimUser.Name = &SCIMName{
			Formatted:  user.FullName,
			GivenName:  names[0],
		}
		if len(names) > 1 {
			scimUser.Name.FamilyName = names[1]
		}
	}

	// Add email
	scimUser.Emails = []SCIMEmail{
		{
			Value:   user.Email,
			Type:    "work",
			Primary: true,
		},
	}

	// Add group membership
	if member.Role.ID != 0 {
		scimUser.Groups = []SCIMGroupRef{
			{
				Value:   fmt.Sprintf("%d", member.RoleID),
				Display: member.Role.Name,
			},
		}
	}

	return scimUser
}

func (s *SCIMService) getPrimaryEmail(scimUser *SCIMUser) string {
	for _, email := range scimUser.Emails {
		if email.Primary || len(scimUser.Emails) == 1 {
			return email.Value
		}
	}
	if len(scimUser.Emails) > 0 {
		return scimUser.Emails[0].Value
	}
	return scimUser.UserName // Fall back to username
}

func (s *SCIMService) getDisplayName(scimUser *SCIMUser) string {
	if scimUser.DisplayName != "" {
		return scimUser.DisplayName
	}
	if scimUser.Name != nil {
		if scimUser.Name.Formatted != "" {
			return scimUser.Name.Formatted
		}
		return strings.TrimSpace(scimUser.Name.GivenName + " " + scimUser.Name.FamilyName)
	}
	return scimUser.UserName
}

func (s *SCIMService) applyPatchReplace(user *models.User, member *OrganizationMember, path string, value interface{}) error {
	switch strings.ToLower(path) {
	case "active":
		if active, ok := value.(bool); ok {
			user.IsActive = active
			if active {
				member.Status = "active"
			} else {
				member.Status = "suspended"
			}
		}
	case "username":
		if username, ok := value.(string); ok {
			user.Username = username
		}
	case "displayname":
		if displayName, ok := value.(string); ok {
			user.FullName = displayName
		}
	case "name.givenname", "name.familyname":
		// Handle name components
	}
	return nil
}

func (s *SCIMService) applyPatchAdd(user *models.User, member *OrganizationMember, path string, value interface{}) error {
	// Similar to replace for most cases
	return s.applyPatchReplace(user, member, path, value)
}

func (s *SCIMService) applyPatchRemove(user *models.User, member *OrganizationMember, path string) error {
	// Handle removal of optional attributes
	return nil
}

// RegisterSCIMRoutes registers SCIM endpoints
func (s *SCIMService) RegisterSCIMRoutes(router *gin.RouterGroup) {
	scim := router.Group("/scim/v2")
	scim.Use(s.SCIMAuthMiddleware())
	{
		// Users
		scim.GET("/Users", s.handleListUsers)
		scim.GET("/Users/:id", s.handleGetUser)
		scim.POST("/Users", s.handleCreateUser)
		scim.PUT("/Users/:id", s.handleReplaceUser)
		scim.PATCH("/Users/:id", s.handlePatchUser)
		scim.DELETE("/Users/:id", s.handleDeleteUser)

		// Groups (if needed)
		scim.GET("/Groups", s.handleListGroups)
		scim.GET("/Groups/:id", s.handleGetGroup)

		// Service Provider Config
		scim.GET("/ServiceProviderConfig", s.handleServiceProviderConfig)
		scim.GET("/Schemas", s.handleSchemas)
		scim.GET("/ResourceTypes", s.handleResourceTypes)
	}
}

// SCIMAuthMiddleware validates SCIM bearer tokens
func (s *SCIMService) SCIMAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			s.scimError(c, http.StatusUnauthorized, "invalidAuth", "Missing or invalid authorization header")
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		// Get organization from URL or header
		orgSlug := c.GetHeader("X-Organization")
		if orgSlug == "" {
			s.scimError(c, http.StatusBadRequest, "invalidOrg", "X-Organization header required")
			return
		}

		var org Organization
		if err := s.db.Where("slug = ?", orgSlug).First(&org).Error; err != nil {
			s.scimError(c, http.StatusNotFound, "notFound", "Organization not found")
			return
		}

		if !s.ValidateSCIMToken(&org, token) {
			s.scimError(c, http.StatusUnauthorized, "invalidAuth", "Invalid SCIM token")
			return
		}

		c.Set("organization", &org)
		c.Next()
	}
}

func (s *SCIMService) scimError(c *gin.Context, status int, scimType, detail string) {
	c.JSON(status, SCIMError{
		Schemas:  []string{SCIMSchemaError},
		Status:   fmt.Sprintf("%d", status),
		ScimType: scimType,
		Detail:   detail,
	})
	c.Abort()
}

func (s *SCIMService) handleListUsers(c *gin.Context) {
	org := c.MustGet("organization").(*Organization)

	filter := c.Query("filter")
	startIndex, _ := strconv.Atoi(c.DefaultQuery("startIndex", "1"))
	count, _ := strconv.Atoi(c.DefaultQuery("count", "100"))

	response, err := s.GetUsers(org, filter, startIndex, count)
	if err != nil {
		s.scimError(c, http.StatusInternalServerError, "serverError", err.Error())
		return
	}

	c.JSON(http.StatusOK, response)
}

func (s *SCIMService) handleGetUser(c *gin.Context) {
	org := c.MustGet("organization").(*Organization)
	userID := c.Param("id")

	user, err := s.GetUser(org, userID)
	if err != nil {
		s.scimError(c, http.StatusNotFound, "notFound", err.Error())
		return
	}

	c.JSON(http.StatusOK, user)
}

func (s *SCIMService) handleCreateUser(c *gin.Context) {
	org := c.MustGet("organization").(*Organization)

	var scimUser SCIMUser
	if err := c.ShouldBindJSON(&scimUser); err != nil {
		s.scimError(c, http.StatusBadRequest, "invalidSyntax", err.Error())
		return
	}

	user, err := s.CreateUser(org, &scimUser)
	if err != nil {
		s.scimError(c, http.StatusInternalServerError, "serverError", err.Error())
		return
	}

	c.JSON(http.StatusCreated, user)
}

func (s *SCIMService) handleReplaceUser(c *gin.Context) {
	org := c.MustGet("organization").(*Organization)
	userID := c.Param("id")

	var scimUser SCIMUser
	if err := c.ShouldBindJSON(&scimUser); err != nil {
		s.scimError(c, http.StatusBadRequest, "invalidSyntax", err.Error())
		return
	}

	user, err := s.UpdateUser(org, userID, &scimUser)
	if err != nil {
		s.scimError(c, http.StatusNotFound, "notFound", err.Error())
		return
	}

	c.JSON(http.StatusOK, user)
}

func (s *SCIMService) handlePatchUser(c *gin.Context) {
	org := c.MustGet("organization").(*Organization)
	userID := c.Param("id")

	var patchOp SCIMPatchOp
	if err := c.ShouldBindJSON(&patchOp); err != nil {
		s.scimError(c, http.StatusBadRequest, "invalidSyntax", err.Error())
		return
	}

	user, err := s.PatchUser(org, userID, &patchOp)
	if err != nil {
		s.scimError(c, http.StatusNotFound, "notFound", err.Error())
		return
	}

	c.JSON(http.StatusOK, user)
}

func (s *SCIMService) handleDeleteUser(c *gin.Context) {
	org := c.MustGet("organization").(*Organization)
	userID := c.Param("id")

	if err := s.DeleteUser(org, userID); err != nil {
		s.scimError(c, http.StatusNotFound, "notFound", err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *SCIMService) handleListGroups(c *gin.Context) {
	org := c.MustGet("organization").(*Organization)

	var roles []Role
	s.db.Where("organization_id = ?", org.ID).Find(&roles)

	var resources []interface{}
	for _, role := range roles {
		resources = append(resources, SCIMGroup{
			Schemas:     []string{SCIMSchemaGroup},
			ID:          fmt.Sprintf("%d", role.ID),
			DisplayName: role.Name,
		})
	}

	c.JSON(http.StatusOK, SCIMListResponse{
		Schemas:      []string{SCIMSchemaListResp},
		TotalResults: len(resources),
		StartIndex:   1,
		ItemsPerPage: len(resources),
		Resources:    resources,
	})
}

func (s *SCIMService) handleGetGroup(c *gin.Context) {
	org := c.MustGet("organization").(*Organization)
	groupID := c.Param("id")

	id, _ := strconv.ParseUint(groupID, 10, 32)

	var role Role
	if err := s.db.Where("id = ? AND organization_id = ?", id, org.ID).First(&role).Error; err != nil {
		s.scimError(c, http.StatusNotFound, "notFound", "Group not found")
		return
	}

	c.JSON(http.StatusOK, SCIMGroup{
		Schemas:     []string{SCIMSchemaGroup},
		ID:          fmt.Sprintf("%d", role.ID),
		DisplayName: role.Name,
	})
}

func (s *SCIMService) handleServiceProviderConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		"documentationUri": "https://docs.apex.build/scim",
		"patch": gin.H{"supported": true},
		"bulk": gin.H{"supported": false},
		"filter": gin.H{
			"supported": true,
			"maxResults": 200,
		},
		"changePassword": gin.H{"supported": false},
		"sort": gin.H{"supported": false},
		"etag": gin.H{"supported": false},
		"authenticationSchemes": []gin.H{
			{
				"type": "oauthbearertoken",
				"name": "OAuth Bearer Token",
				"description": "Authentication using OAuth 2.0 Bearer Token",
			},
		},
	})
}

func (s *SCIMService) handleSchemas(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"schemas": []string{SCIMSchemaListResp},
		"totalResults": 2,
		"Resources": []gin.H{
			{
				"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:Schema"},
				"id": SCIMSchemaUser,
				"name": "User",
			},
			{
				"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:Schema"},
				"id": SCIMSchemaGroup,
				"name": "Group",
			},
		},
	})
}

func (s *SCIMService) handleResourceTypes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"schemas": []string{SCIMSchemaListResp},
		"totalResults": 2,
		"Resources": []gin.H{
			{
				"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"},
				"id": "User",
				"name": "User",
				"endpoint": "/scim/v2/Users",
				"schema": SCIMSchemaUser,
			},
			{
				"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"},
				"id": "Group",
				"name": "Group",
				"endpoint": "/scim/v2/Groups",
				"schema": SCIMSchemaGroup,
			},
		},
	})
}

// MarshalJSON implements custom JSON marshaling for SCIMUser
func (u *SCIMUser) MarshalJSON() ([]byte, error) {
	type Alias SCIMUser
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(u),
	})
}
