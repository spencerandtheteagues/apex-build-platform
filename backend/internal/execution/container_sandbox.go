// APEX.BUILD Container-Based Code Execution Sandbox
// Production-grade secure, isolated code execution using Docker containers
// with seccomp profiles, network isolation, and strict resource limits.

package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// ContainerSandbox provides Docker-based isolated code execution
type ContainerSandbox struct {
	config          *ContainerSandboxConfig
	executions      map[string]*containerExecution
	executionsMu    sync.RWMutex
	baseTempDir     string
	seccompProfile  string
	auditLogger     *AuditLogger
	dockerAvailable bool
	imageCache      map[string]bool
	imageCacheMu    sync.RWMutex
	stats           *SandboxStats
	pkgCache        *PackageCacheManager
}

// ContainerSandboxConfig holds container sandbox configuration
type ContainerSandboxConfig struct {
	// Docker socket path (default: /var/run/docker.sock)
	DockerSocket string

	// Base image prefix for language containers
	ImagePrefix string

	// Default resource limits
	DefaultMemoryLimit int64         // bytes (default: 256MB)
	DefaultCPULimit    float64       // cores (default: 0.5)
	DefaultTimeout     time.Duration // (default: 30s)
	DefaultPidsLimit   int64         // max processes (default: 100)

	// Per-language resource overrides
	LanguageLimits map[string]*LanguageResourceLimits

	// Security settings
	EnableSeccomp       bool
	EnableAppArmor      bool
	EnableReadOnlyRoot  bool
	DropAllCapabilities bool
	NoNewPrivileges     bool

	// Network settings
	DisableNetwork bool
	NetworkMode    string // none, bridge, host (default: none)

	// Filesystem settings
	TmpfsSize           string // size of /tmp tmpfs mount (default: 64m)
	WorkDirSize         string // size of /work tmpfs mount (default: 32m)
	EnablePackageCache  bool
	PackageCacheBaseDir string

	// Logging
	EnableAuditLog bool
	AuditLogPath   string

	// Cleanup settings
	AutoCleanup     bool
	CleanupInterval time.Duration
	MaxContainerAge time.Duration

	// Concurrent execution limits
	MaxConcurrentExecs int32
}

// LanguageResourceLimits defines per-language resource constraints
type LanguageResourceLimits struct {
	MemoryLimit int64         // bytes
	CPULimit    float64       // cores
	Timeout     time.Duration // max execution time
	PidsLimit   int64         // max processes
	TmpfsSize   string        // /tmp size
}

// containerExecution tracks an active container execution
type containerExecution struct {
	ID          string
	ContainerID string
	Language    string
	StartTime   time.Time
	TempDir     string
	Cancel      context.CancelFunc
	Done        chan struct{}
}

// AuditLogger handles security audit logging
type AuditLogger struct {
	logPath string
	mu      sync.Mutex
	file    *os.File
}

// AuditEntry represents a single audit log entry
type AuditEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	ExecutionID string    `json:"execution_id"`
	ContainerID string    `json:"container_id,omitempty"`
	Language    string    `json:"language"`
	Action      string    `json:"action"` // start, complete, timeout, kill, error
	Duration    int64     `json:"duration_ms,omitempty"`
	ExitCode    int       `json:"exit_code,omitempty"`
	MemoryUsed  int64     `json:"memory_used,omitempty"`
	Error       string    `json:"error,omitempty"`
	CodeHash    string    `json:"code_hash,omitempty"`
}

// SandboxStats tracks execution statistics
type SandboxStats struct {
	TotalExecutions    int64
	SuccessfulExecs    int64
	FailedExecs        int64
	TimeoutExecs       int64
	KilledExecs        int64
	ConcurrentExecs    int32
	MaxConcurrentExecs int32
	TotalCPUTime       int64
	TotalMemoryUsed    int64
}

