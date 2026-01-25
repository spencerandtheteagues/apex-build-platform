// APEX.BUILD Language Runners
// Individual language execution handlers

package execution

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Runner interface for language-specific execution
type Runner interface {
	// Language returns the language identifier
	Language() string

	// Extensions returns supported file extensions
	Extensions() []string

	// WriteCode writes code to a temp file and returns the filename
	WriteCode(tempDir, code string) (string, error)

	// BuildCommand builds the execution command for code in tempDir
	BuildCommand(tempDir, filename string) (*exec.Cmd, error)

	// BuildCommandForFile builds the execution command for an existing file
	BuildCommandForFile(filepath, tempDir string, args []string) (*exec.Cmd, error)

	// NeedsCompilation returns whether the language needs compilation
	NeedsCompilation() bool

	// Compile compiles the code if needed and returns the binary path
	Compile(tempDir, filename string) (string, error)
}

// runners maps language IDs to their runners
var runners = make(map[string]Runner)

// RegisterRunner registers a language runner
func RegisterRunner(r Runner) {
	runners[r.Language()] = r
	for _, ext := range r.Extensions() {
		runners[ext] = r
	}
}

// GetRunner returns the runner for a language
func GetRunner(language string) (Runner, error) {
	// Normalize language name
	language = strings.ToLower(strings.TrimSpace(language))

	// Check for direct match
	if r, ok := runners[language]; ok {
		return r, nil
	}

	// Check aliases
	aliases := map[string]string{
		"js":         "javascript",
		"node":       "javascript",
		"nodejs":     "javascript",
		"ts":         "typescript",
		"py":         "python",
		"python3":    "python",
		"golang":     "go",
		"rs":         "rust",
		"c++":        "cpp",
		"cplusplus":  "cpp",
		"rb":         "ruby",
	}

	if alias, ok := aliases[language]; ok {
		if r, ok := runners[alias]; ok {
			return r, nil
		}
	}

	return nil, fmt.Errorf("unsupported language: %s", language)
}

func init() {
	// Register all runners
	RegisterRunner(&NodeRunner{})
	RegisterRunner(&TypeScriptRunner{})
	RegisterRunner(&PythonRunner{})
	RegisterRunner(&GoRunner{})
	RegisterRunner(&RustRunner{})
	RegisterRunner(&CRunner{})
	RegisterRunner(&CppRunner{})
	RegisterRunner(&JavaRunner{})
	RegisterRunner(&RubyRunner{})
	RegisterRunner(&PHPRunner{})
}

// =============================================================================
// NodeRunner - JavaScript/Node.js execution
// =============================================================================

type NodeRunner struct{}

func (r *NodeRunner) Language() string {
	return "javascript"
}

func (r *NodeRunner) Extensions() []string {
	return []string{".js", ".mjs"}
}

func (r *NodeRunner) WriteCode(tempDir, code string) (string, error) {
	filename := "main.js"
	filepath := filepath.Join(tempDir, filename)
	if err := os.WriteFile(filepath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write JavaScript file: %w", err)
	}
	return filename, nil
}

func (r *NodeRunner) BuildCommand(tempDir, filename string) (*exec.Cmd, error) {
	// Check if node is available
	nodePath, err := exec.LookPath("node")
	if err != nil {
		return nil, fmt.Errorf("node not found: %w", err)
	}

	cmd := exec.Command(nodePath, filepath.Join(tempDir, filename))
	return cmd, nil
}

func (r *NodeRunner) BuildCommandForFile(filePath, tempDir string, args []string) (*exec.Cmd, error) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		return nil, fmt.Errorf("node not found: %w", err)
	}

	cmdArgs := append([]string{filePath}, args...)
	cmd := exec.Command(nodePath, cmdArgs...)
	return cmd, nil
}

func (r *NodeRunner) NeedsCompilation() bool {
	return false
}

func (r *NodeRunner) Compile(tempDir, filename string) (string, error) {
	return filepath.Join(tempDir, filename), nil
}

// =============================================================================
// TypeScriptRunner - TypeScript execution with ts-node
// =============================================================================

type TypeScriptRunner struct{}

func (r *TypeScriptRunner) Language() string {
	return "typescript"
}

func (r *TypeScriptRunner) Extensions() []string {
	return []string{".ts"}
}

func (r *TypeScriptRunner) WriteCode(tempDir, code string) (string, error) {
	filename := "main.ts"
	filepath := filepath.Join(tempDir, filename)
	if err := os.WriteFile(filepath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write TypeScript file: %w", err)
	}
	return filename, nil
}

