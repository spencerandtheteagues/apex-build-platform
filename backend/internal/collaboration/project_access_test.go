package collaboration

import (
	"errors"
	"testing"

	"apex-build/pkg/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDatabaseAdapter(t *testing.T) *DatabaseAdapter {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Project{}, &models.File{}))

	return NewDatabaseAdapter(db)
}

func TestDatabaseAdapterResolveProjectAccess(t *testing.T) {
	adapter := newTestDatabaseAdapter(t)

	require.NoError(t, adapter.db.Create(&models.Project{ID: 1, OwnerID: 42, Name: "private-owner", Language: "typescript"}).Error)
	require.NoError(t, adapter.db.Create(&models.Project{ID: 2, OwnerID: 7, Name: "public-view", Language: "typescript", IsPublic: true}).Error)
	require.NoError(t, adapter.db.Create(&models.Project{ID: 3, OwnerID: 7, Name: "private-blocked", Language: "typescript"}).Error)

	ownerAccess, err := adapter.ResolveProjectAccess(42, 1)
	require.NoError(t, err)
	require.Equal(t, PermissionOwner, ownerAccess.Permission)
	require.Equal(t, ProjectRoomID(1), ownerAccess.RoomID)

	viewAccess, err := adapter.ResolveProjectAccess(42, 2)
	require.NoError(t, err)
	require.Equal(t, PermissionViewer, viewAccess.Permission)
	require.True(t, viewAccess.Public)

	_, err = adapter.ResolveProjectAccess(42, 3)
	require.True(t, errors.Is(err, ErrProjectAccessDenied))
}

func TestDatabaseAdapterLoadFileAndRoomParsing(t *testing.T) {
	adapter := newTestDatabaseAdapter(t)

	require.NoError(t, adapter.db.Create(&models.Project{ID: 9, OwnerID: 99, Name: "docs", Language: "go"}).Error)
	require.NoError(t, adapter.db.Create(&models.File{ID: 11, ProjectID: 9, Path: "main.go", Name: "main.go", Type: "file", Content: "package main"}).Error)

	projectID, content, err := adapter.LoadFile(11)
	require.NoError(t, err)
	require.Equal(t, uint(9), projectID)
	require.Equal(t, "package main", content)

	parsedProjectID, err := ProjectIDFromRoomID(ProjectRoomID(9))
	require.NoError(t, err)
	require.Equal(t, uint(9), parsedProjectID)

	_, err = ProjectIDFromRoomID("invalid-room")
	require.Error(t, err)
}
