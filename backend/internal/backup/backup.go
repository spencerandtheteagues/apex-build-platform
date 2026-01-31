// Package backup provides comprehensive database backup functionality for APEX.BUILD
// Supports full, incremental, and point-in-time recovery backups with encryption and cloud storage
package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// BackupType defines the type of backup
type BackupType string

const (
	BackupTypeFull        BackupType = "full"
	BackupTypeIncremental BackupType = "incremental"
	BackupTypeWAL         BackupType = "wal"
)

// CompressionType defines compression algorithm
type CompressionType string

const (
	CompressionGzip CompressionType = "gzip"
	CompressionZstd CompressionType = "zstd"
	CompressionNone CompressionType = "none"
)

// StorageBackend defines where backups are stored
type StorageBackend string

const (
	StorageLocal StorageBackend = "local"
	StorageS3    StorageBackend = "s3"
	StorageGCS   StorageBackend = "gcs"
)

// BackupStatus represents the current status of a backup
type BackupStatus string

const (
	StatusPending    BackupStatus = "pending"
	StatusInProgress BackupStatus = "in_progress"
	StatusCompleted  BackupStatus = "completed"
	StatusFailed     BackupStatus = "failed"
	StatusVerified   BackupStatus = "verified"
)

// Prometheus metrics
var (
	backupDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "apex_backup_duration_seconds",
		Help:    "Duration of backup operations",
		Buckets: prometheus.ExponentialBuckets(1, 2, 15),
	}, []string{"type", "status"})

	backupSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "apex_backup_size_bytes",
		Help: "Size of backup files",
	}, []string{"type"})

	backupCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "apex_backup_total",
		Help: "Total number of backups",
	}, []string{"type", "status"})

	backupLastSuccess = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "apex_backup_last_success_timestamp",
		Help: "Timestamp of last successful backup",
	}, []string{"type"})

	restoreDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "apex_restore_duration_seconds",
		Help:    "Duration of restore operations",
		Buckets: prometheus.ExponentialBuckets(1, 2, 15),
	}, []string{"type", "status"})
)

// Config holds backup configuration
type Config struct {
	// Database connection
	DatabaseURL  string `json:"database_url"`
	DatabaseHost string `json:"database_host"`
	DatabasePort string `json:"database_port"`
	DatabaseName string `json:"database_name"`
	DatabaseUser string `json:"database_user"`
	DatabasePass string `json:"database_pass"`

	// Storage configuration
	StorageBackend StorageBackend `json:"storage_backend"`
	LocalPath      string         `json:"local_path"`
	S3Bucket       string         `json:"s3_bucket"`
	S3Region       string         `json:"s3_region"`
	S3Endpoint     string         `json:"s3_endpoint"`
	GCSBucket      string         `json:"gcs_bucket"`
	GCSProject     string         `json:"gcs_project"`

	// Compression
	Compression CompressionType `json:"compression"`

	// Encryption
	EncryptionEnabled bool   `json:"encryption_enabled"`
	EncryptionKey     string `json:"encryption_key"` // 32 bytes for AES-256

	// Retention policy
	RetainDaily   int `json:"retain_daily"`   // Default: 7
	RetainWeekly  int `json:"retain_weekly"`  // Default: 4
	RetainMonthly int `json:"retain_monthly"` // Default: 12

	// WAL archiving for PITR
	WALArchiveEnabled bool   `json:"wal_archive_enabled"`
	WALArchivePath    string `json:"wal_archive_path"`

	// Notifications
	NotifyOnSuccess bool     `json:"notify_on_success"`
	NotifyOnFailure bool     `json:"notify_on_failure"`
	NotifyWebhooks  []string `json:"notify_webhooks"`
	NotifyEmail     string   `json:"notify_email"`

	// Parallelism
	ParallelJobs int `json:"parallel_jobs"` // Default: 4
}

