package plugin_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/indrasvat/dootsabha/internal/plugin"
)

// isolatePATH sets PATH to empty so DiscoverExtensions only sees extraDirs.
func isolatePATH(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", "")
}

// setupExtensionDir creates a temp directory with fake extension binaries.
func setupExtensionDir(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		path := filepath.Join(dir, "dootsabha-"+name)
		if err := os.WriteFile(path, []byte("#!/bin/bash\necho hello"), 0o755); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return dir
}

func TestDiscoverExtensionsEmpty(t *testing.T) {
	isolatePATH(t)
	dir := t.TempDir()
	exts := plugin.DiscoverExtensions(dir)
	if len(exts) != 0 {
		t.Errorf("expected 0 extensions, got %d", len(exts))
	}
}

func TestDiscoverExtensionsSingle(t *testing.T) {
	isolatePATH(t)
	dir := setupExtensionDir(t, "hello")
	exts := plugin.DiscoverExtensions(dir)
	if len(exts) != 1 {
		t.Fatalf("expected 1 extension, got %d", len(exts))
	}
	if exts[0].Name != "hello" {
		t.Errorf("name = %q, want hello", exts[0].Name)
	}
	if exts[0].Path != filepath.Join(dir, "dootsabha-hello") {
		t.Errorf("path = %q", exts[0].Path)
	}
}

func TestDiscoverExtensionsMultiple(t *testing.T) {
	isolatePATH(t)
	dir := setupExtensionDir(t, "hello", "world", "test")
	exts := plugin.DiscoverExtensions(dir)
	if len(exts) != 3 {
		t.Fatalf("expected 3 extensions, got %d", len(exts))
	}
	names := make(map[string]bool)
	for _, ext := range exts {
		names[ext.Name] = true
	}
	for _, want := range []string{"hello", "world", "test"} {
		if !names[want] {
			t.Errorf("missing extension %q", want)
		}
	}
}

func TestDiscoverExtensionsDedup(t *testing.T) {
	isolatePATH(t)
	dir1 := setupExtensionDir(t, "hello")
	dir2 := setupExtensionDir(t, "hello")
	exts := plugin.DiscoverExtensions(dir1, dir2)
	if len(exts) != 1 {
		t.Errorf("expected 1 dedup'd extension, got %d", len(exts))
	}
}

func TestDiscoverExtensionsSkipsNonExecutable(t *testing.T) {
	isolatePATH(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "dootsabha-noexec")
	if err := os.WriteFile(path, []byte("#!/bin/bash"), 0o644); err != nil {
		t.Fatal(err)
	}
	exts := plugin.DiscoverExtensions(dir)
	if len(exts) != 0 {
		t.Errorf("expected 0 (non-executable), got %d", len(exts))
	}
}

func TestDiscoverExtensionsSkipsDirectories(t *testing.T) {
	isolatePATH(t)
	dir := t.TempDir()
	subdir := filepath.Join(dir, "dootsabha-subdir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	exts := plugin.DiscoverExtensions(dir)
	if len(exts) != 0 {
		t.Errorf("expected 0 (directory), got %d", len(exts))
	}
}

func TestDiscoverExtensionsSkipsBarePrefix(t *testing.T) {
	isolatePATH(t)
	dir := t.TempDir()
	// "dootsabha-" with no suffix — should be skipped.
	path := filepath.Join(dir, "dootsabha-")
	if err := os.WriteFile(path, []byte("#!/bin/bash"), 0o755); err != nil {
		t.Fatal(err)
	}
	exts := plugin.DiscoverExtensions(dir)
	if len(exts) != 0 {
		t.Errorf("expected 0 (bare prefix), got %d", len(exts))
	}
}

func TestDiscoverExtensionsSkipsNonPrefixed(t *testing.T) {
	isolatePATH(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "some-other-binary")
	if err := os.WriteFile(path, []byte("#!/bin/bash"), 0o755); err != nil {
		t.Fatal(err)
	}
	exts := plugin.DiscoverExtensions(dir)
	if len(exts) != 0 {
		t.Errorf("expected 0 (no prefix), got %d", len(exts))
	}
}

func TestFindExtensionFound(t *testing.T) {
	dir := setupExtensionDir(t, "greet")
	ext, found := plugin.FindExtension("greet", dir)
	if !found {
		t.Fatal("expected to find extension")
	}
	if ext.Name != "greet" {
		t.Errorf("name = %q", ext.Name)
	}
	if ext.Path != filepath.Join(dir, "dootsabha-greet") {
		t.Errorf("path = %q", ext.Path)
	}
}

func TestFindExtensionNotFound(t *testing.T) {
	dir := t.TempDir()
	_, found := plugin.FindExtension("nonexistent", dir)
	if found {
		t.Fatal("expected not found")
	}
}

func TestFindExtensionFirstDirWins(t *testing.T) {
	dir1 := setupExtensionDir(t, "hello")
	dir2 := setupExtensionDir(t, "hello")
	ext, found := plugin.FindExtension("hello", dir1, dir2)
	if !found {
		t.Fatal("expected to find extension")
	}
	// Should find in dir1 first.
	if ext.Path != filepath.Join(dir1, "dootsabha-hello") {
		t.Errorf("expected path from dir1, got %q", ext.Path)
	}
}

func TestExtensionDirsContainsExpectedPaths(t *testing.T) {
	dirs := plugin.ExtensionDirs()
	if len(dirs) < 1 {
		t.Fatal("expected at least 1 directory from ExtensionDirs()")
	}
	// Last entry should always be /usr/local/bin.
	if dirs[len(dirs)-1] != "/usr/local/bin" {
		t.Errorf("last dir = %q, want /usr/local/bin", dirs[len(dirs)-1])
	}
	// First entry should be ~/.local/bin (if home is available).
	home, err := os.UserHomeDir()
	if err == nil {
		want := filepath.Join(home, ".local", "bin")
		if dirs[0] != want {
			t.Errorf("first dir = %q, want %q", dirs[0], want)
		}
	}
}

func TestExtensionDirsOrder(t *testing.T) {
	dirs := plugin.ExtensionDirs()
	if len(dirs) != 2 {
		t.Skipf("expected 2 dirs, got %d (home may not be available)", len(dirs))
	}
	// ~/.local/bin should come before /usr/local/bin.
	home, _ := os.UserHomeDir()
	if dirs[0] != filepath.Join(home, ".local", "bin") {
		t.Errorf("dirs[0] = %q, want ~/.local/bin", dirs[0])
	}
	if dirs[1] != "/usr/local/bin" {
		t.Errorf("dirs[1] = %q, want /usr/local/bin", dirs[1])
	}
}

func TestExtensionEnv(t *testing.T) {
	env := plugin.ExtensionEnv()
	foundPlugin := false
	foundVersion := false
	for _, e := range env {
		if e == "DOOTSABHA_PLUGIN=1" {
			foundPlugin = true
		}
		if len(e) > len("DOOTSABHA_VERSION=") && e[:len("DOOTSABHA_VERSION=")] == "DOOTSABHA_VERSION=" {
			foundVersion = true
		}
	}
	if !foundPlugin {
		t.Error("missing DOOTSABHA_PLUGIN=1")
	}
	if !foundVersion {
		t.Error("missing DOOTSABHA_VERSION")
	}
}
