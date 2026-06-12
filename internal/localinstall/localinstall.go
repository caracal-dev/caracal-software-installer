package localinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/caracal-dev/caracal-software-installer/internal/catalog"
	"github.com/caracal-dev/caracal-software-installer/internal/downloadindex"
	"github.com/caracal-dev/caracal-software-installer/internal/installer"
)

type Record struct {
	CategoryID      string
	CategoryName    string
	SubcategoryID   string
	SubcategoryName string
	Package         *catalog.Package
	Index           downloadindex.Entry
}

func LoadRecords(scriptDir string, indexPath string) ([]Record, error) {
	lookup, err := downloadindex.Load(indexPath)
	if err != nil {
		return nil, err
	}

	records := make([]Record, 0, len(lookup))
	seen := make(map[string]struct{}, len(lookup))
	for _, category := range catalog.Build(scriptDir, lookup) {
		for _, subcategory := range category.Subcategories {
			for _, pkg := range subcategory.Packages {
				seen[pkg.ID] = struct{}{}
				records = append(records, Record{
					CategoryID:      category.ID,
					CategoryName:    category.Name,
					SubcategoryID:   subcategory.ID,
					SubcategoryName: subcategory.Name,
					Package:         pkg,
					Index:           lookup[pkg.ID],
				})
			}
		}
	}

	for id, entry := range lookup {
		if _, ok := seen[id]; ok {
			continue
		}

		categoryName := entry["category"]
		if categoryName == "" {
			categoryName = "Uncategorized"
		}
		categoryID := normalizeTag(categoryName)
		if categoryID == "" {
			categoryID = "uncategorized"
		}

		records = append(records, Record{
			CategoryID:    categoryID,
			CategoryName:  categoryName,
			SubcategoryID: "csv-only",
			Package: &catalog.Package{
				ID:                id,
				Name:              entry["name"],
				SoftwareTypes:     softwareTypesFromEntry(entry),
				OpenSource:        boolField(entry["open_source"], false),
				HasFreeVersion:    boolField(entry["has_free_version"], true),
				ExternalActionURL: externalURL(entry),
			},
			Index: entry,
		})
	}

	return records, nil
}

func ResolveJobs(scriptDir string, records []Record, localFiles []string, ids []string, force bool) ([]installer.Job, []string, error) {
	if len(ids) > 1 {
		return nil, nil, fmt.Errorf("usage: caracal install --from-file <archive> [id]")
	}

	byID := RecordsByID(records)
	jobs := make([]installer.Job, 0, len(localFiles))
	skipped := make([]string, 0)
	for _, rawPath := range localFiles {
		localPath, err := ResolveFile(rawPath)
		if err != nil {
			return nil, nil, err
		}

		var record Record
		if len(ids) == 1 {
			var ok bool
			record, ok = byID[ids[0]]
			if !ok {
				return nil, nil, fmt.Errorf("software id not found: %s", ids[0])
			}
		} else {
			record, err = MatchRecord(records, localPath)
			if err != nil {
				return nil, nil, err
			}
		}

		pkg, err := Package(scriptDir, record, localPath)
		if err != nil {
			return nil, nil, err
		}

		state := installer.Detect(pkg)
		if state.Installed && !force {
			skipped = append(skipped, pkg.Name)
			continue
		}

		jobs = append(jobs, installer.Job{Package: pkg, Mode: installer.ModeInstall})
	}

	return jobs, skipped, nil
}

func RecordsByID(records []Record) map[string]Record {
	byID := make(map[string]Record, len(records))
	for _, record := range records {
		byID[record.Package.ID] = record
	}
	return byID
}

func ResolveFile(rawPath string) (string, error) {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return "", fmt.Errorf("local install file path is empty")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve local install file %q: %w", path, err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("local install file not found: %s", absPath)
	}
	if info.IsDir() {
		return "", fmt.Errorf("local install path must be a file, got directory: %s", absPath)
	}
	if !HasSupportedExtension(absPath) {
		return "", fmt.Errorf("unsupported local install file type: %s", absPath)
	}
	return absPath, nil
}

