package downloadindex

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var knownFields = map[string]struct{}{
	"id":                  {},
	"name":                {},
	"version":             {},
	"url":                 {},
	"formats":             {},
	"primary_bundle_name": {},
	"data_dir_name":       {},
	"data_target_name":    {},
	"warmplace_subdir":    {},
	"launcher_name":       {},
	"desktop_id":          {},
	"desktop_comment":     {},
	"repo_url":            {},
	"extract_dir":         {},
	"category":            {},
	"project_website":     {},
	"dl_within_app":       {},
	"open_source":         {},
	"has_free_version":    {},
}

var requiredFields = []string{"id", "name"}

type Entry map[string]string

type URLFailure struct {
	PackageID string
	Field     string
	URL       string
	Err       error
}

func Load(indexPath string) (map[string]Entry, error) {
	rows, err := loadRows(indexPath)
	if err != nil {
		return nil, err
	}

	lookup := make(map[string]Entry, len(rows))
	duplicates := make([]string, 0)

	for _, row := range rows {
		packageID := row["id"]
		if packageID == "" {
			return nil, fmt.Errorf("download index contains a row with an empty id")
		}
		if _, exists := lookup[packageID]; exists {
			duplicates = append(duplicates, packageID)
		}
		lookup[packageID] = row
	}

	if len(duplicates) > 0 {
		sort.Strings(duplicates)
		return nil, fmt.Errorf("download index contains duplicate ids: %s", strings.Join(duplicates, ", "))
	}

	return lookup, nil
}

func Get(indexPath string, packageID string, field string, allowEmpty bool) (string, error) {
	if _, ok := knownFields[field]; !ok {
		return "", fmt.Errorf("unknown field: %s", field)
	}

	lookup, err := Load(indexPath)
	if err != nil {
		return "", err
	}

	entry, ok := lookup[packageID]
	if !ok {
		return "", fmt.Errorf("package id not found in download index: %s", packageID)
	}

	value := entry[field]
	if value == "" && !allowEmpty {
		return "", fmt.Errorf("field %q is empty for package id %q", field, packageID)
	}

	return value, nil
}

func Validate(indexPath string) (int, error) {
	lookup, err := Load(indexPath)
	if err != nil {
		return 0, err
	}

	for _, packageID := range sortedIDs(lookup) {
		entry := lookup[packageID]
		for _, field := range requiredFields {
			if strings.TrimSpace(entry[field]) == "" {
				return 0, fmt.Errorf("%s: missing required field %q", packageID, field)
			}
		}

		if strings.TrimSpace(entry["url"]) == "" && strings.TrimSpace(entry["repo_url"]) == "" {
			return 0, fmt.Errorf("%s: expected either url or repo_url to be set", packageID)
		}
	}

	return len(lookup), nil
}

func CheckURLs(indexPath string, timeout time.Duration, progress io.Writer) ([]URLFailure, int, error) {
	lookup, err := Load(indexPath)
	if err != nil {
		return nil, 0, err
	}

	client := &http.Client{Timeout: timeout}
	failures := make([]URLFailure, 0)

	for _, packageID := range sortedIDs(lookup) {
		entry := lookup[packageID]
		for _, candidate := range urlCandidates(entry) {
			if progress != nil {
				fmt.Fprintf(progress, "[check] %s %s: %s\n", packageID, candidate.Field, candidate.URL)
			}

			if err := probeURL(client, candidate.URL); err != nil {
				failures = append(failures, URLFailure{
					PackageID: packageID,
					Field:     candidate.Field,
					URL:       candidate.URL,
					Err:       err,
				})
			}
		}
	}

	return failures, len(lookup), nil
}

func probeURL(client *http.Client, url string) error {
	headReq, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return err
	}

	headResp, headErr := client.Do(headReq)
	if headErr == nil {
		defer headResp.Body.Close()
		if headResp.StatusCode >= 200 && headResp.StatusCode < 400 {
			return nil
		}
		headErr = fmt.Errorf("HEAD returned HTTP %d", headResp.StatusCode)
	}

	getReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	getReq.Header.Set("Range", "bytes=0-0")

	getResp, getErr := client.Do(getReq)
	if getErr == nil {
		defer getResp.Body.Close()
		if getResp.StatusCode >= 200 && getResp.StatusCode < 400 {
			return nil
		}
		getErr = fmt.Errorf("GET returned HTTP %d", getResp.StatusCode)
	}

	if getErr != nil {
		return getErr
	}

	return headErr
}

type urlCandidate struct {
	Field string
	URL   string
}

func urlCandidates(entry Entry) []urlCandidate {
	fields := []string{"url", "repo_url", "project_website"}
	candidates := make([]urlCandidate, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))

	for _, field := range fields {
		url := strings.TrimSpace(entry[field])
		if url == "" {
			continue
		}
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		candidates = append(candidates, urlCandidate{Field: field, URL: url})
	}

	return candidates
}

func loadRows(indexPath string) ([]Entry, error) {
	file, err := os.Open(filepath.Clean(indexPath))
	if err != nil {
		return nil, fmt.Errorf("open download index: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("download index is missing a header row: %s", indexPath)
		}
		return nil, fmt.Errorf("read download index header: %w", err)
	}

	header = trimFields(header)
	if err := validateHeader(header); err != nil {
		return nil, err
	}

	rows := make([]Entry, 0)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read download index row: %w", err)
		}

		row := make(Entry, len(header))
		for i, key := range header {
			if i < len(record) {
				row[key] = strings.TrimSpace(record[i])
				continue
			}
			row[key] = ""
		}
		rows = append(rows, row)
	}

	return rows, nil
}

func validateHeader(header []string) error {
	headerSet := make(map[string]struct{}, len(header))
	unknown := make([]string, 0)

	for _, field := range header {
		headerSet[field] = struct{}{}
		if _, ok := knownFields[field]; !ok {
			unknown = append(unknown, field)
		}
	}

	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("download index contains unknown columns: %s", strings.Join(unknown, ", "))
	}

	missing := make([]string, 0)
	for _, field := range requiredFields {
		if _, ok := headerSet[field]; !ok {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("download index is missing required columns: %s", strings.Join(missing, ", "))
	}

	return nil
}

func trimFields(fields []string) []string {
	trimmed := make([]string, 0, len(fields))
	for _, field := range fields {
		trimmed = append(trimmed, strings.TrimSpace(field))
	}
	return trimmed
}

func sortedIDs(lookup map[string]Entry) []string {
	ids := make([]string, 0, len(lookup))
	for packageID := range lookup {
		ids = append(ids, packageID)
	}
	sort.Strings(ids)
	return ids
}
