package bootstrap

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/caracal-dev/caracal-software-installer/internal/catalog"
	"github.com/caracal-dev/caracal-software-installer/internal/downloadindex"
)

type Resolved struct {
	ScriptDir         string
	DownloadIndexPath string
	Logo              string
	Categories        []*catalog.Category
}

func Load() (*Resolved, error) {
	scriptDir, err := ResolveScriptDir()
	if err != nil {
		return nil, err
	}

	downloadIndexPath, err := ResolveDownloadIndexPath(scriptDir)
	if err != nil {
		return nil, err
	}

	downloadLookup, err := downloadindex.Load(downloadIndexPath)
	if err != nil {
		return nil, err
	}

	return &Resolved{
		ScriptDir:         scriptDir,
		DownloadIndexPath: downloadIndexPath,
		Logo:              ResolveLogo(),
		Categories:        catalog.Build(scriptDir, downloadLookup),
	}, nil
}

func ResolveScriptDir() (string, error) {
	if envDir := os.Getenv("CARACAL_INSTALLER_SCRIPT_DIR"); envDir != "" && hasCoreScripts(envDir) {
		return envDir, nil
	}

	var candidates []string

	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, candidateScriptDirs(wd)...)
	}

	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, candidateScriptDirs(filepath.Dir(exe))...)
	}

	candidates = append(candidates, "/usr/lib/caracal-software-installer/scripts")

	seen := make(map[string]struct{})
	for _, dir := range candidates {
		if dir == "" {
			continue
		}

		clean := filepath.Clean(dir)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}

		if hasCoreScripts(clean) {
			return clean, nil
		}
	}

	return "", fmt.Errorf("could not find installer scripts; checked CARACAL_INSTALLER_SCRIPT_DIR, /usr/lib/caracal-software-installer/scripts, and repo-local scripts directories")
}

func ResolveLogo() string {
	if envPath := os.Getenv("CARACAL_INSTALLER_LOGO"); envPath != "" {
		if data, err := readRegularFile(envPath); err == nil {
			return strings.TrimRight(string(data), "\n")
		}
	}

	candidates := []string{}

	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, candidateFiles(wd, "logo.txt")...)
	}

	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, candidateFiles(filepath.Dir(exe), "logo.txt")...)
	}

	candidates = append(candidates, "/usr/share/caracal-software-installer/logo.txt")

	seen := make(map[string]struct{})
	for _, path := range candidates {
		if path == "" {
			continue
		}

		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}

		data, err := readRegularFile(clean)
		if err == nil {
			return strings.TrimRight(string(data), "\n")
		}
	}

	return ""
}

func readRegularFile(path string) ([]byte, error) {
	clean, err := cleanReadablePath(path)
	if err != nil {
		return nil, err
	}
	// #nosec G304 -- cleanReadablePath rejects root/current-dir paths and requires an existing regular file.
	return os.ReadFile(clean)
}

func cleanReadablePath(path string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "." || clean == string(filepath.Separator) {
		return "", fmt.Errorf("refusing to read unsafe path %q", path)
	}

	info, err := os.Stat(clean)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", &fs.PathError{Op: "read", Path: clean, Err: fs.ErrInvalid}
	}
	return clean, nil
}

func ResolveDownloadIndexPath(scriptDir string) (string, error) {
	if envPath := os.Getenv("CARACAL_INSTALLER_DOWNLOAD_INDEX_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}
	}

	candidates := []string{
		filepath.Join(filepath.Dir(scriptDir), "data", "download-index.csv"),
	}

	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, candidateRelativePaths(wd, filepath.Join("data", "download-index.csv"))...)
	}

	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, candidateRelativePaths(filepath.Dir(exe), filepath.Join("data", "download-index.csv"))...)
	}

	candidates = append(
		candidates,
		"/usr/lib/caracal-software-installer/data/download-index.csv",
		"/usr/share/caracal-software-installer/data/download-index.csv",
	)

	seen := make(map[string]struct{})
	for _, path := range candidates {
		if path == "" {
			continue
		}

		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}

		if _, err := os.Stat(clean); err == nil {
			return clean, nil
		}
	}

	return "", fmt.Errorf("could not find download index; checked CARACAL_INSTALLER_DOWNLOAD_INDEX_PATH, /usr/lib/caracal-software-installer/data/download-index.csv, /usr/share/caracal-software-installer/data/download-index.csv, and repo-local data directories")
}

func candidateScriptDirs(start string) []string {
	var dirs []string
	for dir := filepath.Clean(start); ; dir = filepath.Dir(dir) {
		dirs = append(dirs, filepath.Join(dir, "scripts"))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return dirs
}

func hasCoreScripts(dir string) bool {
	required := []string{
		"install-reaper.sh",
		"install-cardinal.sh",
	}

	for _, name := range required {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return false
		}
	}

	return true
}

func candidateFiles(start string, name string) []string {
	var files []string
	for dir := filepath.Clean(start); ; dir = filepath.Dir(dir) {
		files = append(files, filepath.Join(dir, name))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return files
}

func candidateRelativePaths(start string, relative string) []string {
	var paths []string
	for dir := filepath.Clean(start); ; dir = filepath.Dir(dir) {
		paths = append(paths, filepath.Join(dir, relative))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return paths
}
