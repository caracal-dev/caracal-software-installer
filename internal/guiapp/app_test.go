package guiapp

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/caracal-dev/caracal-software-installer/internal/catalog"
	"github.com/caracal-dev/caracal-software-installer/internal/installer"
)

func TestTransformActionExecAddsDesktopRootEnvironment(t *testing.T) {
	binDir := t.TempDir()
	for _, name := range []string{"pkexec", "env"} {
		path := filepath.Join(binDir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("write fake %s: %v", name, err)
		}
	}

	t.Setenv("PATH", binDir)
	t.Setenv("USER", "desktop-user")
	t.Setenv("DISPLAY", ":0")
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")

	app := New(nil)
	transformed, err := app.transformActionExec(
		installer.Job{Package: &catalog.Package{Name: "Mixbus"}},
		catalog.Action{Title: "Install Mixbus"},
		[]string{"sudo", "bash", "/usr/lib/caracal-software-installer/scripts/install-mixbus.sh"},
	)
	if err != nil {
		t.Fatalf("transformActionExec returned error: %v", err)
	}

	if len(transformed) < 2 || filepath.Base(transformed[0]) != "pkexec" || filepath.Base(transformed[1]) != "env" {
		t.Fatalf("expected pkexec env prefix, got %#v", transformed)
	}

	for _, want := range []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"DISPLAY=:0",
		"WAYLAND_DISPLAY=wayland-0",
		"XDG_RUNTIME_DIR=/run/user/1000",
		"CARACAL_INSTALLER_TARGET_USER=desktop-user",
	} {
		if !slices.Contains(transformed, want) {
			t.Fatalf("expected transformed command to contain %q, got %#v", want, transformed)
		}
	}

	if got := transformed[len(transformed)-2:]; !slices.Equal(got, []string{"bash", "/usr/lib/caracal-software-installer/scripts/install-mixbus.sh"}) {
		t.Fatalf("expected sudo to be stripped while preserving command, got %#v", got)
	}
}

func TestSetDesktopIconCopiesSelectedIcon(t *testing.T) {
	root := t.TempDir()
	iconDir := filepath.Join(root, "build", "icons")
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		t.Fatalf("mkdir icon dir: %v", err)
	}

	defaultIcon := []byte("default icon")
	selectedIcon := []byte("selected icon")
	if err := os.WriteFile(filepath.Join(iconDir, "appicon.png"), defaultIcon, 0o644); err != nil {
		t.Fatalf("write default icon: %v", err)
	}
	if err := os.WriteFile(filepath.Join(iconDir, "caracal-lakers.png"), selectedIcon, 0o644); err != nil {
		t.Fatalf("write selected icon: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "build", "appicon.png"), defaultIcon, 0o644); err != nil {
		t.Fatalf("write target icon: %v", err)
	}

	t.Setenv("CARACAL_INSTALLER_ICON_DIR", iconDir)

	app := New(nil)
	payload, err := app.SetDesktopIcon("caracal-lakers.png")
	if err != nil {
		t.Fatalf("SetDesktopIcon returned error: %v", err)
	}
	if payload.ActiveID != "caracal-lakers.png" {
		t.Fatalf("expected active icon caracal-lakers.png, got %q", payload.ActiveID)
	}

	target, err := os.ReadFile(filepath.Join(root, "build", "appicon.png"))
	if err != nil {
		t.Fatalf("read target icon: %v", err)
	}
	if string(target) != string(selectedIcon) {
		t.Fatalf("target icon was not updated")
	}
}
