package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/caracal-dev/caracal-software-installer/internal/bootstrap"
	"github.com/caracal-dev/caracal-software-installer/internal/catalog"
	"github.com/caracal-dev/caracal-software-installer/internal/downloadindex"
	"github.com/caracal-dev/caracal-software-installer/internal/installer"
)

type catalogRecord struct {
	CategoryID      string
	CategoryName    string
	SubcategoryID   string
	SubcategoryName string
	Package         *catalog.Package
	Index           downloadindex.Entry
}

type stringList []string

func (values *stringList) String() string {
	return strings.Join(*values, ",")
}

func (values *stringList) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			*values = append(*values, trimmed)
		}
	}
	return nil
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("caracal", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	indexPath := fs.String("index", "", "path to the CSV download index")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	rest := fs.Args()
	if len(rest) == 0 {
		printHelp()
		return 0
	}

	switch rest[0] {
	case "help", "--help", "-h":
		printHelp()
		return 0
	case "install":
		return runInstall(*indexPath, rest[1:])
	case "scan":
		return runScan(*indexPath, rest[1:])
	case "launch":
		return runLaunch(rest[1:])
	case "list":
		return runList(*indexPath, rest[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", rest[0])
		printHelp()
		return 2
	}
}

func runInstall(indexOverride string, args []string) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dryRun := fs.Bool("dry-run", false, "print installer actions without running them")
	force := fs.Bool("force", false, "run install actions even if installed markers are already present")
	var localFiles stringList
	fs.Var(&localFiles, "from-file", "install from a local archive/package; package id is optional when the file can be matched")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	ids := fs.Args()
	if len(localFiles) > 0 {
		return runInstallFromFiles(indexOverride, *dryRun, *force, localFiles, ids)
	}
	if len(ids) == 0 {
		fmt.Fprintln(os.Stderr, "usage: caracal install [--dry-run] [--force] <id> [id...]")
		fmt.Fprintln(os.Stderr, "       caracal install [--dry-run] [--force] --from-file <archive> [id]")
		return 2
	}

	records, err := loadRecords(indexOverride)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	byID := recordsByID(records)

	jobs := make([]installer.Job, 0, len(ids))
	for _, id := range ids {
		record, ok := byID[id]
		if !ok {
			fmt.Fprintf(os.Stderr, "software id not found: %s\n", id)
			return 1
		}

		pkg := record.Package
		if len(pkg.InstallActions) == 0 {
			if pkg.ExternalActionURL != "" {
				fmt.Fprintf(os.Stderr, "%s cannot be installed directly by the CLI. Open: %s\n", pkg.ID, pkg.ExternalActionURL)
				return 1
			}
			fmt.Fprintf(os.Stderr, "%s has no install action configured.\n", pkg.ID)
			return 1
		}

		state := installer.Detect(pkg)
		if state.Installed && !*force && !*dryRun {
			fmt.Printf("%s is already installed. Use --force to run the installer anyway.\n", pkg.ID)
			continue
		}

		jobs = append(jobs, installer.Job{Package: pkg, Mode: installer.ModeInstall})
	}

	if len(jobs) == 0 {
		return 0
	}

	if *dryRun {
		for _, job := range jobs {
			fmt.Printf("%s:\n", job.Package.ID)
			for _, action := range job.Package.InstallActions {
				fmt.Printf("  %s\n", action.Title)
				fmt.Printf("    %s\n", shellCommand(action.Exec))
			}
		}
		return 0
	}

	results := installer.Run(jobs)
	for _, result := range results {
		if !result.Success {
			return 1
		}
	}
	return 0
}

func runInstallFromFiles(indexOverride string, dryRun bool, force bool, localFiles []string, ids []string) int {
	if len(ids) > 1 {
		fmt.Fprintln(os.Stderr, "usage: caracal install --from-file <archive> [id]")
		return 2
	}

	records, err := loadRecords(indexOverride)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	byID := recordsByID(records)

	jobs := make([]installer.Job, 0, len(localFiles))
	for _, rawPath := range localFiles {
		localPath, err := resolveLocalInstallFile(rawPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}

		var record catalogRecord
		if len(ids) == 1 {
			var ok bool
			record, ok = byID[ids[0]]
			if !ok {
				fmt.Fprintf(os.Stderr, "software id not found: %s\n", ids[0])
				return 1
			}
		} else {
			record, err = matchLocalInstallRecord(records, localPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
		}

		pkg, err := localInstallPackage(record, localPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}

		state := installer.Detect(pkg)
		if state.Installed && !force && !dryRun {
			fmt.Printf("%s is already installed. Use --force to run the installer anyway.\n", pkg.ID)
			continue
		}

		jobs = append(jobs, installer.Job{Package: pkg, Mode: installer.ModeInstall})
	}

	if len(jobs) == 0 {
		return 0
	}

	if dryRun {
		for _, job := range jobs {
			fmt.Printf("%s:\n", job.Package.ID)
			for _, action := range job.Package.InstallActions {
				fmt.Printf("  %s\n", action.Title)
				fmt.Printf("    %s\n", shellCommand(action.Exec))
			}
		}
		return 0
	}

	results := installer.Run(jobs)
	for _, result := range results {
		if !result.Success {
			return 1
		}
	}
	return 0
}

func runScan(indexOverride string, args []string) int {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	timeout := fs.Duration("timeout", 20*time.Second, "maximum time for each URL probe")
	quiet := fs.Bool("quiet", false, "hide per-link progress output")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	indexPath, err := resolveIndexPath(indexOverride)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if _, err := downloadindex.Validate(indexPath); err != nil {
		fmt.Fprintf(os.Stderr, "[error] %v\n", err)
		return 1
	}

	progress := os.Stdout
	if *quiet {
		progress = nil
	}

	failures, checked, err := downloadindex.CheckURLs(indexPath, *timeout, scanProgress(progress))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[error] %v\n", err)
		return 1
	}

	if len(failures) > 0 {
		for _, failure := range failures {
			fmt.Fprintf(os.Stderr, "%s %s %s: %s\n", colorize("[broken]", ansiRed), failure.PackageID, failure.Field, failure.URL)
			fmt.Fprintln(os.Stderr, failure.Err)
		}
		fmt.Fprintf(os.Stderr, "%s Found %d broken install link(s).\n", colorize("[fail]", ansiRed), len(failures))
		return 1
	}

	fmt.Printf("%s Scanned %d software entries; all install links responded.\n", colorize("[ok]", ansiGreen), checked)
	return 0
}

const (
	ansiReset = "\033[0m"
	ansiGreen = "\033[32m"
	ansiRed   = "\033[31m"
	ansiCyan  = "\033[36m"
)

func scanProgress(output *os.File) func(downloadindex.URLCheckEvent) {
	if output == nil {
		return nil
	}

	spinner := []byte{'|', '/', '-', '\\'}
	frame := 0
	return func(event downloadindex.URLCheckEvent) {
		switch event.Status {
		case downloadindex.URLCheckChecking:
			fmt.Fprintf(
				output,
				"\r\033[2K%s %c %-28s %-15s %s",
				colorize("[scan]", ansiCyan),
				spinner[frame%len(spinner)],
				event.PackageID,
				event.Field,
				event.URL,
			)
			frame++
		case downloadindex.URLCheckPassed:
			fmt.Fprintf(
				output,
				"\r\033[2K%s %-28s %-15s %s\n",
				colorize("[ok]", ansiGreen),
				event.PackageID,
				event.Field,
				event.URL,
			)
		case downloadindex.URLCheckFailed:
			fmt.Fprintf(
				output,
				"\r\033[2K%s %-28s %-15s %s\n",
				colorize("[fail]", ansiRed),
				event.PackageID,
				event.Field,
				event.URL,
			)
		}
	}
}

func colorize(value string, color string) string {
	if color == "" || os.Getenv("NO_COLOR") != "" {
		return value
	}
	return color + value + ansiReset
}

func runLaunch(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: caracal launch")
		return 2
	}

	cmd := exec.Command("caracal-software-installer-gui")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "launch failed: %v\n", err)
		return 1
	}
	return 0
}