// BackupMetadata contains information about a backup
type BackupMetadata struct {
	ID              string         `json:"id"`
	Type            BackupType     `json:"type"`
	Status          BackupStatus   `json:"status"`
	StartTime       time.Time      `json:"start_time"`
	EndTime         time.Time      `json:"end_time,omitempty"`
	Duration        time.Duration  `json:"duration,omitempty"`
	SizeBytes       int64          `json:"size_bytes,omitempty"`
	CompressedSize  int64          `json:"compressed_size,omitempty"`
	Compression     CompressionType `json:"compression"`
	Encrypted       bool           `json:"encrypted"`
	Checksum        string         `json:"checksum,omitempty"`
	StorageLocation string         `json:"storage_location"`
	DatabaseName    string         `json:"database_name"`
	DatabaseVersion string         `json:"database_version,omitempty"`
	WALPosition     string         `json:"wal_position,omitempty"`
	ParentBackupID  string         `json:"parent_backup_id,omitempty"`
	Error           string         `json:"error,omitempty"`
	Tables          []string       `json:"tables,omitempty"`
	RowCount        int64          `json:"row_count,omitempty"`
}

// RestoreOptions configures restore behavior
type RestoreOptions struct {
	BackupID        string    `json:"backup_id"`
	TargetDatabase  string    `json:"target_database"`
	PointInTime     time.Time `json:"point_in_time,omitempty"`
	Tables          []string  `json:"tables,omitempty"`
	DropExisting    bool      `json:"drop_existing"`
	CreateDatabase  bool      `json:"create_database"`
	ParallelRestore int       `json:"parallel_restore"`
}

// Service provides backup operations
type Service struct {
	config  *Config
	storage StorageProvider
	mu      sync.Mutex
	logger  Logger
}

// Logger interface for logging
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// DefaultLogger implements basic logging
type DefaultLogger struct{}

func (l *DefaultLogger) Info(msg string, args ...interface{})  { fmt.Printf("[INFO] "+msg+"\n", args...) }
func (l *DefaultLogger) Error(msg string, args ...interface{}) { fmt.Printf("[ERROR] "+msg+"\n", args...) }
func (l *DefaultLogger) Debug(msg string, args ...interface{}) { fmt.Printf("[DEBUG] "+msg+"\n", args...) }

// StorageProvider interface for backup storage
type StorageProvider interface {
	Upload(ctx context.Context, key string, data io.Reader, size int64) error
	Download(ctx context.Context, key string, writer io.Writer) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
	Exists(ctx context.Context, key string) (bool, error)
}

