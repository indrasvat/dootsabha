package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const extensionPrefix = "dootsabha-"

// ExtensionDirs returns standard directories to scan for extensions
// beyond $PATH: ~/.local/bin (user), /usr/local/bin (system).
func ExtensionDirs() []string {
	dirs := []string{"/usr/local/bin"}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append([]string{filepath.Join(home, ".local", "bin")}, dirs...)
	}
	return dirs
}

// Extension describes a discovered external extension binary.
type Extension struct {
	// Name is the extension name (without the dootsabha- prefix).
	Name string
	// Path is the absolute path to the binary.
	Path string
}

// DiscoverExtensions scans $PATH and optional plugin directories for binaries
// named dootsabha-{name}. Returns deduplicated extensions (first match wins).
func DiscoverExtensions(extraDirs ...string) []Extension {
	seen := make(map[string]bool)
	var extensions []Extension

	// Prepend extra directories so user-local dirs win over $PATH.
	pathEnv := os.Getenv("PATH")
	dirs := make([]string, 0, len(extraDirs)+1)
	dirs = append(dirs, extraDirs...)
	dirs = append(dirs, filepath.SplitList(pathEnv)...)

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasPrefix(name, extensionPrefix) {
				continue
			}

			extName := strings.TrimPrefix(name, extensionPrefix)
			if extName == "" {
				continue
			}
			if seen[extName] {
				continue
			}

			fullPath := filepath.Join(dir, name)
			info, err := os.Stat(fullPath)
			if err != nil {
				continue
			}
			// Must be executable.
			if info.Mode()&0o111 == 0 {
				continue
			}

			seen[extName] = true
			extensions = append(extensions, Extension{
				Name: extName,
				Path: fullPath,
			})
		}
	}

	return extensions
}

// FindExtension searches for a specific extension by name.
// Returns the extension and true if found, zero value and false otherwise.
func FindExtension(name string, extraDirs ...string) (Extension, bool) {
	// Prepend extra directories so user-local dirs win over $PATH.
	pathEnv := os.Getenv("PATH")
	dirs := make([]string, 0, len(extraDirs)+1)
	dirs = append(dirs, extraDirs...)
	dirs = append(dirs, filepath.SplitList(pathEnv)...)

	binaryName := extensionPrefix + name

	for _, dir := range dirs {
		fullPath := filepath.Join(dir, binaryName)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		if info.Mode()&0o111 == 0 {
			continue
		}
		return Extension{Name: name, Path: fullPath}, true
	}

	return Extension{}, false
}

// ExtensionEnv returns environment variables that provide Tier 1 context
// to extension binaries (DOOTSABHA_* env vars).
func ExtensionEnv() []string {
	env := os.Environ()
	env = append(env, fmt.Sprintf("DOOTSABHA_VERSION=%s", "1.0.0"))
	env = append(env, "DOOTSABHA_PLUGIN=1")
	return env
}