func HasSupportedExtension(path string) bool {
	lower := strings.ToLower(filepath.Base(path))
	for _, suffix := range []string{
		".zip",
		".7z",
		".deb",
		".tar",
		".tar.gz",
		".tgz",
		".tar.xz",
		".txz",
		".tar.bz2",
		".tbz2",
		".tar.zst",
		".clap",
		".vst3",
		".so",
	} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func MatchRecord(records []Record, localPath string) (Record, error) {
	type scoredRecord struct {
		record Record
		score  int
	}

	matches := make([]scoredRecord, 0)
	for _, record := range records {
		score := matchScore(record, localPath)
		if score > 0 {
			matches = append(matches, scoredRecord{record: record, score: score})
		}
	}

	if len(matches) == 0 {
		return Record{}, fmt.Errorf("could not match %s to a Caracal package; pass the package id explicitly, for example: caracal install --from-file %s reaper", filepath.Base(localPath), shellQuote(localPath))
	}

	sort.Slice(matches, func(i int, j int) bool {
		if matches[i].score == matches[j].score {
			return matches[i].record.Package.ID < matches[j].record.Package.ID
		}
		return matches[i].score > matches[j].score
	})

	if len(matches) > 1 && matches[0].score == matches[1].score {
		ids := []string{matches[0].record.Package.ID, matches[1].record.Package.ID}
		for _, match := range matches[2:] {
			if match.score != matches[0].score {
				break
			}
			ids = append(ids, match.record.Package.ID)
		}
		return Record{}, fmt.Errorf("could not choose one Caracal package for %s; matched: %s. Pass the package id explicitly", filepath.Base(localPath), strings.Join(ids, ", "))
	}

	return matches[0].record, nil
}

func matchScore(record Record, localPath string) int {
	if !hasRoute(record) {
		return 0
	}

	base := strings.ToLower(filepath.Base(localPath))
	baseKey := normalizeFileMatchKey(stripArchiveSuffix(base))
	score := 0

	if urlBase := strings.ToLower(filepath.Base(strings.TrimSpace(record.Index["url"]))); urlBase != "" && urlBase != "." {
		urlBase = strings.Split(urlBase, "?")[0]
		if base == urlBase {
			score = max(score, 120)
		}
		if urlKey := normalizeFileMatchKey(stripArchiveSuffix(urlBase)); urlKey != "" && baseKey == urlKey {
			score = max(score, 110)
		}
	}

	for _, candidate := range []string{
		record.Package.ID,
		record.Package.Name,
		record.Index["name"],
		record.Index["primary_bundle_name"],
	} {
		key := normalizeFileMatchKey(candidate)
		if key == "" {
			continue
		}
		switch {
		case baseKey == key:
			score = max(score, 100)
		case strings.Contains(baseKey, key):
			score = max(score, min(90, 40+len(key)))
		}
	}

	return score
}

func hasRoute(record Record) bool {
	id := record.Package.ID
	switch id {
	case "reaper", "renoise", "bitwig-studio", "mixbus", "decent-sampler", "sunvox", "virtual-ans":
		return true
	}
	if isAlienDebPackageID(id) {
		return true
	}
	if strings.TrimSpace(record.Index["formats"]) != "" && strings.TrimSpace(record.Index["primary_bundle_name"]) != "" {
		return true
	}
	if strings.HasSuffix(strings.ToLower(record.Index["url"]), ".deb") {
		return true
	}
	return false
}

func Package(scriptDir string, record Record, localPath string) (*catalog.Package, error) {
	pkg := *record.Package
	pkg.InstallActions = nil
	pkg.ExternalActionURL = ""
	pkg.AvailabilityNote = ""

	script := func(name string, args ...string) []string {
		exec := []string{"bash", filepath.Join(scriptDir, name)}
		return append(exec, args...)
	}
	sudoScript := func(name string, args ...string) []string {
		exec := []string{"sudo", "bash", filepath.Join(scriptDir, name)}
		return append(exec, args...)
	}

	entry := record.Index
	switch pkg.ID {
	case "reaper":
		pkg.InstallActions = []catalog.Action{{Title: "Install REAPER from local file", Exec: sudoScript("install-reaper-local.sh", localPath)}}
	case "renoise":
		pkg.InstallActions = []catalog.Action{{Title: "Install Renoise from local file", Exec: sudoScript("install-renoise-local.sh", localPath)}}
	case "bitwig-studio":
		pkg.InstallActions = []catalog.Action{{Title: "Install Bitwig Studio from local file", Exec: sudoScript("install-bitwig-local.sh", localPath)}}
	case "mixbus":
		pkg.InstallActions = []catalog.Action{{Title: "Install Mixbus from local file", Exec: sudoScript("install-mixbus-local.sh", localPath)}}
	case "decent-sampler":
		pkg.InstallActions = []catalog.Action{{Title: "Install Decent Sampler from local file", Exec: sudoScript("install-decent-sampler-local.sh", localPath)}}
	case "sunvox":
		pkg.InstallActions = []catalog.Action{{Title: "Install SunVox from local file", Exec: sudoScript("install-warmplace-zip-app.sh", "sunvox", "SunVox", entry["version"], entry["url"], "sunvox", "sunvox", "sunvox", "Modular tracker and synthesizer", localPath)}}
	case "virtual-ans":
		pkg.InstallActions = []catalog.Action{{Title: "Install Virtual ANS from local file", Exec: sudoScript("install-warmplace-zip-app.sh", "virtual-ans", "Virtual ANS", entry["version"], entry["url"], "virtual_ans", "virtual-ans", "virtual-ans", "Spectral drawing synthesizer", localPath)}}
	default:
		if isAlienDebPackageID(pkg.ID) || (strings.HasSuffix(strings.ToLower(localPath), ".deb") && strings.TrimSpace(entry["primary_bundle_name"]) == "") {
			pkg.InstallActions = []catalog.Action{{
				Title: fmt.Sprintf("Install %s from local file", pkg.Name),
				Exec: script(
					"install-plugin-archive.sh",
					pkg.ID,
					pkg.Name,
					entry["url"],
					entry["primary_bundle_name"],
					entry["formats"],
					entry["data_dir_name"],
					entry["data_target_name"],
					localPath,
				),
			}}
			break
		}
		if strings.TrimSpace(entry["formats"]) != "" && strings.TrimSpace(entry["primary_bundle_name"]) != "" {
			pkg.InstallActions = []catalog.Action{{
				Title: fmt.Sprintf("Install %s from local file", pkg.Name),
				Exec: script(
					"install-plugin-archive.sh",
					pkg.ID,
					pkg.Name,
					entry["url"],
					entry["primary_bundle_name"],
					entry["formats"],
					entry["data_dir_name"],
					entry["data_target_name"],
					localPath,
				),
			}}
		}
	}

	if len(pkg.InstallActions) == 0 {
		return nil, fmt.Errorf("%s does not support local-file installs yet", pkg.ID)
	}
	return &pkg, nil
}

func isAlienDebPackageID(id string) bool {
	switch id {
	case "byod",
		"ts-m1n3",
		"chameleon",
		"smartamp",
		"smartpedal",
		"proteus",
		"epochamp",
		"neuralpi",
		"the-prince",
		"chow-phaser",
		"chow-tape-model",
		"chow-multitool":
		return true
	default:
		return false
	}
}

func stripArchiveSuffix(value string) string {
	for _, suffix := range []string{
		".tar.gz",
		".tar.xz",
		".tar.bz2",
		".tar.zst",
		".tgz",
		".txz",
		".tbz2",
		".zip",
		".7z",
		".deb",
		".tar",
		".clap",
		".vst3",
		".so",
	} {
		if strings.HasSuffix(value, suffix) {
			return strings.TrimSuffix(value, suffix)
		}
	}
	return value
}

func normalizeFileMatchKey(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func normalizeTag(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastHyphen := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastHyphen = false
		default:
			if !lastHyphen {
				builder.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}

func softwareTypesFromEntry(entry downloadindex.Entry) []string {
	seen := make(map[string]struct{}, 4)
	types := make([]string, 0, 4)
	add := func(value string) {
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		types = append(types, value)
	}

	for _, format := range strings.Split(entry["formats"], ",") {
		switch normalizeTag(format) {
		case "clap":
			add("clap")
		case "vst", "vst3":
			add("vst")
		case "lv2":
			add("lv2")
		}
	}
	if len(types) == 0 {
		add("standalone")
	}
	return types
}

func boolField(value string, defaultValue bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return defaultValue
	}
}

func externalURL(entry downloadindex.Entry) string {
	if entry["url"] != "" {
		return entry["url"]
	}
	if entry["project_website"] != "" {
		return entry["project_website"]
	}
	return entry["repo_url"]
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("-_./:=+,%@", r))
	}) == -1 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
