// Package git - Git Integration for APEX.BUILD
// Provides version control functionality directly in the browser
package git

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"apex-build/pkg/models"

	"gorm.io/gorm"
)

// GitService provides git operations for projects
type GitService struct {
	db          *gorm.DB
	githubToken string // Server-level GitHub token (optional)
	mu          sync.RWMutex
}

// Repository represents a git repository configuration
type Repository struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	ProjectID   uint      `json:"project_id" gorm:"index"`
	RemoteURL   string    `json:"remote_url"`
	Provider    string    `json:"provider"` // github, gitlab, bitbucket
	RepoOwner   string    `json:"repo_owner"`
	RepoName    string    `json:"repo_name"`
	Branch      string    `json:"branch"`
	LastSync    time.Time `json:"last_sync"`
	IsConnected bool      `json:"is_connected"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Commit represents a git commit
type Commit struct {
	SHA       string    `json:"sha"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	Email     string    `json:"email"`
	Timestamp time.Time `json:"timestamp"`
	Files     []string  `json:"files,omitempty"`
}

// Branch represents a git branch
type Branch struct {
	Name      string    `json:"name"`
	SHA       string    `json:"sha"`
	IsDefault bool      `json:"is_default"`
	Protected bool      `json:"protected"`
	Ahead     int       `json:"ahead"`
	Behind    int       `json:"behind"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FileChange represents a file change in the working tree
type FileChange struct {
	Path      string `json:"path"`
	Status    string `json:"status"` // added, modified, deleted, renamed
	Staged    bool   `json:"staged"`
	OldPath   string `json:"old_path,omitempty"` // For renames
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// DiffResult represents the diff of a file
type DiffResult struct {
	Path      string   `json:"path"`
	OldPath   string   `json:"old_path,omitempty"`
	Additions int      `json:"additions"`
	Deletions int      `json:"deletions"`
	Hunks     []*Hunk  `json:"hunks"`
}

// Hunk represents a change hunk in a diff
type Hunk struct {
	OldStart  int      `json:"old_start"`
	OldLines  int      `json:"old_lines"`
	NewStart  int      `json:"new_start"`
	NewLines  int      `json:"new_lines"`
	Header    string   `json:"header"`
	Lines     []string `json:"lines"`
}

// PullRequest represents a GitHub pull request
type PullRequest struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	Author    string    `json:"author"`
	Branch    string    `json:"branch"`
	BaseBranch string   `json:"base_branch"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	URL       string    `json:"url"`
}

// NewGitService creates a new git service
func NewGitService(db *gorm.DB) *GitService {
	return &GitService{db: db}
}

