package main

import (
	"strings"
	"testing"

	"github.com/caracal-dev/caracal-software-installer/internal/catalog"
	"github.com/caracal-dev/caracal-software-installer/internal/downloadindex"
)

func TestMatchLocalInstallRecordMatchesExactArchiveBasename(t *testing.T) {
	records := []catalogRecord{
		testRecord("reaper", "REAPER", downloadindex.Entry{
			"url": "https://www.reaper.fm/files/7.x/reaper765_linux_x86_64.tar.xz",
		}),
		testRecord("renoise", "Renoise", downloadindex.Entry{
			"url": "https://files.renoise.com/demo/Renoise_3_5_4_Demo_Linux_x86_64.tar.gz",
		}),
	}

	record, err := matchLocalInstallRecord(records, "/tmp/reaper765_linux_x86_64.tar.xz")
	if err != nil {
		t.Fatalf("matchLocalInstallRecord returned error: %v", err)
	}
	if record.Package.ID != "reaper" {
		t.Fatalf("expected reaper match, got %s", record.Package.ID)
	}
}

func TestMatchLocalInstallRecordMatchesPrimaryBundleName(t *testing.T) {
	records := []catalogRecord{
		testRecord("tal-j8x", "TAL-J-8X", downloadindex.Entry{
			"url":                 "https://tal-software.com/downloads/plugins/TAL-J8X_64_linux.zip",
			"formats":             "clap,vst3,vst",
			"primary_bundle_name": "TAL-J8X",
		}),
		testRecord("tal-pha", "TAL-Pha", downloadindex.Entry{
			"url":                 "https://tal-software.com/downloads/plugins/TAL-Pha_64_linux.zip",
			"formats":             "clap,vst3,vst",
			"primary_bundle_name": "TAL-Pha",
		}),
	}

	record, err := matchLocalInstallRecord(records, "/tmp/TAL-J8X_64_linux.zip")
	if err != nil {
		t.Fatalf("matchLocalInstallRecord returned error: %v", err)
	}
	if record.Package.ID != "tal-j8x" {
		t.Fatalf("expected tal-j8x match, got %s", record.Package.ID)
	}
}

func TestMatchLocalInstallRecordReportsAmbiguousMatches(t *testing.T) {
	records := []catalogRecord{
		testRecord("floe-vst", "Floe", downloadindex.Entry{
			"formats":             "vst3",
			"primary_bundle_name": "Floe",
		}),
		testRecord("floe-clap", "Floe", downloadindex.Entry{
			"formats":             "clap",
			"primary_bundle_name": "Floe",
		}),
	}

	_, err := matchLocalInstallRecord(records, "/tmp/Floe-Linux.tar.gz")
	if err == nil || !strings.Contains(err.Error(), "Pass the package id explicitly") {
		t.Fatalf("expected ambiguous match error, got %v", err)
	}
}

func testRecord(id string, name string, entry downloadindex.Entry) catalogRecord {
	if entry["name"] == "" {
		entry["name"] = name
	}
	return catalogRecord{
		Package: &catalog.Package{
			ID:   id,
			Name: name,
		},
		Index: entry,
	}
}