// DefaultContainerSandboxConfig returns production-ready default configuration
func DefaultContainerSandboxConfig() *ContainerSandboxConfig {
	return &ContainerSandboxConfig{
		DockerSocket:        "/var/run/docker.sock",
		ImagePrefix:         "apex-sandbox",
		DefaultMemoryLimit:  256 * 1024 * 1024, // 256MB
		DefaultCPULimit:     0.5,
		DefaultTimeout:      30 * time.Second,
		DefaultPidsLimit:    100,
		EnableSeccomp:       true,
		EnableAppArmor:      runtime.GOOS == "linux",
		EnableReadOnlyRoot:  true,
		DropAllCapabilities: true,
		NoNewPrivileges:     true,
		DisableNetwork:      true,
		NetworkMode:         "none",
		TmpfsSize:           "64m",
		WorkDirSize:         "32m",
		EnablePackageCache:  true,
		PackageCacheBaseDir: filepath.Join(os.TempDir(), "apex-sandbox-pkg-cache"),
		EnableAuditLog:      true,
		AuditLogPath:        "/var/log/apex-sandbox/audit.log",
		AutoCleanup:         true,
		CleanupInterval:     5 * time.Minute,
		MaxContainerAge:     10 * time.Minute,
		MaxConcurrentExecs:  50,
		LanguageLimits: map[string]*LanguageResourceLimits{
			"python": {
				MemoryLimit: 256 * 1024 * 1024,
				CPULimit:    0.5,
				Timeout:     30 * time.Second,
				PidsLimit:   50,
				TmpfsSize:   "64m",
			},
			"javascript": {
				MemoryLimit: 256 * 1024 * 1024,
				CPULimit:    0.5,
				Timeout:     30 * time.Second,
				PidsLimit:   50,
				TmpfsSize:   "64m",
			},
			"go": {
				MemoryLimit: 512 * 1024 * 1024,
				CPULimit:    1.0,
				Timeout:     60 * time.Second,
				PidsLimit:   100,
				TmpfsSize:   "128m",
			},
			"rust": {
				MemoryLimit: 512 * 1024 * 1024,
				CPULimit:    1.0,
				Timeout:     60 * time.Second,
				PidsLimit:   100,
				TmpfsSize:   "128m",
			},
			"java": {
				MemoryLimit: 512 * 1024 * 1024,
				CPULimit:    1.0,
				Timeout:     60 * time.Second,
				PidsLimit:   200,
				TmpfsSize:   "128m",
			},
			"c": {
				MemoryLimit: 128 * 1024 * 1024,
				CPULimit:    0.5,
				Timeout:     30 * time.Second,
				PidsLimit:   50,
				TmpfsSize:   "32m",
			},
			"cpp": {
				MemoryLimit: 256 * 1024 * 1024,
				CPULimit:    0.5,
				Timeout:     30 * time.Second,
				PidsLimit:   50,
				TmpfsSize:   "64m",
			},
		},
	}
}

// NewContainerSandbox creates a new container-based sandbox
func NewContainerSandbox(config *ContainerSandboxConfig) (*ContainerSandbox, error) {
	if config == nil {
		config = DefaultContainerSandboxConfig()
	}

	// Docker Desktop/non-Linux hosts can reject our custom seccomp profile while the
	// Linux production path should keep seccomp enabled by default.
	if runtime.GOOS != "linux" && config.EnableSeccomp {
		cfgCopy := *config
		cfgCopy.EnableSeccomp = false
		config = &cfgCopy
	}

	// Create base temp directory
	baseTempDir := filepath.Join(os.TempDir(), "apex-container-sandbox")
	if err := os.MkdirAll(baseTempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sandbox temp directory: %w", err)
	}

	sandbox := &ContainerSandbox{
		config:      config,
		executions:  make(map[string]*containerExecution),
		baseTempDir: baseTempDir,
		imageCache:  make(map[string]bool),
		stats:       &SandboxStats{},
		pkgCache:    NewPackageCacheManager(config.PackageCacheBaseDir, config.EnablePackageCache),
	}

	// Check Docker availability
	sandbox.dockerAvailable = sandbox.checkDockerAvailable()
	if !sandbox.dockerAvailable {
		return nil, fmt.Errorf("Docker is not available - container sandbox requires Docker")
	}

	// Generate seccomp profile
	seccompPath := filepath.Join(baseTempDir, "seccomp-profile.json")
	if err := sandbox.writeSeccompProfile(seccompPath); err != nil {
		return nil, fmt.Errorf("failed to write seccomp profile: %w", err)
	}
	sandbox.seccompProfile = seccompPath

	// Initialize audit logger
	if config.EnableAuditLog {
		auditDir := filepath.Dir(config.AuditLogPath)
		if err := os.MkdirAll(auditDir, 0750); err != nil {
			// Non-fatal: continue without audit logging
			fmt.Printf("Warning: could not create audit log directory: %v\n", err)
		} else {
			logger, err := NewAuditLogger(config.AuditLogPath)
			if err != nil {
				fmt.Printf("Warning: could not initialize audit logger: %v\n", err)
			} else {
				sandbox.auditLogger = logger
			}
		}
	}

	// Ensure sandbox images exist
	if err := sandbox.ensureImages(); err != nil {
		return nil, fmt.Errorf("failed to ensure sandbox images: %w", err)
	}

	// Start cleanup goroutine
	if config.AutoCleanup {
		go sandbox.cleanupLoop()
	}

	return sandbox, nil
}

// checkDockerAvailable verifies Docker daemon is accessible
func (s *ContainerSandbox) checkDockerAvailable() bool {
	cmd := osexec.Command("docker", "info")
	cmd.Env = append(os.Environ(), "DOCKER_HOST=unix://"+s.config.DockerSocket)
	return cmd.Run() == nil
}