// ConnectRepository connects a project to a remote git repository
func (g *GitService) ConnectRepository(ctx context.Context, projectID uint, remoteURL, token string) (*Repository, error) {
	// Parse remote URL
	provider, owner, name := g.parseRemoteURL(remoteURL)
	if provider == "" {
		return nil, fmt.Errorf("unsupported git provider")
	}

	// Verify repository access
	if !g.verifyRepositoryAccess(ctx, provider, owner, name, token) {
		return nil, fmt.Errorf("cannot access repository - check URL and permissions")
	}

	// Create or update repository record
	repo := &Repository{
		ProjectID:   projectID,
		RemoteURL:   remoteURL,
		Provider:    provider,
		RepoOwner:   owner,
		RepoName:    name,
		Branch:      "main",
		IsConnected: true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := g.db.WithContext(ctx).Save(repo).Error; err != nil {
		return nil, err
	}

	return repo, nil
}

// GetRepository gets the repository configuration for a project
func (g *GitService) GetRepository(ctx context.Context, projectID uint) (*Repository, error) {
	var repo Repository
	if err := g.db.WithContext(ctx).Where("project_id = ?", projectID).First(&repo).Error; err != nil {
		return nil, err
	}
	return &repo, nil
}

// GetBranches lists all branches for a repository
func (g *GitService) GetBranches(ctx context.Context, projectID uint, token string) ([]*Branch, error) {
	repo, err := g.GetRepository(ctx, projectID)
	if err != nil {
		return nil, err
	}

	switch repo.Provider {
	case "github":
		return g.getGitHubBranches(ctx, repo, token)
	default:
		return nil, fmt.Errorf("provider not supported: %s", repo.Provider)
	}
}

// GetCommits gets commit history
func (g *GitService) GetCommits(ctx context.Context, projectID uint, branch string, limit int, token string) ([]*Commit, error) {
	repo, err := g.GetRepository(ctx, projectID)
	if err != nil {
		return nil, err
	}

	if branch == "" {
		branch = repo.Branch
	}
	if limit <= 0 {
		limit = 20
	}

	switch repo.Provider {
	case "github":
		return g.getGitHubCommits(ctx, repo, branch, limit, token)
	default:
		return nil, fmt.Errorf("provider not supported: %s", repo.Provider)
	}
}

// GetWorkingTreeStatus gets the status of changed files
func (g *GitService) GetWorkingTreeStatus(ctx context.Context, projectID uint) ([]*FileChange, error) {
	// For cloud IDE, we track changes in the database
	// Get project's "base" files (from last commit/sync) and compare with current files

	var changes []*FileChange

	// Get all current files
	var currentFiles []models.File
	if err := g.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&currentFiles).Error; err != nil {
		return nil, err
	}

	// Get base version files (stored in FileVersion or tracked separately)
	// For now, we'll mark all files as "modified" if they have version > 1
	for _, f := range currentFiles {
		if f.Version > 1 {
			changes = append(changes, &FileChange{
				Path:   f.Path,
				Status: "modified",
				Staged: false,
			})
		} else {
			// Check if file was added after project creation
			changes = append(changes, &FileChange{
				Path:   f.Path,
				Status: "added",
				Staged: false,
			})
		}
	}

	return changes, nil
}

// CreateCommit creates a new commit with staged changes
func (g *GitService) CreateCommit(ctx context.Context, projectID uint, message string, files []string, token string) (*Commit, error) {
	repo, err := g.GetRepository(ctx, projectID)
	if err != nil {
		return nil, err
	}

	switch repo.Provider {
	case "github":
		return g.createGitHubCommit(ctx, repo, message, files, token, projectID)
	default:
		return nil, fmt.Errorf("provider not supported: %s", repo.Provider)
	}
}

// Push pushes commits to remote
func (g *GitService) Push(ctx context.Context, projectID uint, token string) error {
	// For GitHub API, commits are pushed immediately when created
	// This is a no-op for API-based git, but would be needed for actual git
	return nil
}

// Pull pulls changes from remote
func (g *GitService) Pull(ctx context.Context, projectID uint, token string) error {
	repo, err := g.GetRepository(ctx, projectID)
	if err != nil {
		return err
	}

	switch repo.Provider {
	case "github":
		return g.pullFromGitHub(ctx, repo, token, projectID)
	default:
		return fmt.Errorf("provider not supported: %s", repo.Provider)
	}
}

// CreateBranch creates a new branch
func (g *GitService) CreateBranch(ctx context.Context, projectID uint, branchName, baseBranch, token string) (*Branch, error) {
	repo, err := g.GetRepository(ctx, projectID)
	if err != nil {
		return nil, err
	}

	switch repo.Provider {
	case "github":
		return g.createGitHubBranch(ctx, repo, branchName, baseBranch, token)
	default:
		return nil, fmt.Errorf("provider not supported: %s", repo.Provider)
	}
}

// SwitchBranch switches to a different branch
func (g *GitService) SwitchBranch(ctx context.Context, projectID uint, branchName, token string) error {
	repo, err := g.GetRepository(ctx, projectID)
	if err != nil {
		return err
	}

	// Update the current branch
	repo.Branch = branchName
	repo.UpdatedAt = time.Now()

	if err := g.db.WithContext(ctx).Save(repo).Error; err != nil {
		return err
	}

	// Pull the new branch content
	return g.Pull(ctx, projectID, token)
}

