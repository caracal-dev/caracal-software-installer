package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/caracal-os/caracal-software-installer/internal/catalog"
	"github.com/caracal-os/caracal-software-installer/internal/downloadindex"
	"github.com/caracal-os/caracal-software-installer/internal/ui"
)

func main() {
	scriptDir, err := resolveScriptDir()
	if err != nil {
		log.Fatal(err)
	}

	downloadIndexPath, err := resolveDownloadIndexPath(scriptDir)
	if err != nil {
		log.Fatal(err)
	}

	downloadLookup, err := downloadindex.Load(downloadIndexPath)
	if err != nil {
		log.Fatal(err)
	}

	logo := resolveLogo()

	app := ui.New(catalog.Build(scriptDir, downloadLookup), logo)
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func resolveScriptDir() (string, error) {
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

func resolveLogo() string {
	if envPath := os.Getenv("CARACAL_INSTALLER_LOGO"); envPath != "" {
		if data, err := os.ReadFile(envPath); err == nil {
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

		data, err := os.ReadFile(clean)
		if err == nil {
			return strings.TrimRight(string(data), "\n")
		}
	}

	return ""
}

func resolveDownloadIndexPath(scriptDir string) (string, error) {
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