// writeSeccompProfile creates a restrictive seccomp profile
func (s *ContainerSandbox) writeSeccompProfile(path string) error {
	profile := SeccompProfile{
		DefaultAction: "SCMP_ACT_ERRNO",
		Architectures: []string{
			"SCMP_ARCH_X86_64",
			"SCMP_ARCH_X86",
			"SCMP_ARCH_AARCH64",
			"SCMP_ARCH_ARM",
		},
		Syscalls: []SeccompSyscall{
			// Essential syscalls for process execution
			{Names: []string{"read", "write", "open", "close", "stat", "fstat", "lstat"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"poll", "lseek", "mmap", "mprotect", "munmap", "brk"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"rt_sigaction", "rt_sigprocmask", "rt_sigreturn", "ioctl"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"access", "pipe", "select", "sched_yield", "mremap"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"dup", "dup2", "pause", "nanosleep", "getitimer", "alarm"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"setitimer", "getpid", "socket", "connect", "sendto"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"recvfrom", "sendmsg", "recvmsg", "shutdown", "bind"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"listen", "getsockname", "getpeername", "socketpair"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"setsockopt", "getsockopt", "clone", "fork", "vfork"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"execve", "exit", "wait4", "kill", "uname", "fcntl"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"flock", "fsync", "fdatasync", "truncate", "ftruncate"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"getdents", "getcwd", "chdir", "fchdir", "rename"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"mkdir", "rmdir", "creat", "link", "unlink", "symlink"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"readlink", "chmod", "fchmod", "chown", "fchown"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"lchown", "umask", "gettimeofday", "getrlimit", "getrusage"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"sysinfo", "times", "getuid", "getgid", "setuid"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"setgid", "geteuid", "getegid", "setpgid", "getppid"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"getpgrp", "setsid", "setreuid", "setregid", "getgroups"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"setgroups", "setresuid", "getresuid", "setresgid"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"getresgid", "getpgid", "setfsuid", "setfsgid", "getsid"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"capget", "capset", "rt_sigpending", "rt_sigtimedwait"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"rt_sigqueueinfo", "sigaltstack", "utime", "mknod"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"personality", "statfs", "fstatfs", "getpriority"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"setpriority", "sched_setparam", "sched_getparam"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"sched_setscheduler", "sched_getscheduler"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"sched_get_priority_max", "sched_get_priority_min"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"sched_rr_get_interval", "mlock", "munlock", "mlockall"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"munlockall", "vhangup", "prctl", "arch_prctl"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"setrlimit", "sync", "acct", "settimeofday"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"sethostname", "setdomainname", "ioperm", "iopl"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"init_module", "delete_module", "quotactl"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"gettid", "readahead", "setxattr", "lsetxattr"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"fsetxattr", "getxattr", "lgetxattr", "fgetxattr"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"listxattr", "llistxattr", "flistxattr", "removexattr"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"lremovexattr", "fremovexattr", "tkill", "time"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"futex", "sched_setaffinity", "sched_getaffinity"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"set_thread_area", "io_setup", "io_destroy", "io_getevents"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"io_submit", "io_cancel", "get_thread_area", "epoll_create"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"remap_file_pages", "getdents64", "set_tid_address"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"restart_syscall", "semtimedop", "fadvise64", "timer_create"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"timer_settime", "timer_gettime", "timer_getoverrun"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"timer_delete", "clock_settime", "clock_gettime"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"clock_getres", "clock_nanosleep", "exit_group", "epoll_wait"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"epoll_ctl", "tgkill", "utimes", "mbind"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"set_mempolicy", "get_mempolicy", "mq_open", "mq_unlink"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"mq_timedsend", "mq_timedreceive", "mq_notify"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"mq_getsetattr", "waitid", "add_key", "request_key"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"keyctl", "ioprio_set", "ioprio_get", "inotify_init"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"inotify_add_watch", "inotify_rm_watch", "migrate_pages"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"openat", "mkdirat", "mknodat", "fchownat", "futimesat"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"newfstatat", "unlinkat", "renameat", "linkat", "symlinkat"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"readlinkat", "fchmodat", "faccessat", "pselect6", "ppoll"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"unshare", "set_robust_list", "get_robust_list", "splice"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"tee", "sync_file_range", "vmsplice", "move_pages"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"utimensat", "epoll_pwait", "signalfd", "timerfd_create"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"eventfd", "fallocate", "timerfd_settime", "timerfd_gettime"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"accept4", "signalfd4", "eventfd2", "epoll_create1"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"dup3", "pipe2", "inotify_init1", "preadv", "pwritev"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"rt_tgsigqueueinfo", "perf_event_open", "recvmmsg"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"fanotify_init", "fanotify_mark", "prlimit64"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"name_to_handle_at", "open_by_handle_at", "clock_adjtime"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"syncfs", "sendmmsg", "setns", "getcpu"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"process_vm_readv", "process_vm_writev", "kcmp"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"finit_module", "sched_setattr", "sched_getattr"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"renameat2", "seccomp", "getrandom", "memfd_create"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"bpf", "execveat", "userfaultfd", "membarrier"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"mlock2", "copy_file_range", "preadv2", "pwritev2"}, Action: "SCMP_ACT_ALLOW"},
			{Names: []string{"statx", "io_pgetevents", "rseq"}, Action: "SCMP_ACT_ALLOW"},
			// Block dangerous syscalls explicitly
			{Names: []string{"ptrace"}, Action: "SCMP_ACT_ERRNO", Args: []SeccompArg{{Index: 0, Value: 0, Op: "SCMP_CMP_NE"}}},
			{Names: []string{"mount", "umount2"}, Action: "SCMP_ACT_ERRNO"},
			{Names: []string{"reboot", "swapon", "swapoff"}, Action: "SCMP_ACT_ERRNO"},
			{Names: []string{"kexec_load", "kexec_file_load"}, Action: "SCMP_ACT_ERRNO"},
			{Names: []string{"acct"}, Action: "SCMP_ACT_ERRNO"},
		},
	}

	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// SeccompProfile represents the seccomp security profile
type SeccompProfile struct {
	DefaultAction string           `json:"defaultAction"`
	Architectures []string         `json:"architectures"`
	Syscalls      []SeccompSyscall `json:"syscalls"`
}

// SeccompSyscall defines allowed/blocked syscalls
type SeccompSyscall struct {
	Names  []string     `json:"names"`
	Action string       `json:"action"`
	Args   []SeccompArg `json:"args,omitempty"`
}

// SeccompArg for conditional syscall filtering
type SeccompArg struct {
	Index uint   `json:"index"`
	Value uint64 `json:"value"`
	Op    string `json:"op"`
}

// ensureImages ensures all sandbox images are available
func (s *ContainerSandbox) ensureImages() error {
	languages := []string{"python", "javascript", "go", "rust", "java", "c", "cpp"}

	for _, lang := range languages {
		imageName := fmt.Sprintf("%s-%s:latest", s.config.ImagePrefix, lang)

		// Check if image exists
		cmd := osexec.Command("docker", "image", "inspect", imageName)
		if cmd.Run() == nil {
			s.imageCacheMu.Lock()
			s.imageCache[lang] = true
			s.imageCacheMu.Unlock()
			continue
		}

		// Build the image
		dockerfile := s.generateDockerfile(lang)
		if err := s.buildImage(lang, dockerfile); err != nil {
			// Log warning but continue - will use fallback base images
			fmt.Printf("Warning: could not build sandbox image for %s: %v\n", lang, err)
		} else {
			s.imageCacheMu.Lock()
			s.imageCache[lang] = true
			s.imageCacheMu.Unlock()
		}
	}

	return nil
}

// generateDockerfile creates a minimal, secure Dockerfile for a language
func (s *ContainerSandbox) generateDockerfile(language string) string {
	switch language {
	case "python":
		return `FROM python:3.12-slim-bookworm
RUN useradd -m -s /bin/false sandbox && \
    apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p /work /tmp && \
    chown -R sandbox:sandbox /work /tmp
USER sandbox
WORKDIR /work
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1
`
	case "javascript":
		return `FROM node:20-slim
RUN useradd -m -s /bin/false sandbox && \
    mkdir -p /work /tmp && \
    chown -R sandbox:sandbox /work /tmp
USER sandbox
WORKDIR /work
ENV NODE_ENV=production
`
	case "go":
		return `FROM golang:1.22-bookworm
RUN useradd -m -s /bin/false sandbox && \
    mkdir -p /work /tmp /tmp/go-cache /tmp/go-mod && \
    chown -R sandbox:sandbox /work /tmp /go
USER sandbox
WORKDIR /work
ENV GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod TMPDIR=/tmp CGO_ENABLED=0
`
	case "rust":
		return `FROM rust:1.75-slim-bookworm
RUN useradd -m -s /bin/false sandbox && \
    mkdir -p /work /tmp && \
    chown -R sandbox:sandbox /work /tmp
USER sandbox
WORKDIR /work
ENV CARGO_HOME=/tmp/.cargo
`
	case "java":
		return `FROM eclipse-temurin:21-jdk-jammy
RUN useradd -m -s /bin/false sandbox && \
    mkdir -p /work /tmp && \
    chown -R sandbox:sandbox /work /tmp
USER sandbox
WORKDIR /work
`
	case "c", "cpp":
		return `FROM gcc:13-bookworm
RUN useradd -m -s /bin/false sandbox && \
    mkdir -p /work /tmp && \
    chown -R sandbox:sandbox /work /tmp
USER sandbox
WORKDIR /work
`
	default:
		return `FROM debian:bookworm-slim
RUN useradd -m -s /bin/false sandbox && \
    mkdir -p /work /tmp && \
    chown -R sandbox:sandbox /work /tmp
USER sandbox
WORKDIR /work
`
	}
}

// buildImage builds a Docker image from a Dockerfile string
func (s *ContainerSandbox) buildImage(language, dockerfile string) error {
	imageName := fmt.Sprintf("%s-%s:latest", s.config.ImagePrefix, language)

	// Create temp directory for Dockerfile
	tmpDir, err := os.MkdirTemp("", "apex-dockerfile-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// Write Dockerfile
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0644); err != nil {
		return err
	}

	// Build image
	cmd := osexec.Command("docker", "build", "-t", imageName, "-f", dockerfilePath, tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker build failed: %s", string(output))
	}

	return nil
}

// Execute runs code in an isolated container
func (s *ContainerSandbox) Execute(ctx context.Context, language, code, stdin string) (*ExecutionResult, error) {
	execID := uuid.New().String()
	startTime := time.Now()

	// Check concurrent execution limit
	current := atomic.AddInt32(&s.stats.ConcurrentExecs, 1)
	defer atomic.AddInt32(&s.stats.ConcurrentExecs, -1)

	if current > s.config.MaxConcurrentExecs {
		atomic.AddInt64(&s.stats.FailedExecs, 1)
		return &ExecutionResult{
			ID:          execID,
			Status:      "failed",
			ErrorOutput: "Too many concurrent executions. Please try again later.",
			ExitCode:    1,
			Language:    language,
			StartedAt:   startTime,
		}, nil
	}

	// Update max concurrent
	for {
		max := atomic.LoadInt32(&s.stats.MaxConcurrentExecs)
		if current <= max || atomic.CompareAndSwapInt32(&s.stats.MaxConcurrentExecs, max, current) {
			break
		}
	}

	// Get resource limits for language
	limits := s.getResourceLimits(language)

	// Create temp directory for code
	tempDir, err := os.MkdirTemp(s.baseTempDir, fmt.Sprintf("exec-%s-", execID[:8]))
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, limits.Timeout)

	// Track execution
	exec := &containerExecution{
		ID:        execID,
		Language:  language,
		StartTime: startTime,
		TempDir:   tempDir,
		Cancel:    cancel,
		Done:      make(chan struct{}),
	}

	s.executionsMu.Lock()
	s.executions[execID] = exec
	s.executionsMu.Unlock()

	defer func() {
		cancel()
		close(exec.Done)
		s.executionsMu.Lock()
		delete(s.executions, execID)
		s.executionsMu.Unlock()
		// Cleanup temp directory
		go s.cleanupTempDir(tempDir)
	}()

	// Write code to temp file
	filename, err := s.writeCodeFile(tempDir, language, code)
	if err != nil {
		return &ExecutionResult{
			ID:          execID,
			Status:      "failed",
			ErrorOutput: fmt.Sprintf("Failed to write code: %v", err),
			ExitCode:    1,
			Language:    language,
			StartedAt:   startTime,
		}, nil
	}

	// Build and run container
	result := s.runContainer(execCtx, exec, filename, stdin, limits)

	// Log execution
	s.logExecution(exec, result)

	// Update stats
	atomic.AddInt64(&s.stats.TotalExecutions, 1)
	switch result.Status {
	case "completed":
		atomic.AddInt64(&s.stats.SuccessfulExecs, 1)
	case "timeout":
		atomic.AddInt64(&s.stats.TimeoutExecs, 1)
	case "killed":
		atomic.AddInt64(&s.stats.KilledExecs, 1)
	default:
		atomic.AddInt64(&s.stats.FailedExecs, 1)
	}

	return result, nil
}