// GetPullRequests lists pull requests
func (g *GitService) GetPullRequests(ctx context.Context, projectID uint, state, token string) ([]*PullRequest, error) {
	repo, err := g.GetRepository(ctx, projectID)
	if err != nil {
		return nil, err
	}

	switch repo.Provider {
	case "github":
		return g.getGitHubPullRequests(ctx, repo, state, token)
	default:
		return nil, fmt.Errorf("provider not supported: %s", repo.Provider)
	}
}

// CreatePullRequest creates a new pull request
func (g *GitService) CreatePullRequest(ctx context.Context, projectID uint, title, body, head, base, token string) (*PullRequest, error) {
	repo, err := g.GetRepository(ctx, projectID)
	if err != nil {
		return nil, err
	}

	switch repo.Provider {
	case "github":
		return g.createGitHubPullRequest(ctx, repo, title, body, head, base, token)
	default:
		return nil, fmt.Errorf("provider not supported: %s", repo.Provider)
	}
}

// GitHub-specific implementations

func (g *GitService) getGitHubBranches(ctx context.Context, repo *Repository, token string) ([]*Branch, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/branches", repo.RepoOwner, repo.RepoName)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API error: %d", resp.StatusCode)
	}

	var ghBranches []struct {
		Name      string `json:"name"`
		Commit    struct {
			SHA string `json:"sha"`
		} `json:"commit"`
		Protected bool `json:"protected"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghBranches); err != nil {
		return nil, err
	}

	branches := make([]*Branch, len(ghBranches))
	for i, b := range ghBranches {
		branches[i] = &Branch{
			Name:      b.Name,
			SHA:       b.Commit.SHA,
			IsDefault: b.Name == "main" || b.Name == "master",
			Protected: b.Protected,
		}
	}

	return branches, nil
}

func (g *GitService) getGitHubCommits(ctx context.Context, repo *Repository, branch string, limit int, token string) ([]*Commit, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits?sha=%s&per_page=%d",
		repo.RepoOwner, repo.RepoName, branch, limit)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API error: %d", resp.StatusCode)
	}

	var ghCommits []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name  string    `json:"name"`
				Email string    `json:"email"`
				Date  time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghCommits); err != nil {
		return nil, err
	}

	commits := make([]*Commit, len(ghCommits))
	for i, c := range ghCommits {
		commits[i] = &Commit{
			SHA:       c.SHA,
			Message:   c.Commit.Message,
			Author:    c.Commit.Author.Name,
			Email:     c.Commit.Author.Email,
			Timestamp: c.Commit.Author.Date,
		}
	}

	return commits, nil
}

func (g *GitService) createGitHubCommit(ctx context.Context, repo *Repository, message string, filePaths []string, token string, projectID uint) (*Commit, error) {
	// Step 1: Get the latest commit SHA
	refURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/ref/heads/%s",
		repo.RepoOwner, repo.RepoName, repo.Branch)

	req, _ := http.NewRequestWithContext(ctx, "GET", refURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ref); err != nil {
		return nil, err
	}

	baseSHA := ref.Object.SHA

	// Step 2: Get the base tree
	commitURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/commits/%s",
		repo.RepoOwner, repo.RepoName, baseSHA)

	req, _ = http.NewRequestWithContext(ctx, "GET", commitURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var commit struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&commit); err != nil {
		return nil, err
	}

	baseTreeSHA := commit.Tree.SHA

	// Step 3: Create blobs for each file
	var treeEntries []map[string]interface{}
	for _, path := range filePaths {
		var file models.File
		if err := g.db.WithContext(ctx).Where("project_id = ? AND path = ?", projectID, path).First(&file).Error; err != nil {
			continue
		}

		// Create blob
		blobURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/blobs", repo.RepoOwner, repo.RepoName)
		blobData := map[string]string{
			"content":  base64.StdEncoding.EncodeToString([]byte(file.Content)),
			"encoding": "base64",
		}
		blobJSON, _ := json.Marshal(blobData)

		req, _ = http.NewRequestWithContext(ctx, "POST", blobURL, strings.NewReader(string(blobJSON)))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		req.Header.Set("Content-Type", "application/json")

		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			continue
		}

		var blob struct {
			SHA string `json:"sha"`
		}
		json.NewDecoder(resp.Body).Decode(&blob)
		resp.Body.Close()

		treeEntries = append(treeEntries, map[string]interface{}{
			"path": path,
			"mode": "100644",
			"type": "blob",
			"sha":  blob.SHA,
		})
	}

	// Step 4: Create tree
	treeURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees", repo.RepoOwner, repo.RepoName)
	treeData := map[string]interface{}{
		"base_tree": baseTreeSHA,
		"tree":      treeEntries,
	}
	treeJSON, _ := json.Marshal(treeData)

	req, _ = http.NewRequestWithContext(ctx, "POST", treeURL, strings.NewReader(string(treeJSON)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tree struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, err
	}

	// Step 5: Create commit
	createCommitURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/commits", repo.RepoOwner, repo.RepoName)
	commitData := map[string]interface{}{
		"message": message,
		"tree":    tree.SHA,
		"parents": []string{baseSHA},
	}
	commitJSON, _ := json.Marshal(commitData)

	req, _ = http.NewRequestWithContext(ctx, "POST", createCommitURL, strings.NewReader(string(commitJSON)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var newCommit struct {
		SHA    string `json:"sha"`
		Author struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"author"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&newCommit); err != nil {
		return nil, err
	}

	// Step 6: Update ref
	updateRefURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs/heads/%s",
		repo.RepoOwner, repo.RepoName, repo.Branch)
	refData := map[string]interface{}{
		"sha":   newCommit.SHA,
		"force": false,
	}
	refJSON, _ := json.Marshal(refData)

	req, _ = http.NewRequestWithContext(ctx, "PATCH", updateRefURL, strings.NewReader(string(refJSON)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	return &Commit{
		SHA:       newCommit.SHA,
		Message:   message,
		Author:    newCommit.Author.Name,
		Email:     newCommit.Author.Email,
		Timestamp: newCommit.Author.Date,
		Files:     filePaths,
	}, nil
}

func (g *GitService) pullFromGitHub(ctx context.Context, repo *Repository, token string, projectID uint) error {
	// Get the tree for the current branch
	treeURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1",
		repo.RepoOwner, repo.RepoName, repo.Branch)

	req, _ := http.NewRequestWithContext(ctx, "GET", treeURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
		} `json:"tree"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return err
	}

	// Download and update each file
	for _, item := range tree.Tree {
		if item.Type != "blob" {
			continue
		}

		// Get file content
		blobURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/blobs/%s",
			repo.RepoOwner, repo.RepoName, item.SHA)

		req, _ := http.NewRequestWithContext(ctx, "GET", blobURL, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github.v3+json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}

		var blob struct {
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
		}
		json.NewDecoder(resp.Body).Decode(&blob)
		resp.Body.Close()

		var content string
		if blob.Encoding == "base64" {
			decoded, _ := base64.StdEncoding.DecodeString(strings.ReplaceAll(blob.Content, "\n", ""))
			content = string(decoded)
		} else {
			content = blob.Content
		}

		// Update or create file in database
		var file models.File
		result := g.db.WithContext(ctx).Where("project_id = ? AND path = ?", projectID, item.Path).First(&file)
		if result.Error != nil {
			// Create new file
			file = models.File{
				ProjectID: projectID,
				Name:      g.getFileName(item.Path),
				Path:      item.Path,
				Content:   content,
				Size:      int64(len(content)),
				MimeType:  g.getMimeType(item.Path),
				Version:   1,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			g.db.WithContext(ctx).Create(&file)
		} else {
			// Update existing file
			file.Content = content
			file.Size = int64(len(content))
			file.UpdatedAt = time.Now()
			g.db.WithContext(ctx).Save(&file)
		}
	}

	// Update last sync time
	repo.LastSync = time.Now()
	g.db.WithContext(ctx).Save(repo)

	return nil
}