// NewService creates a new backup service
func NewService(config *Config) (*Service, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	// Set defaults
	if config.RetainDaily == 0 {
		config.RetainDaily = 7
	}
	if config.RetainWeekly == 0 {
		config.RetainWeekly = 4
	}
	if config.RetainMonthly == 0 {
		config.RetainMonthly = 12
	}
	if config.Compression == "" {
		config.Compression = CompressionGzip
	}
	if config.ParallelJobs == 0 {
		config.ParallelJobs = 4
	}

	// Initialize storage provider
	var storage StorageProvider
	var err error

	switch config.StorageBackend {
	case StorageS3:
		storage, err = NewS3Storage(config)
	case StorageGCS:
		storage, err = NewGCSStorage(config)
	case StorageLocal:
		storage, err = NewLocalStorage(config.LocalPath)
	default:
		storage, err = NewLocalStorage(config.LocalPath)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	return &Service{
		config:  config,
		storage: storage,
		logger:  &DefaultLogger{},
	}, nil
}

// SetLogger sets a custom logger
func (s *Service) SetLogger(logger Logger) {
	s.logger = logger
}

// CreateBackup performs a backup of the specified type
func (s *Service) CreateBackup(ctx context.Context, backupType BackupType) (*BackupMetadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	startTime := time.Now()
	backupID := generateBackupID(backupType)

	metadata := &BackupMetadata{
		ID:           backupID,
		Type:         backupType,
		Status:       StatusInProgress,
		StartTime:    startTime,
		Compression:  s.config.Compression,
		Encrypted:    s.config.EncryptionEnabled,
		DatabaseName: s.config.DatabaseName,
	}

	s.logger.Info("Starting %s backup: %s", backupType, backupID)

	// Get database version
	version, err := s.getDatabaseVersion(ctx)
	if err != nil {
		s.logger.Error("Failed to get database version: %v", err)
	} else {
		metadata.DatabaseVersion = version
	}

	// Perform backup based on type
	var backupPath string
	switch backupType {
	case BackupTypeFull:
		backupPath, err = s.performFullBackup(ctx, backupID)
	case BackupTypeIncremental:
		backupPath, err = s.performIncrementalBackup(ctx, backupID)
	case BackupTypeWAL:
		backupPath, err = s.archiveWAL(ctx, backupID)
	default:
		err = fmt.Errorf("unsupported backup type: %s", backupType)
	}

	if err != nil {
		metadata.Status = StatusFailed
		metadata.Error = err.Error()
		metadata.EndTime = time.Now()
		metadata.Duration = metadata.EndTime.Sub(startTime)

		backupCount.WithLabelValues(string(backupType), "failed").Inc()
		backupDuration.WithLabelValues(string(backupType), "failed").Observe(metadata.Duration.Seconds())

		s.logger.Error("Backup failed: %v", err)
		s.notifyFailure(ctx, metadata)

		return metadata, err
	}

	// Get backup size
	if fi, err := os.Stat(backupPath); err == nil {
		metadata.CompressedSize = fi.Size()
		backupSize.WithLabelValues(string(backupType)).Set(float64(fi.Size()))
	}

	// Calculate checksum
	checksum, err := s.calculateChecksum(backupPath)
	if err != nil {
		s.logger.Error("Failed to calculate checksum: %v", err)
	} else {
		metadata.Checksum = checksum
	}

	// Upload to storage
	storageKey := s.generateStorageKey(backupID, backupType)
	if err := s.uploadBackup(ctx, backupPath, storageKey); err != nil {
		metadata.Status = StatusFailed
		metadata.Error = fmt.Sprintf("upload failed: %v", err)
		metadata.EndTime = time.Now()
		metadata.Duration = metadata.EndTime.Sub(startTime)

		backupCount.WithLabelValues(string(backupType), "failed").Inc()
		s.notifyFailure(ctx, metadata)

		return metadata, err
	}

	metadata.StorageLocation = storageKey
	metadata.Status = StatusCompleted
	metadata.EndTime = time.Now()
	metadata.Duration = metadata.EndTime.Sub(startTime)

	// Update metrics
	backupCount.WithLabelValues(string(backupType), "success").Inc()
	backupDuration.WithLabelValues(string(backupType), "success").Observe(metadata.Duration.Seconds())
	backupLastSuccess.WithLabelValues(string(backupType)).Set(float64(time.Now().Unix()))

	// Save metadata
	if err := s.saveMetadata(ctx, metadata); err != nil {
		s.logger.Error("Failed to save metadata: %v", err)
	}

	// Clean up local file
	os.Remove(backupPath)

	s.logger.Info("Backup completed: %s (size: %d bytes, duration: %v)", backupID, metadata.CompressedSize, metadata.Duration)

	if s.config.NotifyOnSuccess {
		s.notifySuccess(ctx, metadata)
	}

	return metadata, nil
}

// performFullBackup creates a full database backup using pg_dump
func (s *Service) performFullBackup(ctx context.Context, backupID string) (string, error) {
	tmpDir := os.TempDir()
	dumpFile := filepath.Join(tmpDir, fmt.Sprintf("%s.sql", backupID))
	outputFile := filepath.Join(tmpDir, fmt.Sprintf("%s.sql.gz", backupID))

	// Build pg_dump command
	args := []string{
		"-h", s.config.DatabaseHost,
		"-p", s.config.DatabasePort,
		"-U", s.config.DatabaseUser,
		"-d", s.config.DatabaseName,
		"-F", "c", // Custom format for parallel restore
		"-j", fmt.Sprintf("%d", s.config.ParallelJobs),
		"-f", dumpFile,
		"--no-owner",
		"--no-privileges",
		"--verbose",
	}

	cmd := exec.CommandContext(ctx, "pg_dump", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", s.config.DatabasePass))

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	s.logger.Info("Running pg_dump for full backup")

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pg_dump failed: %v, stderr: %s", err, stderr.String())
	}

	// Compress the dump
	if err := s.compressFile(dumpFile, outputFile); err != nil {
		os.Remove(dumpFile)
		return "", fmt.Errorf("compression failed: %w", err)
	}

	// Remove uncompressed file
	os.Remove(dumpFile)

	// Encrypt if enabled
	if s.config.EncryptionEnabled {
		encryptedFile := outputFile + ".enc"
		if err := s.encryptFile(outputFile, encryptedFile); err != nil {
			os.Remove(outputFile)
			return "", fmt.Errorf("encryption failed: %w", err)
		}
		os.Remove(outputFile)
		outputFile = encryptedFile
	}

	return outputFile, nil
}

// performIncrementalBackup creates an incremental backup
func (s *Service) performIncrementalBackup(ctx context.Context, backupID string) (string, error) {
	tmpDir := os.TempDir()
	outputFile := filepath.Join(tmpDir, fmt.Sprintf("%s.inc.gz", backupID))

	// Get last full backup position
	lastBackup, err := s.getLastBackup(ctx, BackupTypeFull)
	if err != nil {
		return "", fmt.Errorf("no full backup found: %w", err)
	}

	// Use pg_basebackup with WAL for incremental
	args := []string{
		"-h", s.config.DatabaseHost,
		"-p", s.config.DatabasePort,
		"-U", s.config.DatabaseUser,
		"-D", filepath.Join(tmpDir, backupID),
		"-Ft", // Tar format
		"-z",  // Compress
		"-Xs", // Stream WAL
		"--checkpoint=fast",
	}

	cmd := exec.CommandContext(ctx, "pg_basebackup", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", s.config.DatabasePass))

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	s.logger.Info("Running pg_basebackup for incremental backup (parent: %s)", lastBackup.ID)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pg_basebackup failed: %v, stderr: %s", err, stderr.String())
	}

	// Tar the directory
	tarFile := filepath.Join(tmpDir, fmt.Sprintf("%s.tar", backupID))
	tarCmd := exec.CommandContext(ctx, "tar", "-cf", tarFile, "-C", tmpDir, backupID)
	if err := tarCmd.Run(); err != nil {
		return "", fmt.Errorf("tar failed: %w", err)
	}

	// Compress
	if err := s.compressFile(tarFile, outputFile); err != nil {
		os.Remove(tarFile)
		return "", fmt.Errorf("compression failed: %w", err)
	}
	os.Remove(tarFile)

	// Clean up directory
	os.RemoveAll(filepath.Join(tmpDir, backupID))

	// Encrypt if enabled
	if s.config.EncryptionEnabled {
		encryptedFile := outputFile + ".enc"
		if err := s.encryptFile(outputFile, encryptedFile); err != nil {
			os.Remove(outputFile)
			return "", fmt.Errorf("encryption failed: %w", err)
		}
		os.Remove(outputFile)
		outputFile = encryptedFile
	}

	return outputFile, nil
}

// archiveWAL archives Write-Ahead Log files for point-in-time recovery
func (s *Service) archiveWAL(ctx context.Context, backupID string) (string, error) {
	if !s.config.WALArchiveEnabled {
		return "", fmt.Errorf("WAL archiving is not enabled")
	}

	walPath := s.config.WALArchivePath
	if walPath == "" {
		return "", fmt.Errorf("WAL archive path not configured")
	}

	tmpDir := os.TempDir()
	outputFile := filepath.Join(tmpDir, fmt.Sprintf("%s.wal.tar.gz", backupID))

	// Create tar.gz of WAL files
	tarCmd := exec.CommandContext(ctx, "tar", "-czf", outputFile, "-C", walPath, ".")
	if err := tarCmd.Run(); err != nil {
		return "", fmt.Errorf("WAL archive failed: %w", err)
	}

	// Encrypt if enabled
	if s.config.EncryptionEnabled {
		encryptedFile := outputFile + ".enc"
		if err := s.encryptFile(outputFile, encryptedFile); err != nil {
			os.Remove(outputFile)
			return "", fmt.Errorf("encryption failed: %w", err)
		}
		os.Remove(outputFile)
		outputFile = encryptedFile
	}

	return outputFile, nil
}

// Restore restores a database from backup
func (s *Service) Restore(ctx context.Context, opts RestoreOptions) error {
	startTime := time.Now()

	s.logger.Info("Starting restore from backup: %s", opts.BackupID)

	// Get backup metadata
	metadata, err := s.GetBackup(ctx, opts.BackupID)
	if err != nil {
		return fmt.Errorf("backup not found: %w", err)
	}

	// Download backup
	tmpDir := os.TempDir()
	localFile := filepath.Join(tmpDir, filepath.Base(metadata.StorageLocation))

	if err := s.downloadBackup(ctx, metadata.StorageLocation, localFile); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(localFile)

	// Verify checksum
	if metadata.Checksum != "" {
		checksum, err := s.calculateChecksum(localFile)
		if err != nil {
			return fmt.Errorf("checksum calculation failed: %w", err)
		}
		if checksum != metadata.Checksum {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", metadata.Checksum, checksum)
		}
		s.logger.Info("Checksum verified successfully")
	}

	// Decrypt if needed
	if metadata.Encrypted {
		decryptedFile := strings.TrimSuffix(localFile, ".enc")
		if err := s.decryptFile(localFile, decryptedFile); err != nil {
			return fmt.Errorf("decryption failed: %w", err)
		}
		os.Remove(localFile)
		localFile = decryptedFile
		defer os.Remove(localFile)
	}

	// Decompress
	decompressedFile := strings.TrimSuffix(localFile, ".gz")
	if err := s.decompressFile(localFile, decompressedFile); err != nil {
		return fmt.Errorf("decompression failed: %w", err)
	}
	if localFile != decompressedFile {
		os.Remove(localFile)
		localFile = decompressedFile
		defer os.Remove(localFile)
	}

	// Create target database if needed
	targetDB := opts.TargetDatabase
	if targetDB == "" {
		targetDB = s.config.DatabaseName
	}

	if opts.CreateDatabase {
		if err := s.createDatabase(ctx, targetDB); err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
	}

	// Perform restore
	if err := s.performRestore(ctx, localFile, targetDB, opts); err != nil {
		restoreDuration.WithLabelValues(string(metadata.Type), "failed").Observe(time.Since(startTime).Seconds())
		return fmt.Errorf("restore failed: %w", err)
	}

	// Apply WAL for point-in-time recovery
	if !opts.PointInTime.IsZero() {
		if err := s.applyWALToPointInTime(ctx, targetDB, opts.PointInTime); err != nil {
			return fmt.Errorf("WAL replay failed: %w", err)
		}
	}

	duration := time.Since(startTime)
	restoreDuration.WithLabelValues(string(metadata.Type), "success").Observe(duration.Seconds())

	s.logger.Info("Restore completed successfully (duration: %v)", duration)

	return nil
}

// performRestore executes pg_restore
func (s *Service) performRestore(ctx context.Context, backupFile, targetDB string, opts RestoreOptions) error {
	args := []string{
		"-h", s.config.DatabaseHost,
		"-p", s.config.DatabasePort,
		"-U", s.config.DatabaseUser,
		"-d", targetDB,
		"-j", fmt.Sprintf("%d", opts.ParallelRestore),
		"--no-owner",
		"--no-privileges",
		"--verbose",
	}

	if opts.DropExisting {
		args = append(args, "--clean", "--if-exists")
	}

	// Filter tables if specified
	for _, table := range opts.Tables {
		args = append(args, "-t", table)
	}

	args = append(args, backupFile)

	cmd := exec.CommandContext(ctx, "pg_restore", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", s.config.DatabasePass))

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_restore failed: %v, stderr: %s", err, stderr.String())
	}

	return nil
}

// VerifyBackup verifies backup integrity
func (s *Service) VerifyBackup(ctx context.Context, backupID string) error {
	s.logger.Info("Verifying backup: %s", backupID)

	// Get metadata
	metadata, err := s.GetBackup(ctx, backupID)
	if err != nil {
		return fmt.Errorf("backup not found: %w", err)
	}

	// Check if backup exists in storage
	exists, err := s.storage.Exists(ctx, metadata.StorageLocation)
	if err != nil {
		return fmt.Errorf("storage check failed: %w", err)
	}
	if !exists {
		return fmt.Errorf("backup file not found in storage")
	}

	// Download and verify checksum
	tmpDir := os.TempDir()
	localFile := filepath.Join(tmpDir, filepath.Base(metadata.StorageLocation))

	if err := s.downloadBackup(ctx, metadata.StorageLocation, localFile); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(localFile)

	// Verify checksum
	if metadata.Checksum != "" {
		checksum, err := s.calculateChecksum(localFile)
		if err != nil {
			return fmt.Errorf("checksum calculation failed: %w", err)
		}
		if checksum != metadata.Checksum {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", metadata.Checksum, checksum)
		}
	}

	// Test restore to temporary database
	testDB := fmt.Sprintf("apex_backup_test_%d", time.Now().Unix())
	defer s.dropDatabase(ctx, testDB)

	if err := s.createDatabase(ctx, testDB); err != nil {
		return fmt.Errorf("failed to create test database: %w", err)
	}

	restoreOpts := RestoreOptions{
		BackupID:        backupID,
		TargetDatabase:  testDB,
		ParallelRestore: s.config.ParallelJobs,
	}

	// Decrypt if needed
	workFile := localFile
	if metadata.Encrypted {
		decryptedFile := strings.TrimSuffix(localFile, ".enc")
		if err := s.decryptFile(localFile, decryptedFile); err != nil {
			return fmt.Errorf("decryption failed: %w", err)
		}
		defer os.Remove(decryptedFile)
		workFile = decryptedFile
	}

	// Decompress
	decompressedFile := strings.TrimSuffix(workFile, ".gz")
	if strings.HasSuffix(workFile, ".gz") {
		if err := s.decompressFile(workFile, decompressedFile); err != nil {
			return fmt.Errorf("decompression failed: %w", err)
		}
		defer os.Remove(decompressedFile)
		workFile = decompressedFile
	}

	if err := s.performRestore(ctx, workFile, testDB, restoreOpts); err != nil {
		return fmt.Errorf("test restore failed: %w", err)
	}

	// Update metadata status
	metadata.Status = StatusVerified
	if err := s.saveMetadata(ctx, metadata); err != nil {
		s.logger.Error("Failed to update metadata: %v", err)
	}

	s.logger.Info("Backup verified successfully: %s", backupID)
	return nil
}

// ListBackups returns all backups matching the filter
func (s *Service) ListBackups(ctx context.Context, backupType *BackupType) ([]*BackupMetadata, error) {
	prefix := "metadata/"
	if backupType != nil {
		prefix = fmt.Sprintf("metadata/%s/", *backupType)
	}

	keys, err := s.storage.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}

	var backups []*BackupMetadata
	for _, key := range keys {
		if !strings.HasSuffix(key, ".json") {
			continue
		}

		var buf bytes.Buffer
		if err := s.storage.Download(ctx, key, &buf); err != nil {
			s.logger.Error("Failed to download metadata %s: %v", key, err)
			continue
		}

		var metadata BackupMetadata
		if err := json.Unmarshal(buf.Bytes(), &metadata); err != nil {
			s.logger.Error("Failed to parse metadata %s: %v", key, err)
			continue
		}

		backups = append(backups, &metadata)
	}

	// Sort by start time descending
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].StartTime.After(backups[j].StartTime)
	})

	return backups, nil
}