// getResourceLimits returns the resource limits for a language
func (s *ContainerSandbox) getResourceLimits(language string) *LanguageResourceLimits {
	if limits, ok := s.config.LanguageLimits[language]; ok {
		return limits
	}

	return &LanguageResourceLimits{
		MemoryLimit: s.config.DefaultMemoryLimit,
		CPULimit:    s.config.DefaultCPULimit,
		Timeout:     s.config.DefaultTimeout,
		PidsLimit:   s.config.DefaultPidsLimit,
		TmpfsSize:   s.config.TmpfsSize,
	}
}

// writeCodeFile writes code to the appropriate file for the language
func (s *ContainerSandbox) writeCodeFile(tempDir, language, code string) (string, error) {
	var filename string
	var processedCode string

	switch language {
	case "python":
		filename = "main.py"
		processedCode = code
	case "javascript":
		filename = "main.js"
		processedCode = code
	case "go":
		filename = "main.go"
		// Ensure package main
		if !strings.Contains(code, "package ") {
			processedCode = "package main\n\n" + code
		} else {
			processedCode = code
		}
	case "rust":
		filename = "main.rs"
		// Ensure main function
		if !strings.Contains(code, "fn main") {
			processedCode = "fn main() {\n" + code + "\n}"
		} else {
			processedCode = code
		}
	case "java":
		// Extract class name
		className := extractJavaClassNameFromCode(code)
		if className == "" {
			code = "public class Main {\n    public static void main(String[] args) {\n        " + code + "\n    }\n}"
			className = "Main"
		}
		filename = className + ".java"
		processedCode = code
	case "c":
		filename = "main.c"
		if !strings.Contains(code, "#include") {
			processedCode = "#include <stdio.h>\n#include <stdlib.h>\n#include <string.h>\n\n" + code
		} else {
			processedCode = code
		}
	case "cpp":
		filename = "main.cpp"
		if !strings.Contains(code, "#include") {
			processedCode = "#include <iostream>\n#include <vector>\n#include <string>\n#include <algorithm>\nusing namespace std;\n\n" + code
		} else {
			processedCode = code
		}
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}

	filePath := filepath.Join(tempDir, filename)
	if err := os.WriteFile(filePath, []byte(processedCode), 0644); err != nil {
		return "", err
	}

	return filename, nil
}