func (g *GitService) createGitHubBranch(ctx context.Context, repo *Repository, branchName, baseBranch, token string) (*Branch, error) {
	// Get base branch SHA
	refURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/ref/heads/%s",
		repo.RepoOwner, repo.RepoName, baseBranch)

	req, _ := http.NewRequestWithContext(ctx, "GET", refURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ref); err != nil {
		return nil, err
	}

	// Create new ref
	createRefURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs", repo.RepoOwner, repo.RepoName)
	refData := map[string]string{
		"ref": "refs/heads/" + branchName,
		"sha": ref.Object.SHA,
	}
	refJSON, _ := json.Marshal(refData)

	req, _ = http.NewRequestWithContext(ctx, "POST", createRefURL, strings.NewReader(string(refJSON)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create branch: %s", string(body))
	}

	return &Branch{
		Name:      branchName,
		SHA:       ref.Object.SHA,
		IsDefault: false,
		Protected: false,
	}, nil
}

func (g *GitService) getGitHubPullRequests(ctx context.Context, repo *Repository, state, token string) ([]*PullRequest, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=%s",
		repo.RepoOwner, repo.RepoName, state)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ghPRs []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		State  string `json:"state"`
		User   struct {
			Login string `json:"login"`
		} `json:"user"`
		Head struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		HTMLURL   string    `json:"html_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghPRs); err != nil {
		return nil, err
	}

	prs := make([]*PullRequest, len(ghPRs))
	for i, pr := range ghPRs {
		prs[i] = &PullRequest{
			Number:     pr.Number,
			Title:      pr.Title,
			Body:       pr.Body,
			State:      pr.State,
			Author:     pr.User.Login,
			Branch:     pr.Head.Ref,
			BaseBranch: pr.Base.Ref,
			CreatedAt:  pr.CreatedAt,
			UpdatedAt:  pr.UpdatedAt,
			URL:        pr.HTMLURL,
		}
	}

	return prs, nil
}

func (g *GitService) createGitHubPullRequest(ctx context.Context, repo *Repository, title, body, head, base, token string) (*PullRequest, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", repo.RepoOwner, repo.RepoName)

	prData := map[string]string{
		"title": title,
		"body":  body,
		"head":  head,
		"base":  base,
	}
	prJSON, _ := json.Marshal(prData)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(prJSON)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create PR: %s", string(body))
	}

	var pr struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		State  string `json:"state"`
		User   struct {
			Login string `json:"login"`
		} `json:"user"`
		Head struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		HTMLURL   string    `json:"html_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, err
	}

	return &PullRequest{
		Number:     pr.Number,
		Title:      pr.Title,
		Body:       pr.Body,
		State:      pr.State,
		Author:     pr.User.Login,
		Branch:     pr.Head.Ref,
		BaseBranch: pr.Base.Ref,
		CreatedAt:  pr.CreatedAt,
		UpdatedAt:  pr.UpdatedAt,
		URL:        pr.HTMLURL,
	}, nil
}