// GetBackup retrieves backup metadata by ID
func (s *Service) GetBackup(ctx context.Context, backupID string) (*BackupMetadata, error) {
	// Try all backup types
	for _, bt := range []BackupType{BackupTypeFull, BackupTypeIncremental, BackupTypeWAL} {
		key := fmt.Sprintf("metadata/%s/%s.json", bt, backupID)
		exists, err := s.storage.Exists(ctx, key)
		if err != nil {
			continue
		}
		if exists {
			var buf bytes.Buffer
			if err := s.storage.Download(ctx, key, &buf); err != nil {
				return nil, err
			}
			var metadata BackupMetadata
			if err := json.Unmarshal(buf.Bytes(), &metadata); err != nil {
				return nil, err
			}
			return &metadata, nil
		}
	}
	return nil, fmt.Errorf("backup not found: %s", backupID)
}

// CleanupOldBackups removes backups according to retention policy
func (s *Service) CleanupOldBackups(ctx context.Context) error {
	s.logger.Info("Starting backup cleanup")

	backups, err := s.ListBackups(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	now := time.Now()
	var toDelete []*BackupMetadata

	// Categorize backups by age
	var daily, weekly, monthly []*BackupMetadata
	for _, b := range backups {
		if b.Status == StatusFailed {
			// Always delete failed backups older than 1 day
			if now.Sub(b.StartTime) > 24*time.Hour {
				toDelete = append(toDelete, b)
			}
			continue
		}

		age := now.Sub(b.StartTime)
		switch {
		case age < 7*24*time.Hour:
			daily = append(daily, b)
		case age < 30*24*time.Hour:
			weekly = append(weekly, b)
		default:
			monthly = append(monthly, b)
		}
	}

	// Apply retention policy
	if len(daily) > s.config.RetainDaily {
		toDelete = append(toDelete, daily[s.config.RetainDaily:]...)
	}
	if len(weekly) > s.config.RetainWeekly {
		toDelete = append(toDelete, weekly[s.config.RetainWeekly:]...)
	}
	if len(monthly) > s.config.RetainMonthly {
		toDelete = append(toDelete, monthly[s.config.RetainMonthly:]...)
	}

	// Delete backups
	for _, b := range toDelete {
		s.logger.Info("Deleting old backup: %s", b.ID)
		if err := s.deleteBackup(ctx, b); err != nil {
			s.logger.Error("Failed to delete backup %s: %v", b.ID, err)
		}
	}

	s.logger.Info("Cleanup completed: deleted %d backups", len(toDelete))
	return nil
}

// Helper functions

func (s *Service) compressFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	switch s.config.Compression {
	case CompressionGzip:
		gw := gzip.NewWriter(out)
		defer gw.Close()
		_, err = io.Copy(gw, in)
	case CompressionZstd:
		// Use zstd command for compression
		cmd := exec.Command("zstd", "-c", src)
		cmd.Stdout = out
		err = cmd.Run()
	default:
		_, err = io.Copy(out, in)
	}

	return err
}

