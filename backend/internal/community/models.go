// APEX.BUILD Community Models
// Models for project sharing and discovery

package community

import (
	"time"

	"apex-build/pkg/models"
	"gorm.io/gorm"
)

// ProjectStar represents a star/like on a project
type ProjectStar struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	UserID    uint           `json:"user_id" gorm:"uniqueIndex:idx_star_user_project;not null"`
	User      models.User    `json:"user" gorm:"foreignKey:UserID"`
	ProjectID uint           `json:"project_id" gorm:"uniqueIndex:idx_star_user_project;not null"`
	Project   models.Project `json:"project" gorm:"foreignKey:ProjectID"`
	CreatedAt time.Time      `json:"created_at"`
}

// ProjectFork represents a fork of a project
type ProjectFork struct {
	ID         uint           `json:"id" gorm:"primarykey"`
	OriginalID uint           `json:"original_id" gorm:"index;not null"`
	Original   models.Project `json:"original" gorm:"foreignKey:OriginalID"`
	ForkedID   uint           `json:"forked_id" gorm:"uniqueIndex;not null"`
	Forked     models.Project `json:"forked" gorm:"foreignKey:ForkedID"`
	UserID     uint           `json:"user_id" gorm:"index;not null"`
	User       models.User    `json:"user" gorm:"foreignKey:UserID"`
	CreatedAt  time.Time      `json:"created_at"`
}

// ProjectComment represents a comment on a project
type ProjectComment struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	ProjectID uint           `json:"project_id" gorm:"index;not null"`
	Project   models.Project `json:"project" gorm:"foreignKey:ProjectID"`
	UserID    uint           `json:"user_id" gorm:"index;not null"`
	User      models.User    `json:"user" gorm:"foreignKey:UserID"`
	ParentID  *uint          `json:"parent_id" gorm:"index"` // For threaded comments
	Content   string         `json:"content" gorm:"type:text;not null"`
	IsEdited  bool           `json:"is_edited" gorm:"default:false"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Replies
	Replies []ProjectComment `json:"replies,omitempty" gorm:"foreignKey:ParentID"`
}

// ProjectView tracks project views
type ProjectView struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	ProjectID uint           `json:"project_id" gorm:"index;not null"`
	Project   models.Project `json:"project" gorm:"foreignKey:ProjectID"`
	UserID    *uint          `json:"user_id" gorm:"index"` // Nullable for anonymous views
	User      *models.User   `json:"user,omitempty" gorm:"foreignKey:UserID"`
	IPHash    string         `json:"ip_hash" gorm:"index"` // Hashed IP for anonymous tracking
	ViewedAt  time.Time      `json:"viewed_at"`
}

// UserFollow represents a follow relationship between users
type UserFollow struct {
	ID          uint        `json:"id" gorm:"primarykey"`
	FollowerID  uint        `json:"follower_id" gorm:"uniqueIndex:idx_follow_pair;not null"`
	Follower    models.User `json:"follower" gorm:"foreignKey:FollowerID"`
	FollowingID uint        `json:"following_id" gorm:"uniqueIndex:idx_follow_pair;not null"`
	Following   models.User `json:"following" gorm:"foreignKey:FollowingID"`
	CreatedAt   time.Time   `json:"created_at"`
}

// ProjectCategory represents a category for projects
type ProjectCategory struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	Name        string    `json:"name" gorm:"uniqueIndex;not null"`
	Slug        string    `json:"slug" gorm:"uniqueIndex;not null"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"` // Lucide icon name
	Color       string    `json:"color"` // Hex color
	SortOrder   int       `json:"sort_order" gorm:"default:0"`
	CreatedAt   time.Time `json:"created_at"`
}

// ProjectCategoryAssignment links projects to categories
type ProjectCategoryAssignment struct {
	ProjectID  uint            `json:"project_id" gorm:"primaryKey"`
	CategoryID uint            `json:"category_id" gorm:"primaryKey"`
	Category   ProjectCategory `json:"category" gorm:"foreignKey:CategoryID"`
	CreatedAt  time.Time       `json:"created_at"`
}

// FeaturedProject represents a staff-picked/featured project
type FeaturedProject struct {
	ID          uint           `json:"id" gorm:"primarykey"`
	ProjectID   uint           `json:"project_id" gorm:"uniqueIndex;not null"`
	Project     models.Project `json:"project" gorm:"foreignKey:ProjectID"`
	FeaturedBy  uint           `json:"featured_by" gorm:"not null"`
	FeaturedAt  time.Time      `json:"featured_at"`
	ExpiresAt   *time.Time     `json:"expires_at"` // Optional expiration
	Title       string         `json:"title"`      // Custom featured title
	Description string         `json:"description"` // Custom featured description
}