// ExportResult contains the result of exporting a project to GitHub
type ExportResult struct {
	RepoURL   string `json:"repo_url"`
	RepoOwner string `json:"repo_owner"`
	RepoName  string `json:"repo_name"`
	CommitSHA string `json:"commit_sha"`
	Branch    string `json:"branch"`
	FileCount int    `json:"file_count"`
}

// ExportToGitHub creates a new GitHub repository and pushes all project files to it
func (g *GitService) ExportToGitHub(ctx context.Context, projectID uint, repoName, description, token string, isPrivate bool) (*ExportResult, error) {
	// Step 1: Get all project files from the database
	var files []models.File
	if err := g.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&files).Error; err != nil {
		return nil, fmt.Errorf("failed to get project files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("project has no files to export")
	}

	// Step 2: Get the authenticated user's GitHub username
	owner, err := g.getGitHubUser(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get GitHub user: %w", err)
	}

	// Step 3: Create the GitHub repository
	if err := g.createGitHubRepo(ctx, repoName, description, isPrivate, token); err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	// Step 4: Create an initial commit with all files using the Git Data API
	// We need to build: blobs → tree → commit → update ref (creating the default branch)
	var treeEntries []map[string]interface{}
	fileCount := 0

	for _, file := range files {
		if file.Type == "directory" || file.Content == "" {
			continue
		}

		path := file.Path
		if len(path) > 0 && path[0] == '/' {
			path = path[1:]
		}

		// Create blob
		blobURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/blobs", owner, repoName)
		blobData := map[string]string{
			"content":  base64.StdEncoding.EncodeToString([]byte(file.Content)),
			"encoding": "base64",
		}
		blobJSON, _ := json.Marshal(blobData)

		req, _ := http.NewRequestWithContext(ctx, "POST", blobURL, strings.NewReader(string(blobJSON)))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}

		var blob struct {
			SHA string `json:"sha"`
		}
		json.NewDecoder(resp.Body).Decode(&blob)
		resp.Body.Close()

		if blob.SHA == "" {
			continue
		}

		treeEntries = append(treeEntries, map[string]interface{}{
			"path": path,
			"mode": "100644",
			"type": "blob",
			"sha":  blob.SHA,
		})
		fileCount++
	}

	if fileCount == 0 {
		return nil, fmt.Errorf("no valid files to export")
	}

	// Create tree (no base_tree since this is the initial commit)
	treeURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees", owner, repoName)
	treeData := map[string]interface{}{
		"tree": treeEntries,
	}
	treeJSON, _ := json.Marshal(treeData)

	treeReq, _ := http.NewRequestWithContext(ctx, "POST", treeURL, strings.NewReader(string(treeJSON)))
	treeReq.Header.Set("Authorization", "Bearer "+token)
	treeReq.Header.Set("Accept", "application/vnd.github.v3+json")
	treeReq.Header.Set("Content-Type", "application/json")

	treeResp, err := http.DefaultClient.Do(treeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create tree: %w", err)
	}
	defer treeResp.Body.Close()

	if treeResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(treeResp.Body)
		return nil, fmt.Errorf("GitHub tree creation failed (%d): %s", treeResp.StatusCode, string(body))
	}

	var tree struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(treeResp.Body).Decode(&tree); err != nil {
		return nil, fmt.Errorf("failed to decode tree response: %w", err)
	}

	// Create initial commit (no parents)
	commitURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/commits", owner, repoName)
	commitData := map[string]interface{}{
		"message": "Initial commit — exported from APEX.BUILD",
		"tree":    tree.SHA,
		"parents": []string{},
	}
	commitJSON, _ := json.Marshal(commitData)

	commitReq, _ := http.NewRequestWithContext(ctx, "POST", commitURL, strings.NewReader(string(commitJSON)))
	commitReq.Header.Set("Authorization", "Bearer "+token)
	commitReq.Header.Set("Accept", "application/vnd.github.v3+json")
	commitReq.Header.Set("Content-Type", "application/json")

	commitResp, err := http.DefaultClient.Do(commitReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create commit: %w", err)
	}
	defer commitResp.Body.Close()

	if commitResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(commitResp.Body)
		return nil, fmt.Errorf("GitHub commit creation failed (%d): %s", commitResp.StatusCode, string(body))
	}

	var newCommit struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(commitResp.Body).Decode(&newCommit); err != nil {
		return nil, fmt.Errorf("failed to decode commit response: %w", err)
	}

	// Create the main ref pointing to our commit
	refURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs", owner, repoName)
	refData := map[string]string{
		"ref": "refs/heads/main",
		"sha": newCommit.SHA,
	}
	refJSON, _ := json.Marshal(refData)

	refReq, _ := http.NewRequestWithContext(ctx, "POST", refURL, strings.NewReader(string(refJSON)))
	refReq.Header.Set("Authorization", "Bearer "+token)
	refReq.Header.Set("Accept", "application/vnd.github.v3+json")
	refReq.Header.Set("Content-Type", "application/json")

	refResp, err := http.DefaultClient.Do(refReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create ref: %w", err)
	}
	refResp.Body.Close()

	// Step 5: Optionally connect the repository to the project
	repoRecord := &Repository{
		ProjectID:   projectID,
		RemoteURL:   fmt.Sprintf("https://github.com/%s/%s", owner, repoName),
		Provider:    "github",
		RepoOwner:   owner,
		RepoName:    repoName,
		Branch:      "main",
		LastSync:    time.Now(),
		IsConnected: true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	g.db.WithContext(ctx).Save(repoRecord)

	return &ExportResult{
		RepoURL:   fmt.Sprintf("https://github.com/%s/%s", owner, repoName),
		RepoOwner: owner,
		RepoName:  repoName,
		CommitSHA: newCommit.SHA,
		Branch:    "main",
		FileCount: fileCount,
	}, nil
}