// extractJavaClassNameFromCode extracts public class name from Java code
func extractJavaClassNameFromCode(code string) string {
	re := regexp.MustCompile(`public\s+class\s+(\w+)`)
	matches := re.FindStringSubmatch(code)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// runContainer executes code in a Docker container
func (s *ContainerSandbox) runContainer(ctx context.Context, exec *containerExecution, filename, stdin string, limits *LanguageResourceLimits) *ExecutionResult {
	result := &ExecutionResult{
		ID:        exec.ID,
		Language:  exec.Language,
		StartedAt: exec.StartTime,
	}

	// Get image name
	imageName := s.getImageName(exec.Language)

	// Build docker run command
	args := s.buildDockerArgs(exec, filename, limits, imageName)
	if stdin != "" {
		// Required for piping stdin into `docker run`.
		args = append(args[:1], append([]string{"-i"}, args[1:]...)...)
	}

	// Create docker command
	cmd := osexec.CommandContext(ctx, "docker", args...)

	// Setup stdio
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &stdout, limit: 1024 * 1024} // 1MB limit
	cmd.Stderr = &limitedWriter{w: &stderr, limit: 1024 * 1024}

	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	// Run container
	err := cmd.Run()

	completedAt := time.Now()
	result.CompletedAt = &completedAt
	result.Duration = time.Since(exec.StartTime)
	result.DurationMs = result.Duration.Milliseconds()
	result.Output = stdout.String()
	result.ErrorOutput = stderr.String()

	// Check for context cancellation (timeout)
	if ctx.Err() == context.DeadlineExceeded {
		result.Status = "timeout"
		result.TimedOut = true
		result.ExitCode = 124

		// Force kill the container
		go s.forceKillContainer(exec.ContainerID)
	} else if ctx.Err() == context.Canceled {
		result.Status = "killed"
		result.Killed = true
		result.ExitCode = 137
	} else if err != nil {
		if exitErr, ok := err.(*osexec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Status = "failed"
		} else {
			result.Status = "failed"
			result.ExitCode = 1
			result.ErrorOutput = err.Error()
		}
	} else {
		result.Status = "completed"
		result.ExitCode = 0
	}

	return result
}