func (r *TypeScriptRunner) BuildCommand(tempDir, filename string) (*exec.Cmd, error) {
	// Try ts-node first, fall back to npx ts-node
	tsNodePath, err := exec.LookPath("ts-node")
	if err != nil {
		// Try npx
		npxPath, err := exec.LookPath("npx")
		if err != nil {
			return nil, fmt.Errorf("ts-node not found: install with 'npm install -g ts-node typescript'")
		}
		cmd := exec.Command(npxPath, "ts-node", "--transpile-only", filepath.Join(tempDir, filename))
		return cmd, nil
	}

	cmd := exec.Command(tsNodePath, "--transpile-only", filepath.Join(tempDir, filename))
	return cmd, nil
}

func (r *TypeScriptRunner) BuildCommandForFile(filePath, tempDir string, args []string) (*exec.Cmd, error) {
	tsNodePath, err := exec.LookPath("ts-node")
	if err != nil {
		npxPath, err := exec.LookPath("npx")
		if err != nil {
			return nil, fmt.Errorf("ts-node not found")
		}
		cmdArgs := append([]string{"ts-node", "--transpile-only", filePath}, args...)
		cmd := exec.Command(npxPath, cmdArgs...)
		return cmd, nil
	}

	cmdArgs := append([]string{"--transpile-only", filePath}, args...)
	cmd := exec.Command(tsNodePath, cmdArgs...)
	return cmd, nil
}

func (r *TypeScriptRunner) NeedsCompilation() bool {
	return false // ts-node handles compilation transparently
}

func (r *TypeScriptRunner) Compile(tempDir, filename string) (string, error) {
	return filepath.Join(tempDir, filename), nil
}

// =============================================================================
// PythonRunner - Python 3 execution
// =============================================================================

type PythonRunner struct{}

func (r *PythonRunner) Language() string {
	return "python"
}

func (r *PythonRunner) Extensions() []string {
	return []string{".py"}
}

func (r *PythonRunner) WriteCode(tempDir, code string) (string, error) {
	filename := "main.py"
	filepath := filepath.Join(tempDir, filename)
	if err := os.WriteFile(filepath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write Python file: %w", err)
	}
	return filename, nil
}

func (r *PythonRunner) BuildCommand(tempDir, filename string) (*exec.Cmd, error) {
	// Try python3 first, then python
	pythonPath, err := exec.LookPath("python3")
	if err != nil {
		pythonPath, err = exec.LookPath("python")
		if err != nil {
			return nil, fmt.Errorf("python not found")
		}
	}

	cmd := exec.Command(pythonPath, "-u", filepath.Join(tempDir, filename))
	return cmd, nil
}

func (r *PythonRunner) BuildCommandForFile(filePath, tempDir string, args []string) (*exec.Cmd, error) {
	pythonPath, err := exec.LookPath("python3")
	if err != nil {
		pythonPath, err = exec.LookPath("python")
		if err != nil {
			return nil, fmt.Errorf("python not found")
		}
	}

	cmdArgs := append([]string{"-u", filePath}, args...)
	cmd := exec.Command(pythonPath, cmdArgs...)
	return cmd, nil
}

func (r *PythonRunner) NeedsCompilation() bool {
	return false
}

func (r *PythonRunner) Compile(tempDir, filename string) (string, error) {
	return filepath.Join(tempDir, filename), nil
}

// =============================================================================
// GoRunner - Go execution
// =============================================================================

type GoRunner struct{}

func (r *GoRunner) Language() string {
	return "go"
}

func (r *GoRunner) Extensions() []string {
	return []string{".go"}
}

func (r *GoRunner) WriteCode(tempDir, code string) (string, error) {
	filename := "main.go"
	filepath := filepath.Join(tempDir, filename)

	// Ensure code has package main if not present
	if !strings.Contains(code, "package ") {
		code = "package main\n\n" + code
	}

	if err := os.WriteFile(filepath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write Go file: %w", err)
	}
	return filename, nil
}

func (r *GoRunner) BuildCommand(tempDir, filename string) (*exec.Cmd, error) {
	goPath, err := exec.LookPath("go")
	if err != nil {
		return nil, fmt.Errorf("go not found")
	}

	// Use go run for simplicity
	cmd := exec.Command(goPath, "run", filepath.Join(tempDir, filename))
	return cmd, nil
}

