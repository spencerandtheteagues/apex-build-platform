// Package cache - Project caching service
// Implements Redis caching for project listings with 30s TTL
package cache

import (
	"context"
	"encoding/json"
	"time"
)

// ProjectCache provides caching for project-related operations
type ProjectCache struct {
	cache *RedisCache
	ttl   time.Duration
}

// CachedProject represents a project in the cache
type CachedProject struct {
	ID           uint                   `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Language     string                 `json:"language"`
	Framework    string                 `json:"framework"`
	OwnerID      uint                   `json:"owner_id"`
	IsPublic     bool                   `json:"is_public"`
	IsArchived   bool                   `json:"is_archived"`
	FileCount    int                    `json:"file_count"`
	Environment  map[string]interface{} `json:"environment"`
	BuildConfig  map[string]interface{} `json:"build_config"`
	Dependencies map[string]interface{} `json:"dependencies"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// CachedProjectList represents a paginated list of projects
type CachedProjectList struct {
	Projects   []CachedProject `json:"projects"`
	Total      int64           `json:"total"`
	Page       int             `json:"page"`
	Limit      int             `json:"limit"`
	TotalPages int             `json:"total_pages"`
	CachedAt   time.Time       `json:"cached_at"`
}

// NewProjectCache creates a new project cache instance
func NewProjectCache(cache *RedisCache) *ProjectCache {
	return &ProjectCache{
		cache: cache,
		ttl:   30 * time.Second, // 30s TTL as specified
	}
}

// GetProjectList retrieves a cached project list
func (pc *ProjectCache) GetProjectList(ctx context.Context, userID uint, page, limit int) (*CachedProjectList, error) {
	key := ProjectCacheKey(userID, page, limit)

	var result CachedProjectList
	err := pc.cache.GetJSON(ctx, key, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// SetProjectList caches a project list
func (pc *ProjectCache) SetProjectList(ctx context.Context, userID uint, page, limit int, list *CachedProjectList) error {
	key := ProjectCacheKey(userID, page, limit)
	list.CachedAt = time.Now()
	return pc.cache.SetJSON(ctx, key, list, pc.ttl)
}

// GetProject retrieves a cached project by ID
func (pc *ProjectCache) GetProject(ctx context.Context, projectID uint) (*CachedProject, error) {
	key := ProjectDetailCacheKey(projectID)

	var result CachedProject
	err := pc.cache.GetJSON(ctx, key, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// SetProject caches a single project
func (pc *ProjectCache) SetProject(ctx context.Context, project *CachedProject) error {
	key := ProjectDetailCacheKey(project.ID)
	return pc.cache.SetJSON(ctx, key, project, pc.ttl)
}

// InvalidateUserProjects invalidates all cached projects for a user
func (pc *ProjectCache) InvalidateUserProjects(ctx context.Context, userID uint) error {
	pattern := UserProjectsPattern(userID)
	return pc.cache.DeletePattern(ctx, pattern)
}

// InvalidateProject invalidates a specific project cache
func (pc *ProjectCache) InvalidateProject(ctx context.Context, projectID uint) error {
	key := ProjectDetailCacheKey(projectID)
	return pc.cache.Delete(ctx, key)
}

// GetOrLoadProjectList retrieves from cache or loads from database
func (pc *ProjectCache) GetOrLoadProjectList(
	ctx context.Context,
	userID uint,
	page, limit int,
	loader func() (*CachedProjectList, error),
) (*CachedProjectList, error) {
	// Try cache first
	cached, err := pc.GetProjectList(ctx, userID, page, limit)
	if err == nil {
		return cached, nil
	}

	// Load from database
	list, err := loader()
	if err != nil {
		return nil, err
	}

	// Cache the result (ignore errors)
	pc.SetProjectList(ctx, userID, page, limit, list)

	return list, nil
}

// FileCache provides caching for file listings
type FileCache struct {
	cache *RedisCache
	ttl   time.Duration
}

// CachedFile represents a file in the cache
type CachedFile struct {
	ID        uint      `json:"id"`
	ProjectID uint      `json:"project_id"`
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	MimeType  string    `json:"mime_type"`
	Size      int64     `json:"size"`
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CachedFileList represents a list of files
type CachedFileList struct {
	Files    []CachedFile `json:"files"`
	Total    int          `json:"total"`
	CachedAt time.Time    `json:"cached_at"`
}

// NewFileCache creates a new file cache instance
func NewFileCache(cache *RedisCache) *FileCache {
	return &FileCache{
		cache: cache,
		ttl:   60 * time.Second,
	}
}

// GetFileList retrieves a cached file list for a project
func (fc *FileCache) GetFileList(ctx context.Context, projectID uint) (*CachedFileList, error) {
	key := FileListCacheKey(projectID)

	var result CachedFileList
	err := fc.cache.GetJSON(ctx, key, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// SetFileList caches a file list for a project
func (fc *FileCache) SetFileList(ctx context.Context, projectID uint, list *CachedFileList) error {
	key := FileListCacheKey(projectID)
	list.CachedAt = time.Now()
	return fc.cache.SetJSON(ctx, key, list, fc.ttl)
}

// InvalidateFileList invalidates the file list cache for a project
func (fc *FileCache) InvalidateFileList(ctx context.Context, projectID uint) error {
	key := FileListCacheKey(projectID)
	return fc.cache.Delete(ctx, key)
}

// SessionCache provides caching for user sessions
type SessionCache struct {
	cache *RedisCache
	ttl   time.Duration
}

// CachedSession represents a user session in the cache
type CachedSession struct {
	UserID           uint      `json:"user_id"`
	Username         string    `json:"username"`
	Email            string    `json:"email"`
	IsAdmin          bool      `json:"is_admin"`
	SubscriptionType string    `json:"subscription_type"`
	PreferredTheme   string    `json:"preferred_theme"`
	PreferredAI      string    `json:"preferred_ai"`
	CachedAt         time.Time `json:"cached_at"`
}

// NewSessionCache creates a new session cache instance
func NewSessionCache(cache *RedisCache) *SessionCache {
	return &SessionCache{
		cache: cache,
		ttl:   15 * time.Minute,
	}
}

// GetSession retrieves a cached user session
func (sc *SessionCache) GetSession(ctx context.Context, userID uint) (*CachedSession, error) {
	key := UserSessionCacheKey(userID)

	var result CachedSession
	err := sc.cache.GetJSON(ctx, key, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// SetSession caches a user session
func (sc *SessionCache) SetSession(ctx context.Context, session *CachedSession) error {
	key := UserSessionCacheKey(session.UserID)
	session.CachedAt = time.Now()
	return sc.cache.SetJSON(ctx, key, session, sc.ttl)
}

// InvalidateSession invalidates a user's session cache
func (sc *SessionCache) InvalidateSession(ctx context.Context, userID uint) error {
	key := UserSessionCacheKey(userID)
	return sc.cache.Delete(ctx, key)
}

// BatchCacheOperations provides batch operations for efficiency
type BatchCacheOperations struct {
	cache *RedisCache
}

// NewBatchCacheOperations creates a new batch operations handler
func NewBatchCacheOperations(cache *RedisCache) *BatchCacheOperations {
	return &BatchCacheOperations{cache: cache}
}

// BatchGetProjects retrieves multiple projects from cache
func (bo *BatchCacheOperations) BatchGetProjects(ctx context.Context, projectIDs []uint) (map[uint]*CachedProject, []uint) {
	found := make(map[uint]*CachedProject)
	missing := make([]uint, 0)

	for _, id := range projectIDs {
		key := ProjectDetailCacheKey(id)
		data, err := bo.cache.Get(ctx, key)
		if err != nil {
			missing = append(missing, id)
			continue
		}

		var project CachedProject
		if err := json.Unmarshal(data, &project); err != nil {
			missing = append(missing, id)
			continue
		}

		found[id] = &project
	}

	return found, missing
}

// BatchSetProjects caches multiple projects
func (bo *BatchCacheOperations) BatchSetProjects(ctx context.Context, projects []*CachedProject, ttl time.Duration) error {
	for _, project := range projects {
		key := ProjectDetailCacheKey(project.ID)
		if err := bo.cache.SetJSON(ctx, key, project, ttl); err != nil {
			// Continue on error, log would be helpful here
			continue
		}
	}
	return nil
}