// buildDockerArgs constructs the docker run command arguments
func (s *ContainerSandbox) buildDockerArgs(exec *containerExecution, filename string, limits *LanguageResourceLimits, imageName string) []string {
	containerName := fmt.Sprintf("apex-sandbox-%s", exec.ID[:12])
	exec.ContainerID = containerName

	args := []string{
		"run",
		"--rm",
		"--name", containerName,
		// Resource limits
		"--memory", fmt.Sprintf("%d", limits.MemoryLimit),
		"--memory-swap", fmt.Sprintf("%d", limits.MemoryLimit), // Disable swap
		"--cpus", fmt.Sprintf("%.2f", limits.CPULimit),
		"--pids-limit", fmt.Sprintf("%d", limits.PidsLimit),
		// Security settings
		"--cap-drop=ALL",
		"--security-opt=no-new-privileges:true",
	}

	// Add seccomp profile
	if s.config.EnableSeccomp && s.seccompProfile != "" {
		args = append(args, "--security-opt", fmt.Sprintf("seccomp=%s", s.seccompProfile))
	}

	// Read-only root filesystem
	if s.config.EnableReadOnlyRoot {
		args = append(args, "--read-only")
	}

	// Tmpfs mounts
	tmpfsFlags := "rw,exec,nosuid,size=%s,mode=1777,uid=1000,gid=1000"
	if !s.languageNeedsExecutableTmp(exec.Language) {
		tmpfsFlags = "rw,noexec,nosuid,size=%s,mode=1777,uid=1000,gid=1000"
	}
	args = append(args,
		"--tmpfs", fmt.Sprintf("/tmp:"+tmpfsFlags, limits.TmpfsSize),
	)

	// Network isolation
	if s.config.DisableNetwork {
		args = append(args, "--network=none")
	} else if s.config.NetworkMode != "" {
		args = append(args, "--network="+s.config.NetworkMode)
	}

	// Mount code directory (read-only)
	args = append(args,
		"-v", fmt.Sprintf("%s:/work:ro", exec.TempDir),
	)

	// Shared package caches for faster warm starts (Replit-parity behavior)
	if s.pkgCache != nil && s.pkgCache.Enabled() {
		for _, cacheMount := range s.pkgCache.MountsForLanguage(exec.Language) {
			mode := "rw"
			if cacheMount.ReadOnly {
				mode = "ro"
			}
			args = append(args, "-v", fmt.Sprintf("%s:%s:%s", cacheMount.HostPath, cacheMount.ContainerPath, mode))
			for k, v := range cacheMount.EnvironmentMap {
				args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
			}
		}
	}

	// Set user
	args = append(args, "--user", "sandbox")

	// Working directory
	args = append(args, "-w", "/work")

	// Add image
	args = append(args, imageName)

	// Add execution command
	execCmd := s.getExecutionCommand(exec.Language, filename)
	args = append(args, execCmd...)

	return args
}