func (r *GoRunner) BuildCommandForFile(filePath, tempDir string, args []string) (*exec.Cmd, error) {
	goPath, err := exec.LookPath("go")
	if err != nil {
		return nil, fmt.Errorf("go not found")
	}

	cmdArgs := append([]string{"run", filePath}, args...)
	cmd := exec.Command(goPath, cmdArgs...)
	return cmd, nil
}

func (r *GoRunner) NeedsCompilation() bool {
	return true
}

func (r *GoRunner) Compile(tempDir, filename string) (string, error) {
	goPath, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("go not found")
	}

	srcPath := filepath.Join(tempDir, filename)
	binPath := filepath.Join(tempDir, "main")

	cmd := exec.Command(goPath, "build", "-o", binPath, srcPath)
	cmd.Dir = tempDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("compilation failed: %s", string(output))
	}

	return binPath, nil
}

// =============================================================================
// RustRunner - Rust execution
// =============================================================================

type RustRunner struct{}

func (r *RustRunner) Language() string {
	return "rust"
}

func (r *RustRunner) Extensions() []string {
	return []string{".rs"}
}

func (r *RustRunner) WriteCode(tempDir, code string) (string, error) {
	filename := "main.rs"
	filepath := filepath.Join(tempDir, filename)

	// Ensure code has main function wrapper if it doesn't
	if !strings.Contains(code, "fn main") {
		code = "fn main() {\n" + code + "\n}"
	}

	if err := os.WriteFile(filepath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write Rust file: %w", err)
	}
	return filename, nil
}

func (r *RustRunner) BuildCommand(tempDir, filename string) (*exec.Cmd, error) {
	// Compile first
	binPath, err := r.Compile(tempDir, filename)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(binPath)
	return cmd, nil
}

func (r *RustRunner) BuildCommandForFile(filePath, tempDir string, args []string) (*exec.Cmd, error) {
	rustcPath, err := exec.LookPath("rustc")
	if err != nil {
		return nil, fmt.Errorf("rustc not found")
	}

	binPath := filepath.Join(tempDir, "main")

	// Compile
	compileCmd := exec.Command(rustcPath, "-o", binPath, filePath)
	output, err := compileCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("compilation failed: %s", string(output))
	}

	cmd := exec.Command(binPath, args...)
	return cmd, nil
}

func (r *RustRunner) NeedsCompilation() bool {
	return true
}

func (r *RustRunner) Compile(tempDir, filename string) (string, error) {
	rustcPath, err := exec.LookPath("rustc")
	if err != nil {
		return "", fmt.Errorf("rustc not found")
	}

	srcPath := filepath.Join(tempDir, filename)
	binPath := filepath.Join(tempDir, "main")

	cmd := exec.Command(rustcPath, "-o", binPath, srcPath)
	cmd.Dir = tempDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("compilation failed: %s", string(output))
	}

	return binPath, nil
}

// =============================================================================
// CRunner - C execution with GCC
// =============================================================================

type CRunner struct{}

func (r *CRunner) Language() string {
	return "c"
}

func (r *CRunner) Extensions() []string {
	return []string{".c"}
}

func (r *CRunner) WriteCode(tempDir, code string) (string, error) {
	filename := "main.c"
	filepath := filepath.Join(tempDir, filename)

	// Add common includes if not present
	if !strings.Contains(code, "#include") {
		code = "#include <stdio.h>\n#include <stdlib.h>\n#include <string.h>\n\n" + code
	}

	if err := os.WriteFile(filepath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write C file: %w", err)
	}
	return filename, nil
}

func (r *CRunner) BuildCommand(tempDir, filename string) (*exec.Cmd, error) {
	binPath, err := r.Compile(tempDir, filename)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(binPath)
	return cmd, nil
}

func (r *CRunner) BuildCommandForFile(filePath, tempDir string, args []string) (*exec.Cmd, error) {
	gccPath, err := exec.LookPath("gcc")
	if err != nil {
		// Try clang on macOS
		gccPath, err = exec.LookPath("clang")
		if err != nil {
			return nil, fmt.Errorf("gcc/clang not found")
		}
	}

	binPath := filepath.Join(tempDir, "main")

	// Compile
	compileCmd := exec.Command(gccPath, "-o", binPath, "-Wall", "-O2", filePath, "-lm")
	output, err := compileCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("compilation failed: %s", string(output))
	}

	cmd := exec.Command(binPath, args...)
	return cmd, nil
}

func (r *CRunner) NeedsCompilation() bool {
	return true
}