// ProjectStats holds aggregated stats for a project (for quick access)
type ProjectStats struct {
	ProjectID  uint      `json:"project_id" gorm:"primaryKey"`
	StarCount  int       `json:"star_count" gorm:"default:0"`
	ForkCount  int       `json:"fork_count" gorm:"default:0"`
	ViewCount  int       `json:"view_count" gorm:"default:0"`
	CommentCount int     `json:"comment_count" gorm:"default:0"`
	TrendScore float64   `json:"trend_score" gorm:"default:0"` // Calculated trending score
	UpdatedAt  time.Time `json:"updated_at"`
}

// UserStats holds aggregated stats for a user
type UserStats struct {
	UserID         uint      `json:"user_id" gorm:"primaryKey"`
	FollowerCount  int       `json:"follower_count" gorm:"default:0"`
	FollowingCount int       `json:"following_count" gorm:"default:0"`
	ProjectCount   int       `json:"project_count" gorm:"default:0"`
	TotalStars     int       `json:"total_stars" gorm:"default:0"` // Stars received on all projects
	TotalForks     int       `json:"total_forks" gorm:"default:0"` // Forks of all projects
	UpdatedAt      time.Time `json:"updated_at"`
}

// ProjectWithStats is a helper struct for API responses
type ProjectWithStats struct {
	models.Project
	Stats        *ProjectStats `json:"stats,omitempty"`
	IsStarred    bool          `json:"is_starred,omitempty"`
	IsFork       bool          `json:"is_fork,omitempty"`
	OriginalID   *uint         `json:"original_id,omitempty"`
	Categories   []string      `json:"categories,omitempty"`
}

// UserPublicProfile is a helper struct for public user profiles
type UserPublicProfile struct {
	ID            uint   `json:"id"`
	Username      string `json:"username"`
	FullName      string `json:"full_name"`
	AvatarURL     string `json:"avatar_url"`
	Bio           string `json:"bio"`
	Website       string `json:"website"`
	Location      string `json:"location"`
	JoinedAt      string `json:"joined_at"`
	*UserStats
	IsFollowing   bool   `json:"is_following,omitempty"`
}

// Ensure tables are created
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&ProjectStar{},
		&ProjectFork{},
		&ProjectComment{},
		&ProjectView{},
		&UserFollow{},
		&ProjectCategory{},
		&ProjectCategoryAssignment{},
		&FeaturedProject{},
		&ProjectStats{},
		&UserStats{},
	)
}

// SeedCategories creates default project categories
func SeedCategories(db *gorm.DB) error {
	categories := []ProjectCategory{
		{Name: "Games", Slug: "games", Description: "Interactive games and simulations", Icon: "Gamepad2", Color: "#ef4444", SortOrder: 1},
		{Name: "Tools", Slug: "tools", Description: "Useful utilities and productivity tools", Icon: "Wrench", Color: "#3b82f6", SortOrder: 2},
		{Name: "Websites", Slug: "websites", Description: "Web applications and portfolios", Icon: "Globe", Color: "#10b981", SortOrder: 3},
		{Name: "APIs", Slug: "apis", Description: "Backend services and REST APIs", Icon: "Server", Color: "#8b5cf6", SortOrder: 4},
		{Name: "AI/ML", Slug: "ai-ml", Description: "Machine learning and AI projects", Icon: "Brain", Color: "#f59e0b", SortOrder: 5},
		{Name: "Data", Slug: "data", Description: "Data analysis and visualization", Icon: "BarChart3", Color: "#06b6d4", SortOrder: 6},
		{Name: "Mobile", Slug: "mobile", Description: "Mobile app projects", Icon: "Smartphone", Color: "#ec4899", SortOrder: 7},
		{Name: "CLI", Slug: "cli", Description: "Command-line tools and scripts", Icon: "Terminal", Color: "#64748b", SortOrder: 8},
		{Name: "Learning", Slug: "learning", Description: "Educational projects and tutorials", Icon: "GraduationCap", Color: "#22c55e", SortOrder: 9},
		{Name: "Templates", Slug: "templates", Description: "Starter templates and boilerplates", Icon: "Layout", Color: "#a855f7", SortOrder: 10},
	}

	for _, cat := range categories {
		// Only create if doesn't exist
		var existing ProjectCategory
		if err := db.Where("slug = ?", cat.Slug).First(&existing).Error; err != nil {
			db.Create(&cat)
		}
	}

	return nil
}