func runList(indexOverride string, args []string) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var tags stringList
	var formats stringList
	fs.Var(&tags, "tag", "filter by tag/category; can be repeated or comma-separated")
	fs.Var(&formats, "format", "filter by format/type such as standalone, clap, vst, or lv2")
	verbose := fs.Bool("verbose", false, "include names, categories, and formats")
	openSource := fs.Bool("open-source", false, "only show open-source software")
	free := fs.Bool("free", false, "only show software with a free version")
	installed := fs.Bool("installed", false, "only show installed software")
	daws := fs.Bool("daws", false, "only show DAWs")
	effects := fs.Bool("effects", false, "only show effects")
	instruments := fs.Bool("instruments", false, "only show virtual instruments")
	utilities := fs.Bool("utilities", false, "only show utilities")
	synths := fs.Bool("synths", false, "only show synths")
	drums := fs.Bool("drums", false, "only show drums and percussion")
	samplers := fs.Bool("samplers", false, "only show samplers and sample players")
	guitar := fs.Bool("guitar", false, "only show guitar and amp tools")
	mixing := fs.Bool("mixing", false, "only show mixing tools")
	reverb := fs.Bool("reverb", false, "only show reverb and spatial tools")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: caracal list [filters]")
		return 2
	}

	if *daws {
		tags = append(tags, "daws")
	}
	if *effects {
		tags = append(tags, "effects")
	}
	if *instruments {
		tags = append(tags, "virtual-instruments")
	}
	if *utilities {
		tags = append(tags, "utilities")
	}
	if *synths {
		tags = append(tags, "synth")
	}
	if *drums {
		tags = append(tags, "drums")
	}
	if *samplers {
		tags = append(tags, "sampler")
	}
	if *guitar {
		tags = append(tags, "guitar")
	}
	if *mixing {
		tags = append(tags, "mixing")
	}
	if *reverb {
		tags = append(tags, "reverb")
	}

	records, err := loadRecords(indexOverride)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	sort.Slice(records, func(i int, j int) bool {
		return records[i].Package.ID < records[j].Package.ID
	})

	for _, record := range records {
		if !matchesListFilters(record, tags, formats, *openSource, *free, *installed) {
			continue
		}
		if *verbose {
			state := "not installed"
			if installer.Detect(record.Package).Installed {
				state = "installed"
			}
			fmt.Printf("%-28s %-28s %-20s %-24s %s\n", record.Package.ID, record.Package.Name, record.CategoryName, strings.Join(record.Package.SoftwareTypes, ","), state)
			continue
		}
		fmt.Println(record.Package.ID)
	}

	return 0
}