func (r *CRunner) Compile(tempDir, filename string) (string, error) {
	gccPath, err := exec.LookPath("gcc")
	if err != nil {
		gccPath, err = exec.LookPath("clang")
		if err != nil {
			return "", fmt.Errorf("gcc/clang not found")
		}
	}

	srcPath := filepath.Join(tempDir, filename)
	binPath := filepath.Join(tempDir, "main")

	cmd := exec.Command(gccPath, "-o", binPath, "-Wall", "-O2", srcPath, "-lm")
	cmd.Dir = tempDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("compilation failed: %s", string(output))
	}

	return binPath, nil
}

// =============================================================================
// CppRunner - C++ execution with G++
// =============================================================================

type CppRunner struct{}

func (r *CppRunner) Language() string {
	return "cpp"
}

func (r *CppRunner) Extensions() []string {
	return []string{".cpp", ".cc", ".cxx"}
}

func (r *CppRunner) WriteCode(tempDir, code string) (string, error) {
	filename := "main.cpp"
	filepath := filepath.Join(tempDir, filename)

	// Add common includes if not present
	if !strings.Contains(code, "#include") {
		code = "#include <iostream>\n#include <vector>\n#include <string>\n#include <algorithm>\nusing namespace std;\n\n" + code
	}

	if err := os.WriteFile(filepath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write C++ file: %w", err)
	}
	return filename, nil
}

func (r *CppRunner) BuildCommand(tempDir, filename string) (*exec.Cmd, error) {
	binPath, err := r.Compile(tempDir, filename)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(binPath)
	return cmd, nil
}

func (r *CppRunner) BuildCommandForFile(filePath, tempDir string, args []string) (*exec.Cmd, error) {
	gppPath, err := exec.LookPath("g++")
	if err != nil {
		gppPath, err = exec.LookPath("clang++")
		if err != nil {
			return nil, fmt.Errorf("g++/clang++ not found")
		}
	}

	binPath := filepath.Join(tempDir, "main")

	// Compile with C++17
	compileCmd := exec.Command(gppPath, "-o", binPath, "-std=c++17", "-Wall", "-O2", filePath)
	output, err := compileCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("compilation failed: %s", string(output))
	}

	cmd := exec.Command(binPath, args...)
	return cmd, nil
}

func (r *CppRunner) NeedsCompilation() bool {
	return true
}

func (r *CppRunner) Compile(tempDir, filename string) (string, error) {
	gppPath, err := exec.LookPath("g++")
	if err != nil {
		gppPath, err = exec.LookPath("clang++")
		if err != nil {
			return "", fmt.Errorf("g++/clang++ not found")
		}
	}

	srcPath := filepath.Join(tempDir, filename)
	binPath := filepath.Join(tempDir, "main")

	cmd := exec.Command(gppPath, "-o", binPath, "-std=c++17", "-Wall", "-O2", srcPath)
	cmd.Dir = tempDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("compilation failed: %s", string(output))
	}

	return binPath, nil
}

// =============================================================================
// JavaRunner - Java execution
// =============================================================================

type JavaRunner struct{}

func (r *JavaRunner) Language() string {
	return "java"
}

func (r *JavaRunner) Extensions() []string {
	return []string{".java"}
}

func (r *JavaRunner) WriteCode(tempDir, code string) (string, error) {
	// Extract class name from code
	className := extractJavaClassName(code)
	if className == "" {
		// Wrap in Main class if no class found
		code = "public class Main {\n    public static void main(String[] args) {\n        " + code + "\n    }\n}"
		className = "Main"
	}

	filename := className + ".java"
	filepath := filepath.Join(tempDir, filename)

	if err := os.WriteFile(filepath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write Java file: %w", err)
	}
	return filename, nil
}

func (r *JavaRunner) BuildCommand(tempDir, filename string) (*exec.Cmd, error) {
	// Compile first
	className, err := r.Compile(tempDir, filename)
	if err != nil {
		return nil, err
	}

	javaPath, err := exec.LookPath("java")
	if err != nil {
		return nil, fmt.Errorf("java not found")
	}

	cmd := exec.Command(javaPath, "-cp", tempDir, className)
	return cmd, nil
}

