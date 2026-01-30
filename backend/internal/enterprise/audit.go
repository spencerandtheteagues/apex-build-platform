// APEX.BUILD Audit Service
// Enterprise audit logging for compliance and security

package enterprise

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// AuditService handles enterprise audit logging
type AuditService struct {
	db *gorm.DB
}

// NewAuditService creates a new audit service
func NewAuditService(db *gorm.DB) *AuditService {
	// Auto-migrate audit log table
	db.AutoMigrate(&AuditLog{})
	return &AuditService{db: db}
}

// LogEvent records an audit log event
func (s *AuditService) LogEvent(log *AuditLog) {
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	if log.Outcome == "" {
		log.Outcome = "success"
	}
	if log.Severity == "" {
		log.Severity = "info"
	}
	if log.Environment == "" {
		log.Environment = "production"
	}

	if err := s.db.Create(log).Error; err != nil {
		fmt.Printf("Failed to write audit log: %v\n", err)
	}
}

// GetAuditLogs retrieves audit logs with filtering
func (s *AuditService) GetAuditLogs(orgID uint, action, category string, page, pageSize int) ([]AuditLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	query := s.db.Where("organization_id = ?", orgID)

	if action != "" {
		query = query.Where("action = ?", action)
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}

	var total int64
	query.Model(&AuditLog{}).Count(&total)

	var logs []AuditLog
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to fetch audit logs: %w", err)
	}

	return logs, total, nil
}