// getGitHubUser returns the authenticated user's login name
func (g *GitService) getGitHubUser(ctx context.Context, token string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub user API failed (%d): %s", resp.StatusCode, string(body))
	}

	var user struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", err
	}
	return user.Login, nil
}

// createGitHubRepo creates a new GitHub repository (auto_init=false so we control the initial commit)
func (g *GitService) createGitHubRepo(ctx context.Context, name, description string, isPrivate bool, token string) error {
	repoData := map[string]interface{}{
		"name":        name,
		"description": description,
		"private":     isPrivate,
		"auto_init":   false,
	}
	repoJSON, _ := json.Marshal(repoData)

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/user/repos", strings.NewReader(string(repoJSON)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub repo creation failed (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// Helper methods

func (g *GitService) parseRemoteURL(url string) (provider, owner, name string) {
	// Handle various URL formats
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "git@")

	if strings.HasPrefix(url, "github.com") {
		parts := strings.Split(strings.TrimPrefix(url, "github.com/"), "/")
		parts = strings.Split(strings.TrimPrefix(strings.Join(parts, "/"), ":"), "/")
		if len(parts) >= 2 {
			return "github", parts[0], parts[1]
		}
	}

	if strings.HasPrefix(url, "gitlab.com") {
		parts := strings.Split(strings.TrimPrefix(url, "gitlab.com/"), "/")
		if len(parts) >= 2 {
			return "gitlab", parts[0], parts[1]
		}
	}

	return "", "", ""
}

func (g *GitService) verifyRepositoryAccess(ctx context.Context, provider, owner, name, token string) bool {
	if provider != "github" {
		return false
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, name)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func (g *GitService) getFileName(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func (g *GitService) getMimeType(path string) string {
	ext := ""
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			ext = path[i:]
			break
		}
	}

	mimeTypes := map[string]string{
		".js":   "text/javascript",
		".ts":   "text/typescript",
		".tsx":  "text/typescript",
		".jsx":  "text/javascript",
		".json": "application/json",
		".html": "text/html",
		".css":  "text/css",
		".md":   "text/markdown",
		".py":   "text/x-python",
		".go":   "text/x-go",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "text/plain"
}