// getImageName returns the Docker image name for a language
func (s *ContainerSandbox) getImageName(language string) string {
	s.imageCacheMu.RLock()
	exists := s.imageCache[language]
	s.imageCacheMu.RUnlock()

	if exists {
		return fmt.Sprintf("%s-%s:latest", s.config.ImagePrefix, language)
	}

	// Fallback to public images
	switch language {
	case "python":
		return "python:3.12-slim"
	case "javascript":
		return "node:20-slim"
	case "go":
		return "golang:1.22"
	case "rust":
		return "rust:1.75-slim"
	case "java":
		return "eclipse-temurin:21-jdk"
	case "c", "cpp":
		return "gcc:13"
	default:
		return "debian:bookworm-slim"
	}
}

// getExecutionCommand returns the command to execute code in the container
func (s *ContainerSandbox) getExecutionCommand(language, filename string) []string {
	switch language {
	case "python":
		return []string{"python3", "-u", filename}
	case "javascript":
		// `--jitless` avoids executable-memory permission issues in hardened container runtimes.
		return []string{"node", "--jitless", filename}
	case "go":
		return []string{"sh", "-c", fmt.Sprintf("go run %s", filename)}
	case "rust":
		return []string{"sh", "-c", fmt.Sprintf("rustc -o /tmp/main %s && /tmp/main", filename)}
	case "java":
		className := strings.TrimSuffix(filename, ".java")
		return []string{"sh", "-c", fmt.Sprintf("javac -d /tmp %s && java -cp /tmp %s", filename, className)}
	case "c":
		return []string{"sh", "-c", fmt.Sprintf("gcc -o /tmp/main %s -lm && /tmp/main", filename)}
	case "cpp":
		return []string{"sh", "-c", fmt.Sprintf("g++ -o /tmp/main -std=c++17 %s && /tmp/main", filename)}
	default:
		return []string{"sh", "-c", "echo 'Unsupported language'"}
	}
}

// languageNeedsExecutableTmp returns true when /tmp must allow executing compiled artifacts.
func (s *ContainerSandbox) languageNeedsExecutableTmp(language string) bool {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "go", "rust", "c", "cpp", "java":
		return true
	default:
		return false
	}
}

// forceKillContainer forcefully removes a container
func (s *ContainerSandbox) forceKillContainer(containerID string) {
	if containerID == "" {
		return
	}

	// First try graceful stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stopCmd := osexec.CommandContext(ctx, "docker", "stop", "-t", "2", containerID)
	stopCmd.Run()

	// Then force remove
	rmCmd := osexec.Command("docker", "rm", "-f", containerID)
	rmCmd.Run()
}

