package preview

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type dockerPreviewBackendRuntime struct {
	dockerHost    string
	dockerContext string
	connectHost   string
	imagePrefix   string
}

func newDockerPreviewBackendRuntimeFromEnv() (*dockerPreviewBackendRuntime, error) {
	dockerHost := strings.TrimSpace(os.Getenv("APEX_PREVIEW_DOCKER_HOST"))
	if dockerHost == "" {
		dockerHost = strings.TrimSpace(os.Getenv("APEX_PREVIEW_DOCKER_SOCKET"))
	}
	dockerContext := strings.TrimSpace(os.Getenv("APEX_PREVIEW_DOCKER_CONTEXT"))
	if dockerContext == "" {
		dockerContext = strings.TrimSpace(os.Getenv("DOCKER_CONTEXT"))
	}
	if dockerHost == "" {
		dockerHost = strings.TrimSpace(os.Getenv("DOCKER_HOST"))
	}
	connectHost := strings.TrimSpace(os.Getenv("APEX_PREVIEW_CONNECT_HOST"))
	if connectHost == "" {
		connectHost = derivePreviewConnectHost(dockerHost)
	}
	if connectHost == "" {
		connectHost = "localhost"
	}

	rt := &dockerPreviewBackendRuntime{
		dockerHost:    dockerHost,
		dockerContext: dockerContext,
		connectHost:   connectHost,
		imagePrefix:   firstNonEmpty(os.Getenv("APEX_PREVIEW_BACKEND_IMAGE_PREFIX"), "apex-backend-preview"),
	}
	if err := rt.checkDocker(); err != nil {
		return nil, err
	}
	rt.cleanupOrphanedPreviewContainers()
	return rt, nil
}

func (r *dockerPreviewBackendRuntime) Name() string { return "container" }

func (r *dockerPreviewBackendRuntime) RequiresLocalDependencyInstall() bool { return false }

func (r *dockerPreviewBackendRuntime) StartProcess(cfg *ProcessStartConfig) (*ProcessHandle, error) {
	if cfg == nil {
		return nil, fmt.Errorf("process config is required")
	}
	workDir := strings.TrimSpace(cfg.Dir)
	if workDir == "" {
		return nil, fmt.Errorf("working directory is required")
	}
	port := envPort(cfg.Env)
	if port == 0 {
		return nil, fmt.Errorf("PORT environment variable is required")
	}
	command := strings.TrimSpace(cfg.Command)
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}
	if _, err := os.Stat(workDir); err != nil {
		return nil, fmt.Errorf("working directory unavailable: %w", err)
	}

	imageName := fmt.Sprintf("%s-%d-%d:latest", r.imagePrefix, port, time.Now().UnixNano())
	containerName := fmt.Sprintf("%s-%d-%d", r.imagePrefix, port, time.Now().UnixNano())
	dockerfilePath, err := r.writeDockerfile(workDir)
	if err != nil {
		return nil, err
	}
	defer os.Remove(dockerfilePath)

	buildCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	build := r.dockerCommandContext(buildCtx, "build", "-t", imageName, "-f", dockerfilePath, workDir)
	if out, buildErr := build.CombinedOutput(); buildErr != nil {
		return nil, fmt.Errorf("docker backend image build failed: %w\n%s", buildErr, truncateInstallOutput(out))
	}

	runArgs := []string{
		"run", "-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%d:%d", port, port),
		"--memory", "512m",
		"--memory-swap", "512m",
		"--cpus", "0.75",
		"--pids-limit", "200",
		"--cap-drop=ALL",
		"--cap-add=NET_BIND_SERVICE",
		"--security-opt=no-new-privileges:true",
		"--restart=no",
		"--label", "apex.backend-preview=true",
		"--label", fmt.Sprintf("apex.preview-port=%d", port),
	}
	for _, env := range filteredPreviewBackendEnv(cfg.Env) {
		runArgs = append(runArgs, "-e", env)
	}
	runArgs = append(runArgs, imageName, command)
	runArgs = append(runArgs, cfg.Args...)

	runCtx, runCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer runCancel()
	run := r.dockerCommandContext(runCtx, runArgs...)
	out, err := run.CombinedOutput()
	if err != nil {
		_ = r.dockerCommand("rmi", "-f", imageName).Run()
		return nil, fmt.Errorf("docker backend container failed to start: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	containerID := strings.TrimSpace(string(out))
	if containerID == "" {
		_ = r.dockerCommand("rm", "-f", containerName).Run()
		_ = r.dockerCommand("rmi", "-f", imageName).Run()
		return nil, fmt.Errorf("docker backend container did not return an ID")
	}

	logCmd := r.dockerCommand("logs", "-f", containerID)
	stdout, err := logCmd.StdoutPipe()
	if err != nil {
		_ = r.stopContainer(containerID, imageName)
		return nil, fmt.Errorf("docker logs stdout: %w", err)
	}
	stderr, err := logCmd.StderrPipe()
	if err != nil {
		_ = r.stopContainer(containerID, imageName)
		return nil, fmt.Errorf("docker logs stderr: %w", err)
	}
	if err := logCmd.Start(); err != nil {
		_ = r.stopContainer(containerID, imageName)
		return nil, fmt.Errorf("docker logs start: %w", err)
	}

	readyURL := r.readyURL(port)
	return &ProcessHandle{
		Pid:        0,
		StdoutPipe: stdout,
		StderrPipe: stderr,
		ReadyURL:   readyURL,
		Wait: func() (int, error) {
			waitCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			waitOut, waitErr := r.dockerCommandContext(waitCtx, "wait", containerID).CombinedOutput()
			_ = logCmd.Process.Kill()
			_ = logCmd.Wait()
			exitCode := 1
			if parsed, parseErr := strconv.Atoi(strings.TrimSpace(string(waitOut))); parseErr == nil {
				exitCode = parsed
			}
			if waitErr != nil {
				return exitCode, waitErr
			}
			if exitCode != 0 {
				return exitCode, fmt.Errorf("container exited with code %d", exitCode)
			}
			return 0, nil
		},
		SignalStop: func() {
			_ = r.dockerCommand("stop", "-t", "5", containerID).Run()
			_ = r.dockerCommand("rm", "-f", containerID).Run()
			_ = r.dockerCommand("rmi", "-f", imageName).Run()
			_ = logCmd.Process.Kill()
		},
		ForceKill: func() {
			_ = r.stopContainer(containerID, imageName)
			_ = logCmd.Process.Kill()
		},
	}, nil
}

func (r *dockerPreviewBackendRuntime) checkDocker() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker CLI not found: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if out, err := r.dockerCommandContext(ctx, "info", "--format", "{{.ServerVersion}}").CombinedOutput(); err != nil {
		return fmt.Errorf("docker backend runtime unavailable: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (r *dockerPreviewBackendRuntime) cleanupOrphanedPreviewContainers() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := r.dockerCommandContext(
		ctx,
		"ps",
		"-aq",
		"--filter", "label=apex.backend-preview=true",
	).CombinedOutput()
	if err != nil {
		return
	}
	ids := strings.Fields(string(out))
	if len(ids) == 0 {
		return
	}
	args := append([]string{"rm", "-f"}, ids...)
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cleanupCancel()
	_ = r.dockerCommandContext(cleanupCtx, args...).Run()
}

func (r *dockerPreviewBackendRuntime) IsPortAvailable(port int) bool {
	if port <= 0 {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := r.dockerCommandContext(
		ctx,
		"ps",
		"--filter", fmt.Sprintf("label=apex.preview-port=%d", port),
		"--format", "{{.ID}}",
	).CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) != "" {
		return false
	}

	host := r.previewConnectHostname()
	if host == "" {
		return true
	}
	conn, dialErr := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 200*time.Millisecond)
	if dialErr == nil {
		_ = conn.Close()
		return false
	}
	return true
}

