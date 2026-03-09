package collaboration

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"apex-build/pkg/models"

	"gorm.io/gorm"
)

var (
	ErrProjectAccessDenied = errors.New("project access denied")
	ErrProjectNotFound     = errors.New("project not found")
	ErrFileNotFound        = errors.New("file not found")
)

type ProjectAccess struct {
	ProjectID  uint
	RoomID     string
	Permission PermissionLevel
	Public     bool
}

type AccessResolver func(userID, projectID uint) (*ProjectAccess, error)

type FileStore interface {
	LoadFile(fileID uint) (projectID uint, content string, err error)
}

type DatabaseAdapter struct {
	db *gorm.DB
}

func NewDatabaseAdapter(db *gorm.DB) *DatabaseAdapter {
	return &DatabaseAdapter{db: db}
}

func ProjectRoomID(projectID uint) string {
	return fmt.Sprintf("project_%d", projectID)
}

func ProjectIDFromRoomID(roomID string) (uint, error) {
	const prefix = "project_"
	if !strings.HasPrefix(roomID, prefix) {
		return 0, fmt.Errorf("invalid room id %q", roomID)
	}

	projectID, err := strconv.ParseUint(strings.TrimPrefix(roomID, prefix), 10, 32)
	if err != nil || projectID == 0 {
		return 0, fmt.Errorf("invalid room id %q", roomID)
	}

	return uint(projectID), nil
}

func (a *DatabaseAdapter) ResolveProjectAccess(userID, projectID uint) (*ProjectAccess, error) {
	if a == nil || a.db == nil {
		return nil, errors.New("collaboration access not configured")
	}

	var project models.Project
	if err := a.db.Select("id", "owner_id", "is_public").First(&project, projectID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, err
	}

	access := &ProjectAccess{
		ProjectID: project.ID,
		RoomID:    ProjectRoomID(project.ID),
		Public:    project.IsPublic,
	}

	switch {
	case project.OwnerID == userID:
		access.Permission = PermissionOwner
	case project.IsPublic:
		access.Permission = PermissionViewer
	default:
		return nil, ErrProjectAccessDenied
	}

	return access, nil
}

func (a *DatabaseAdapter) LoadFile(fileID uint) (uint, string, error) {
	if a == nil || a.db == nil {
		return 0, "", errors.New("collaboration file store not configured")
	}

	var file models.File
	if err := a.db.Select("id", "project_id", "content").First(&file, fileID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, "", ErrFileNotFound
		}
		return 0, "", err
	}

	return file.ProjectID, file.Content, nil
}
