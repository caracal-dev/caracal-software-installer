package installer

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/caracal-dev/caracal-software-installer/internal/catalog"
)

func TestMarkerExistsExpandsRelativeGlobIntoHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	target := filepath.Join(home, ".vst3", "DragonflyHall.vst3")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}

	if !markerExists(".vst3/Dragonfly*.vst3") {
		t.Fatal("expected relative glob marker to match inside HOME")
	}
}

func TestLogDirCreatesDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir, err := logDir()
	if err != nil {
		t.Fatalf("logDir: %v", err)
	}
	expected := filepath.Join(home, ".local", "share", "caracal-software-installer", "logs")
	if dir != expected {
		t.Errorf("logDir = %q, want %q", dir, expected)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("logDir did not create the directory")
	}
}

func TestOpenActionLogWritesHeader(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	job := Job{
		Package: &catalog.Package{
			ID:   "reaper",
			Name: "REAPER",
		},
		Mode: ModeInstall,
	}
	action := catalog.Action{
		Title: "Install REAPER",
		Exec:  []string{"bash", "install-reaper.sh"},
	}

	f, path, err := openActionLog(job, action)
	if err != nil {
		t.Fatalf("openActionLog: %v", err)
	}
	defer f.Close()

	if path == "" {
		t.Fatal("expected non-empty log path")
	}
	if !strings.HasSuffix(path, ".log") {
		t.Errorf("log path should end with .log: %s", path)
	}
	if !strings.Contains(path, "reaper") {
		t.Errorf("log path should contain package id: %s", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "REAPER") {
		t.Errorf("log header missing package name: %s", text)
	}
	if !strings.Contains(text, "install") {
		t.Errorf("log header missing mode: %s", text)
	}
	if !strings.Contains(text, "Install REAPER") {
		t.Errorf("log header missing action title: %s", text)
	}
}

func TestStreamOutputWritesToLogFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	logFile, err := os.CreateTemp(home, "stream-test-*.log")
	if err != nil {
		t.Fatalf("create temp log: %v", err)
	}
	defer logFile.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	reader := strings.NewReader("line1\nline2\nline3\n")
	streamOutput(&wg, reader, "stdout", Job{}, catalog.Action{}, nil, logFile)
	wg.Wait()

	content, err := os.ReadFile(logFile.Name())
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "[stdout] line1") {
		t.Errorf("log missing first line: %s", text)
	}
	if !strings.Contains(text, "[stdout] line2") {
		t.Errorf("log missing second line: %s", text)
	}
	if !strings.Contains(text, "[stdout] line3") {
		t.Errorf("log missing third line: %s", text)
	}
}

func TestLogDirIsXdgDataHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir, err := logDir()
	if err != nil {
		t.Fatalf("logDir: %v", err)
	}
	expected := filepath.Join(home, ".local", "share", "caracal-software-installer", "logs")
	if dir != expected {
		t.Errorf("logDir = %q, want %q", dir, expected)
	}
}