func (r *dockerPreviewBackendRuntime) previewConnectHostname() string {
	host := strings.TrimSpace(r.connectHost)
	if host == "" {
		return ""
	}
	if parsed, err := url.Parse(host); err == nil && parsed.Scheme != "" {
		host = parsed.Hostname()
	}
	return strings.TrimSpace(host)
}

func (r *dockerPreviewBackendRuntime) dockerEnv() []string {
	env := append([]string(nil), os.Environ()...)
	if r.dockerContext != "" && envValue(env, "DOCKER_CONTEXT") == "" {
		env = append(env, "DOCKER_CONTEXT="+r.dockerContext)
	}
	if r.dockerHost != "" {
		host := r.dockerHost
		if !strings.Contains(host, "://") {
			host = "unix://" + host
		}
		env = append(env, "DOCKER_HOST="+host)
	}
	return env
}

func (r *dockerPreviewBackendRuntime) dockerCommand(args ...string) *exec.Cmd {
	cmd := exec.Command("docker", args...)
	cmd.Env = r.dockerEnv()
	return cmd
}

func (r *dockerPreviewBackendRuntime) dockerCommandContext(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = r.dockerEnv()
	return cmd
}

func (r *dockerPreviewBackendRuntime) writeDockerfile(workDir string) (string, error) {
	path := filepath.Join(workDir, ".apex-backend-preview.Dockerfile")
	content := `FROM node:20-slim
WORKDIR /app
COPY . .
RUN if [ -f package.json ]; then npm install --include=dev --no-audit --no-fund --loglevel=error; fi
RUN if [ -f package.json ] && node -e "const s=require('./package.json').scripts||{}; process.exit(s['build:server']?0:(s.build?0:1))"; then \
      if node -e "const s=require('./package.json').scripts||{}; process.exit(s['build:server']?0:1)"; then npm run build:server; else npm run build; fi; \
    fi
EXPOSE 9100
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write docker backend preview Dockerfile: %w", err)
	}
	return path, nil
}

func (r *dockerPreviewBackendRuntime) readyURL(port int) string {
	host := r.previewConnectHostname()
	if host == "" {
		host = "localhost"
	}
	return "http://" + host + ":" + strconv.Itoa(port)
}

func (r *dockerPreviewBackendRuntime) stopContainer(containerID, imageName string) error {
	_ = r.dockerCommand("rm", "-f", containerID).Run()
	if strings.TrimSpace(imageName) != "" {
		_ = r.dockerCommand("rmi", "-f", imageName).Run()
	}
	return nil
}

func envPort(env []string) int {
	last := 0
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, "PORT="); ok {
			port, _ := strconv.Atoi(strings.TrimSpace(value))
			if port > 0 {
				last = port
			}
		}
	}
	return last
}

func filteredPreviewBackendEnv(env []string) []string {
	allowed := map[string]bool{
		"PORT":      true,
		"HOST":      true,
		"NODE_ENV":  true,
		"FLASK_ENV": true,
		"DEBUG":     true,
	}
	order := make([]string, 0, len(allowed))
	selected := make(map[string]string, len(allowed))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		key = strings.TrimSpace(key)
		include := allowed[key] || strings.HasPrefix(key, "VITE_") || strings.HasPrefix(key, "PUBLIC_")
		if !include {
			if hostValue, exists := os.LookupEnv(key); !exists || hostValue != value {
				include = true
			}
		}
		if include {
			if _, exists := selected[key]; !exists {
				order = append(order, key)
			}
			selected[key] = value
		}
	}
	out := make([]string, 0, len(order))
	for _, key := range order {
		out = append(out, key+"="+selected[key])
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