func (r *JavaRunner) BuildCommandForFile(filePath, tempDir string, args []string) (*exec.Cmd, error) {
	javacPath, err := exec.LookPath("javac")
	if err != nil {
		return nil, fmt.Errorf("javac not found")
	}

	javaPath, err := exec.LookPath("java")
	if err != nil {
		return nil, fmt.Errorf("java not found")
	}

	// Read file to get class name
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	className := extractJavaClassName(string(content))
	if className == "" {
		className = strings.TrimSuffix(filepath.Base(filePath), ".java")
	}

	// Compile
	compileCmd := exec.Command(javacPath, "-d", tempDir, filePath)
	output, err := compileCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("compilation failed: %s", string(output))
	}

	cmdArgs := append([]string{"-cp", tempDir, className}, args...)
	cmd := exec.Command(javaPath, cmdArgs...)
	return cmd, nil
}

func (r *JavaRunner) NeedsCompilation() bool {
	return true
}

func (r *JavaRunner) Compile(tempDir, filename string) (string, error) {
	javacPath, err := exec.LookPath("javac")
	if err != nil {
		return "", fmt.Errorf("javac not found")
	}

	srcPath := filepath.Join(tempDir, filename)

	cmd := exec.Command(javacPath, "-d", tempDir, srcPath)
	cmd.Dir = tempDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("compilation failed: %s", string(output))
	}

	// Return class name (without .java extension)
	className := strings.TrimSuffix(filename, ".java")
	return className, nil
}

// extractJavaClassName extracts the public class name from Java code
func extractJavaClassName(code string) string {
	re := regexp.MustCompile(`public\s+class\s+(\w+)`)
	matches := re.FindStringSubmatch(code)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// =============================================================================
// RubyRunner - Ruby execution
// =============================================================================

type RubyRunner struct{}

func (r *RubyRunner) Language() string {
	return "ruby"
}

func (r *RubyRunner) Extensions() []string {
	return []string{".rb"}
}

func (r *RubyRunner) WriteCode(tempDir, code string) (string, error) {
	filename := "main.rb"
	filepath := filepath.Join(tempDir, filename)

	if err := os.WriteFile(filepath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write Ruby file: %w", err)
	}
	return filename, nil
}

func (r *RubyRunner) BuildCommand(tempDir, filename string) (*exec.Cmd, error) {
	rubyPath, err := exec.LookPath("ruby")
	if err != nil {
		return nil, fmt.Errorf("ruby not found")
	}

	cmd := exec.Command(rubyPath, filepath.Join(tempDir, filename))
	return cmd, nil
}

func (r *RubyRunner) BuildCommandForFile(filePath, tempDir string, args []string) (*exec.Cmd, error) {
	rubyPath, err := exec.LookPath("ruby")
	if err != nil {
		return nil, fmt.Errorf("ruby not found")
	}

	cmdArgs := append([]string{filePath}, args...)
	cmd := exec.Command(rubyPath, cmdArgs...)
	return cmd, nil
}

func (r *RubyRunner) NeedsCompilation() bool {
	return false
}

func (r *RubyRunner) Compile(tempDir, filename string) (string, error) {
	return filepath.Join(tempDir, filename), nil
}

// =============================================================================
// PHPRunner - PHP execution
// =============================================================================

type PHPRunner struct{}

func (r *PHPRunner) Language() string {
	return "php"
}

func (r *PHPRunner) Extensions() []string {
	return []string{".php"}
}

func (r *PHPRunner) WriteCode(tempDir, code string) (string, error) {
	filename := "main.php"
	filepath := filepath.Join(tempDir, filename)

	// Add PHP tags if not present
	if !strings.HasPrefix(strings.TrimSpace(code), "<?") {
		code = "<?php\n" + code
	}

	if err := os.WriteFile(filepath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write PHP file: %w", err)
	}
	return filename, nil
}

func (r *PHPRunner) BuildCommand(tempDir, filename string) (*exec.Cmd, error) {
	phpPath, err := exec.LookPath("php")
	if err != nil {
		return nil, fmt.Errorf("php not found")
	}

	cmd := exec.Command(phpPath, filepath.Join(tempDir, filename))
	return cmd, nil
}

func (r *PHPRunner) BuildCommandForFile(filePath, tempDir string, args []string) (*exec.Cmd, error) {
	phpPath, err := exec.LookPath("php")
	if err != nil {
		return nil, fmt.Errorf("php not found")
	}

	cmdArgs := append([]string{filePath}, args...)
	cmd := exec.Command(phpPath, cmdArgs...)
	return cmd, nil
}

func (r *PHPRunner) NeedsCompilation() bool {
	return false
}

func (r *PHPRunner) Compile(tempDir, filename string) (string, error) {
	return filepath.Join(tempDir, filename), nil
}
