package compiler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// TinyGoResult holds the output of a TinyGo compilation
type TinyGoResult struct {
	Success  bool
	Output   string
	WasmPath string
}

// FindTinyGo locates the tinygo binary in PATH or common install locations
func FindTinyGo() (string, error) {
	names := []string{"tinygo", "tinygo.exe"}
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}

	// Fallback: common install locations (Scoop, etc.)
	home := os.Getenv("USERPROFILE")
	if home == "" {
		home = os.Getenv("HOME")
	}
	fallbacks := []string{
		`C:\Program Files\tinygo\bin\tinygo.exe`,
		`C:\Program Files (x86)\tinygo\bin\tinygo.exe`,
	}
	if home != "" {
		fallbacks = append(fallbacks,
			filepath.Join(home, `scoop\apps\tinygo\current\bin\tinygo.exe`),
			filepath.Join(home, `scoop\shims\tinygo.exe`),
			filepath.Join(home, `scoop\apps\tinygo\0.41.1\bin\tinygo.exe`),
		)
	}
	for _, p := range fallbacks {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("tinygo not found in PATH — install from https://tinygo.org/getting-started/install/")
}

// BuildWASM compiles Go source files in srcDir to a single .wasm binary.
// Returns the path to the output .wasm file.
func BuildWASM(srcDir, outputDir string) (*TinyGoResult, error) {
	tinygo, err := FindTinyGo()
	if err != nil {
		return nil, err
	}

	// Check source directory exists
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("source directory not found: %s", srcDir)
	}

	// Ensure output directory exists
	os.MkdirAll(outputDir, 0755)

	wasmPath := filepath.Join(outputDir, "app.wasm")
	goPattern := filepath.Join(srcDir, "*.go")

	cmd := exec.Command(tinygo,
		"build",
		"-o", wasmPath,
		"-target", "wasm",
		"-opt=0",
		"-no-debug",
		goPattern,
	)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	output, err := cmd.CombinedOutput()
	result := &TinyGoResult{
		Output:   string(output),
		WasmPath: wasmPath,
	}

	if err != nil {
		result.Success = false
		return result, fmt.Errorf("tinygo build failed: %w\nOutput:\n%s", err, string(output))
	}

	result.Success = true
	return result, nil
}

// CopyBridge copies the runtime bridge JS file to the output directory alongside app.wasm.
func CopyBridge(outputDir string) error {
	locations := []string{
		filepath.Join("internal", "server", "bridge.js"),
		filepath.Join("..", "..", "internal", "server", "bridge.js"),
	}

	var src string
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			src = loc
			break
		}
	}

	if src == "" {
		return fmt.Errorf("runtime bridge not found")
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	dest := filepath.Join(outputDir, "bridge.js")
	return os.WriteFile(dest, data, 0644)
}