func loadRecords(indexOverride string) ([]catalogRecord, error) {
	scriptDir, err := bootstrap.ResolveScriptDir()
	if err != nil {
		return nil, err
	}

	indexPath, err := resolveIndexPathWithScriptDir(indexOverride, scriptDir)
	if err != nil {
		return nil, err
	}

	lookup, err := downloadindex.Load(indexPath)
	if err != nil {
		return nil, err
	}

	records := make([]catalogRecord, 0, len(lookup))
	seen := make(map[string]struct{}, len(lookup))
	for _, category := range catalog.Build(scriptDir, lookup) {
		for _, subcategory := range category.Subcategories {
			for _, pkg := range subcategory.Packages {
				seen[pkg.ID] = struct{}{}
				records = append(records, catalogRecord{
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

		records = append(records, catalogRecord{
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

func resolveIndexPath(indexOverride string) (string, error) {
	if indexOverride != "" {
		return indexOverride, nil
	}

	scriptDir, err := bootstrap.ResolveScriptDir()
	if err != nil {
		return "", err
	}
	return resolveIndexPathWithScriptDir("", scriptDir)
}

func resolveIndexPathWithScriptDir(indexOverride string, scriptDir string) (string, error) {
	if indexOverride != "" {
		return indexOverride, nil
	}
	return bootstrap.ResolveDownloadIndexPath(scriptDir)
}

func recordsByID(records []catalogRecord) map[string]catalogRecord {
	byID := make(map[string]catalogRecord, len(records))
	for _, record := range records {
		byID[record.Package.ID] = record
	}
	return byID
}

func matchesListFilters(record catalogRecord, tags []string, formats []string, openSource bool, free bool, installed bool) bool {
	if openSource && !record.Package.OpenSource {
		return false
	}
	if free && !record.Package.HasFreeVersion {
		return false
	}
	if installed && !installer.Detect(record.Package).Installed {
		return false
	}
	if len(formats) > 0 && !hasAny(record.Package.SoftwareTypes, formats) {
		return false
	}
	if len(tags) > 0 && !recordHasAnyTag(record, tags) {
		return false
	}
	return true
}

func recordHasAnyTag(record catalogRecord, tags []string) bool {
	recordTags := map[string]struct{}{}
	addTagValues(recordTags, record.CategoryID, record.CategoryName, record.SubcategoryID, record.SubcategoryName, record.Index["category"])
	addTagValues(recordTags, record.Package.SoftwareTypes...)

	for _, tag := range tags {
		if _, ok := recordTags[normalizeTag(tag)]; ok {
			return true
		}
	}
	return false
}

func addTagValues(tags map[string]struct{}, values ...string) {
	for _, value := range values {
		normalized := normalizeTag(value)
		if normalized == "" {
			continue
		}
		tags[normalized] = struct{}{}
		for _, part := range strings.Split(normalized, "-") {
			if part != "" {
				tags[part] = struct{}{}
			}
		}
	}
}

func hasAny(values []string, filters []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		seen[normalizeTag(value)] = struct{}{}
	}
	for _, filter := range filters {
		if _, ok := seen[normalizeTag(filter)]; ok {
			return true
		}
	}
	return false
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

func resolveLocalInstallFile(rawPath string) (string, error) {
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
	if !hasSupportedLocalInstallExtension(absPath) {
		return "", fmt.Errorf("unsupported local install file type: %s", absPath)
	}
	return absPath, nil
}

func hasSupportedLocalInstallExtension(path string) bool {
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

func matchLocalInstallRecord(records []catalogRecord, localPath string) (catalogRecord, error) {
	type scoredRecord struct {
		record catalogRecord
		score  int
	}

	matches := make([]scoredRecord, 0)
	for _, record := range records {
		score := localInstallMatchScore(record, localPath)
		if score > 0 {
			matches = append(matches, scoredRecord{record: record, score: score})
		}
	}

	if len(matches) == 0 {
		return catalogRecord{}, fmt.Errorf("could not match %s to a Caracal package; pass the package id explicitly, for example: caracal install --from-file %s reaper", filepath.Base(localPath), shellQuote(localPath))
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
		return catalogRecord{}, fmt.Errorf("could not choose one Caracal package for %s; matched: %s. Pass the package id explicitly", filepath.Base(localPath), strings.Join(ids, ", "))
	}

	return matches[0].record, nil
}

func localInstallMatchScore(record catalogRecord, localPath string) int {
	if !recordHasLocalInstallRoute(record) {
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

func recordHasLocalInstallRoute(record catalogRecord) bool {
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

func localInstallPackage(record catalogRecord, localPath string) (*catalog.Package, error) {
	scriptDir, err := bootstrap.ResolveScriptDir()
	if err != nil {
		return nil, err
	}

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

func shellCommand(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
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

func printHelp() {
	fmt.Println("Usage:")
	fmt.Println("  caracal [--index path] install [--dry-run] [--force] <id> [id...]")
	fmt.Println("  caracal [--index path] install [--dry-run] [--force] --from-file <archive> [id]")
	fmt.Println("  caracal [--index path] scan [--timeout 20s] [--quiet]")
	fmt.Println("  caracal launch")
	fmt.Println("  caracal [--index path] list [filters]")
	fmt.Println("  caracal help")
	fmt.Println()
	fmt.Println("List filters:")
	fmt.Println("  --daws --effects --instruments --utilities")
	fmt.Println("  --synths --drums --samplers --guitar --mixing --reverb")
	fmt.Println("  --tag <tag> --format <standalone|clap|vst|lv2>")
	fmt.Println("  --open-source --free --installed --verbose")
}