// Kill terminates a running execution
func (s *ContainerSandbox) Kill(execID string) error {
	s.executionsMu.RLock()
	exec, exists := s.executions[execID]
	s.executionsMu.RUnlock()

	if !exists {
		return fmt.Errorf("execution %s not found", execID)
	}

	// Cancel context
	exec.Cancel()

	// Force kill container
	if exec.ContainerID != "" {
		s.forceKillContainer(exec.ContainerID)
	}

	return nil
}

// cleanupTempDir removes a temporary directory
func (s *ContainerSandbox) cleanupTempDir(tempDir string) {
	if tempDir == "" || !strings.HasPrefix(tempDir, s.baseTempDir) {
		return
	}

	// Small delay to ensure container has released files
	time.Sleep(500 * time.Millisecond)
	os.RemoveAll(tempDir)
}

// cleanupLoop periodically cleans up orphaned containers and temp directories
func (s *ContainerSandbox) cleanupLoop() {
	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanupOrphanedContainers()
		s.cleanupOldTempDirs()
	}
}

// cleanupOrphanedContainers removes any orphaned sandbox containers
func (s *ContainerSandbox) cleanupOrphanedContainers() {
	cmd := osexec.Command("docker", "ps", "-a", "--filter", "name=apex-sandbox-", "--format", "{{.Names}}\t{{.Status}}")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}

		containerName := parts[0]
		status := parts[1]

		// Remove exited or created containers
		if strings.Contains(status, "Exited") || strings.Contains(status, "Created") {
			osexec.Command("docker", "rm", "-f", containerName).Run()
		}
	}
}

// cleanupOldTempDirs removes temp directories older than MaxContainerAge
func (s *ContainerSandbox) cleanupOldTempDirs() {
	entries, err := os.ReadDir(s.baseTempDir)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-s.config.MaxContainerAge)

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "exec-") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			os.RemoveAll(filepath.Join(s.baseTempDir, entry.Name()))
		}
	}
}

// logExecution logs an execution to the audit log
func (s *ContainerSandbox) logExecution(exec *containerExecution, result *ExecutionResult) {
	if s.auditLogger == nil {
		return
	}

	entry := AuditEntry{
		Timestamp:   time.Now(),
		ExecutionID: exec.ID,
		ContainerID: exec.ContainerID,
		Language:    exec.Language,
		Action:      result.Status,
		Duration:    result.DurationMs,
		ExitCode:    result.ExitCode,
		MemoryUsed:  result.MemoryUsed,
	}

	if result.Status == "failed" && result.ErrorOutput != "" {
		// Truncate error output for logging
		errOutput := result.ErrorOutput
		if len(errOutput) > 500 {
			errOutput = errOutput[:500] + "..."
		}
		entry.Error = errOutput
	}

	s.auditLogger.Log(entry)
}

// GetStats returns sandbox statistics
func (s *ContainerSandbox) GetStats() *SandboxStats {
	return &SandboxStats{
		TotalExecutions:    atomic.LoadInt64(&s.stats.TotalExecutions),
		SuccessfulExecs:    atomic.LoadInt64(&s.stats.SuccessfulExecs),
		FailedExecs:        atomic.LoadInt64(&s.stats.FailedExecs),
		TimeoutExecs:       atomic.LoadInt64(&s.stats.TimeoutExecs),
		KilledExecs:        atomic.LoadInt64(&s.stats.KilledExecs),
		ConcurrentExecs:    atomic.LoadInt32(&s.stats.ConcurrentExecs),
		MaxConcurrentExecs: atomic.LoadInt32(&s.stats.MaxConcurrentExecs),
	}
}

// GetActiveExecutions returns the count of active executions
func (s *ContainerSandbox) GetActiveExecutions() int {
	s.executionsMu.RLock()
	defer s.executionsMu.RUnlock()
	return len(s.executions)
}

// Cleanup cleans up all sandbox resources
func (s *ContainerSandbox) Cleanup() error {
	// Kill all active executions
	s.executionsMu.Lock()
	for id := range s.executions {
		s.Kill(id)
	}
	s.executionsMu.Unlock()

	// Cleanup orphaned containers
	s.cleanupOrphanedContainers()

	// Remove base temp directory
	if err := os.RemoveAll(s.baseTempDir); err != nil {
		return err
	}

	// Close audit logger
	if s.auditLogger != nil {
		s.auditLogger.Close()
	}

	return nil
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(path string) (*AuditLogger, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return nil, err
	}

	return &AuditLogger{
		logPath: path,
		file:    file,
	}, nil
}

// Log writes an audit entry
func (l *AuditLogger) Log(entry AuditEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	l.file.Write(data)
	l.file.WriteString("\n")
}

// Close closes the audit logger
func (l *AuditLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// CommandContext creates an exec.Cmd with context
func (e *containerExecution) CommandContext(ctx context.Context, name string, args ...string) *osexec.Cmd {
	return osexec.CommandContext(ctx, name, args...)
}
