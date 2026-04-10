package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"apex-build/internal/deploy"
)

type cliRunner func(ctx context.Context, dir string, env []string, name string, args ...string) (string, error)
type binaryLookup func(file string) (string, error)

var urlPattern = regexp.MustCompile(`https?://[^\s]+`)

func defaultCLIRunner(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	errOut := strings.TrimSpace(stderr.String())
	combined := strings.TrimSpace(strings.Join([]string{out, errOut}, "\n"))
	if err != nil {
		if combined == "" {
			return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return combined, fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}

	return combined, nil
}

func BinaryAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func writeProjectWorkspace(files []deploy.ProjectFile) (string, error) {
	dir, err := os.MkdirTemp("", "apex-deploy-*")
	if err != nil {
		return "", err
	}

	for _, file := range files {
		if file.IsDir {
			continue
		}
		relPath, err := safeRelativePath(file.Path)
		if err != nil {
			os.RemoveAll(dir)
			return "", err
		}
		target := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			os.RemoveAll(dir)
			return "", err
		}
		if err := os.WriteFile(target, []byte(file.Content), 0o644); err != nil {
			os.RemoveAll(dir)
			return "", err
		}
	}

	return dir, nil
}

func safeRelativePath(path string) (string, error) {
	cleaned := filepath.Clean("/" + strings.TrimSpace(strings.TrimPrefix(path, "/")))
	rel := strings.TrimPrefix(cleaned, "/")
	if rel == "" || rel == "." {
		return "", fmt.Errorf("invalid empty path")
	}
	return rel, nil
}

func resolveWorkspaceDir(baseDir, subdir string) (string, error) {
	if strings.TrimSpace(subdir) == "" {
		return baseDir, nil
	}
	relPath, err := safeRelativePath(subdir)
	if err != nil {
		return "", err
	}
	target := filepath.Join(baseDir, relPath)
	info, err := os.Stat(target)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", subdir)
	}
	return target, nil
}

func splitLogLines(output string) []string {
	if strings.TrimSpace(output) == "" {
		return nil
	}
	lines := strings.Split(output, "\n")
	logs := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		logs = append(logs, line)
	}
	return logs
}

func extractURL(output string, contains string) string {
	matches := urlPattern.FindAllString(output, -1)
	for _, match := range matches {
		if contains == "" || strings.Contains(match, contains) {
			return match
		}
	}
	return ""
}

func sanitizeDeploymentName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "apex-app"
	}

	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	name := strings.Trim(b.String(), "-")
	if name == "" {
		return "apex-app"
	}
	if len(name) > 48 {
		name = strings.Trim(name[:48], "-")
	}
	if name == "" {
		return "apex-app"
	}
	return name
}

func marshalProviderRef(ref map[string]string) string {
	if len(ref) == 0 {
		return ""
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return ""
	}
	return string(data)
}

func unmarshalProviderRef(value string) map[string]string {
	if strings.TrimSpace(value) == "" {
		return map[string]string{}
	}
	var ref map[string]string
	if err := json.Unmarshal([]byte(value), &ref); err == nil && ref != nil {
		return ref
	}
	return map[string]string{}
}

func decodeLooseJSON(output string) any {
	var data any
	if err := json.Unmarshal([]byte(output), &data); err == nil {
		return data
	}
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if err := json.Unmarshal([]byte(line), &data); err == nil {
			return data
		}
	}
	return nil
}

func findStringValue(data any, keys ...string) string {
	switch node := data.(type) {
	case map[string]any:
		for _, key := range keys {
			if value, ok := node[key]; ok {
				if str, ok := value.(string); ok && strings.TrimSpace(str) != "" {
					return str
				}
			}
		}
		for _, value := range node {
			if found := findStringValue(value, keys...); found != "" {
				return found
			}
		}
	case []any:
		for _, item := range node {
			if found := findStringValue(item, keys...); found != "" {
				return found
			}
		}
	}
	return ""
}

func findMapValue(data any, keys ...string) map[string]any {
	switch node := data.(type) {
	case map[string]any:
		for _, key := range keys {
			if value, ok := node[key]; ok {
				if child, ok := value.(map[string]any); ok {
					return child
				}
			}
		}
		for _, value := range node {
			if found := findMapValue(value, keys...); found != nil {
				return found
			}
		}
	case []any:
		for _, item := range node {
			if found := findMapValue(item, keys...); found != nil {
				return found
			}
		}
	}
	return nil
}

func runShell(ctx context.Context, runner cliRunner, dir string, env []string, script string) (string, error) {
	return runner(ctx, dir, env, "sh", "-lc", script)
}
