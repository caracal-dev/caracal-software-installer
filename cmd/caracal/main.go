package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/caracal-os/caracal-software-installer/internal/bootstrap"
	"github.com/caracal-os/caracal-software-installer/internal/catalog"
	"github.com/caracal-os/caracal-software-installer/internal/downloadindex"
	"github.com/caracal-os/caracal-software-installer/internal/installer"
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

	if err := fs.Parse(args); err != nil {
		return 2
	}

	ids := fs.Args()
	if len(ids) == 0 {
		fmt.Fprintln(os.Stderr, "usage: caracal install [--dry-run] [--force] <id> [id...]")
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

	failures, checked, err := downloadindex.CheckURLs(indexPath, *timeout, progress)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[error] %v\n", err)
		return 1
	}

	if len(failures) > 0 {
		for _, failure := range failures {
			fmt.Fprintf(os.Stderr, "[broken] %s %s: %s\n", failure.PackageID, failure.Field, failure.URL)
			fmt.Fprintln(os.Stderr, failure.Err)
		}
		fmt.Fprintf(os.Stderr, "Found %d broken link(s).\n", len(failures))
		return 1
	}

	fmt.Printf("Scanned %d software entries; all links responded.\n", checked)
	return 0
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