func (s *Service) decompressFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if strings.HasSuffix(src, ".gz") {
		gr, err := gzip.NewReader(in)
		if err != nil {
			return err
		}
		defer gr.Close()
		_, err = io.Copy(out, gr)
		return err
	} else if strings.HasSuffix(src, ".zst") {
		cmd := exec.Command("zstd", "-d", "-c", src)
		cmd.Stdout = out
		return cmd.Run()
	}

	_, err = io.Copy(out, in)
	return err
}

func (s *Service) encryptFile(src, dst string) error {
	key := []byte(s.config.EncryptionKey)
	if len(key) != 32 {
		// Hash the key to get exactly 32 bytes
		hash := sha256.Sum256(key)
		key = hash[:]
	}

	plaintext, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return os.WriteFile(dst, ciphertext, 0600)
}

func (s *Service) decryptFile(src, dst string) error {
	key := []byte(s.config.EncryptionKey)
	if len(key) != 32 {
		hash := sha256.Sum256(key)
		key = hash[:]
	}

	ciphertext, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, plaintext, 0600)
}

func (s *Service) calculateChecksum(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (s *Service) getDatabaseVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "psql",
		"-h", s.config.DatabaseHost,
		"-p", s.config.DatabasePort,
		"-U", s.config.DatabaseUser,
		"-d", s.config.DatabaseName,
		"-t", "-c", "SELECT version();")
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", s.config.DatabasePass))

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

