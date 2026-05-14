package downloadindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetReturnsFieldValue(t *testing.T) {
	indexPath := writeTestIndex(t, strings.Join([]string{
		"id,name,url,repo_url,project_website",
		"reaper,REAPER,https://example.test/reaper,,https://example.test/site",
	}, "\n"))

	value, err := Get(indexPath, "reaper", "url", false)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if value != "https://example.test/reaper" {
		t.Fatalf("unexpected value %q", value)
	}
}

func TestValidateAcceptsRepoURLWithoutArchiveURL(t *testing.T) {
	indexPath := writeTestIndex(t, strings.Join([]string{
		"id,name,url,repo_url",
		"loopino,Loopino,,https://example.test/repo.git",
	}, "\n"))

	count, err := Validate(indexPath)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("unexpected entry count %d", count)
	}
}

func TestValidateRejectsDuplicateIDs(t *testing.T) {
	indexPath := writeTestIndex(t, strings.Join([]string{
		"id,name,url,repo_url",
		"reaper,REAPER,https://example.test/reaper,",
		"reaper,REAPER mirror,https://example.test/reaper-2,",
	}, "\n"))

	_, err := Validate(indexPath)
	if err == nil || !strings.Contains(err.Error(), "duplicate ids") {
		t.Fatalf("expected duplicate id error, got %v", err)
	}
}

func TestValidateAcceptsVariableLengthRows(t *testing.T) {
	indexPath := writeTestIndex(t, strings.Join([]string{
		"id,name,url,repo_url,category",
		"reaper,REAPER,https://example.test/reaper",
		"loopino,Loopino,,https://example.test/repo.git,Synth",
	}, "\n"))

	count, err := Validate(indexPath)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if count != 2 {
		t.Fatalf("unexpected entry count %d", count)
	}
}

func TestURLCandidatesIncludeEveryDistinctURLField(t *testing.T) {
	candidates := urlCandidates(Entry{
		"url":             "https://example.test/download",
		"repo_url":        "https://example.test/repo",
		"project_website": "https://example.test/download",
	})

	if len(candidates) != 2 {
		t.Fatalf("unexpected candidate count %d: %#v", len(candidates), candidates)
	}

	if candidates[0].Field != "url" || candidates[0].URL != "https://example.test/download" {
		t.Fatalf("unexpected first candidate: %#v", candidates[0])
	}
	if candidates[1].Field != "repo_url" || candidates[1].URL != "https://example.test/repo" {
		t.Fatalf("unexpected second candidate: %#v", candidates[1])
	}
}

func writeTestIndex(t *testing.T, contents string) string {
	t.Helper()

	dir := t.TempDir()
	indexPath := filepath.Join(dir, "download-index.csv")
	if err := os.WriteFile(indexPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	return indexPath
}