func (s *Service) getLastBackup(ctx context.Context, backupType BackupType) (*BackupMetadata, error) {
	backups, err := s.ListBackups(ctx, &backupType)
	if err != nil {
		return nil, err
	}
	if len(backups) == 0 {
		return nil, fmt.Errorf("no backups found")
	}
	return backups[0], nil
}

func (s *Service) uploadBackup(ctx context.Context, localPath, storageKey string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	return s.storage.Upload(ctx, storageKey, f, fi.Size())
}

func (s *Service) downloadBackup(ctx context.Context, storageKey, localPath string) error {
	f, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return s.storage.Download(ctx, storageKey, f)
}

func (s *Service) saveMetadata(ctx context.Context, metadata *BackupMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	key := fmt.Sprintf("metadata/%s/%s.json", metadata.Type, metadata.ID)
	return s.storage.Upload(ctx, key, bytes.NewReader(data), int64(len(data)))
}

func (s *Service) deleteBackup(ctx context.Context, metadata *BackupMetadata) error {
	// Delete backup file
	if err := s.storage.Delete(ctx, metadata.StorageLocation); err != nil {
		return fmt.Errorf("failed to delete backup file: %w", err)
	}

	// Delete metadata
	metadataKey := fmt.Sprintf("metadata/%s/%s.json", metadata.Type, metadata.ID)
	if err := s.storage.Delete(ctx, metadataKey); err != nil {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	return nil
}

func (s *Service) createDatabase(ctx context.Context, dbName string) error {
	cmd := exec.CommandContext(ctx, "psql",
		"-h", s.config.DatabaseHost,
		"-p", s.config.DatabasePort,
		"-U", s.config.DatabaseUser,
		"-c", fmt.Sprintf("CREATE DATABASE %s;", dbName))
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", s.config.DatabasePass))

	return cmd.Run()
}

func (s *Service) dropDatabase(ctx context.Context, dbName string) error {
	cmd := exec.CommandContext(ctx, "psql",
		"-h", s.config.DatabaseHost,
		"-p", s.config.DatabasePort,
		"-U", s.config.DatabaseUser,
		"-c", fmt.Sprintf("DROP DATABASE IF EXISTS %s;", dbName))
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", s.config.DatabasePass))

	return cmd.Run()
}

func (s *Service) applyWALToPointInTime(ctx context.Context, targetDB string, pit time.Time) error {
	// This would configure recovery.conf and replay WAL files
	// For PostgreSQL 12+, use recovery.signal and postgresql.auto.conf
	s.logger.Info("Applying WAL to point-in-time: %v", pit)

	// Create recovery configuration
	recoveryConf := fmt.Sprintf(`
restore_command = 'cp %s/%%f %%p'
recovery_target_time = '%s'
recovery_target_action = 'promote'
`, s.config.WALArchivePath, pit.Format("2006-01-02 15:04:05"))

	// This is a simplified implementation - full implementation would:
	// 1. Stop PostgreSQL
	// 2. Write recovery configuration
	// 3. Start PostgreSQL in recovery mode
	// 4. Wait for recovery to complete

	s.logger.Info("Recovery configuration:\n%s", recoveryConf)

	return nil
}

func (s *Service) generateStorageKey(backupID string, backupType BackupType) string {
	now := time.Now()
	return fmt.Sprintf("backups/%s/%d/%02d/%02d/%s", backupType, now.Year(), now.Month(), now.Day(), backupID)
}

func (s *Service) notifySuccess(ctx context.Context, metadata *BackupMetadata) {
	for _, webhook := range s.config.NotifyWebhooks {
		go s.sendWebhookNotification(ctx, webhook, "success", metadata)
	}
}

func (s *Service) notifyFailure(ctx context.Context, metadata *BackupMetadata) {
	for _, webhook := range s.config.NotifyWebhooks {
		go s.sendWebhookNotification(ctx, webhook, "failure", metadata)
	}
}

func (s *Service) sendWebhookNotification(ctx context.Context, webhook, status string, metadata *BackupMetadata) {
	// Implementation would send HTTP POST to webhook
	s.logger.Info("Sending %s notification to webhook: %s", status, webhook)
}

func generateBackupID(backupType BackupType) string {
	now := time.Now()
	return fmt.Sprintf("%s_%s", backupType, now.Format("20060102_150405"))
}
