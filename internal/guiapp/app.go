package guiapp

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/caracal-dev/caracal-software-installer/internal/bootstrap"
	"github.com/caracal-dev/caracal-software-installer/internal/catalog"
	"github.com/caracal-dev/caracal-software-installer/internal/installer"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx    context.Context
	loaded *bootstrap.Resolved

	mu      sync.Mutex
	running bool
}

type CatalogPayload struct {
	Logo       string         `json:"logo"`
	Categories []CategoryView `json:"categories"`
}

type IconSettingsPayload struct {
	Icons    []AppIconView `json:"icons"`
	ActiveID string        `json:"activeId"`
}

type AppIconView struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Default bool   `json:"default"`
	Active  bool   `json:"active"`
}

type CategoryView struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Accent        string            `json:"accent"`
	Subcategories []SubcategoryView `json:"subcategories"`
}

type SubcategoryView struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Packages    []PackageView `json:"packages"`
}

type PackageView struct {
	ID                string           `json:"id"`
	Name              string           `json:"name"`
	Vendor            string           `json:"vendor"`
	CategoryName      string           `json:"categoryName"`
	SubcategoryName   string           `json:"subcategoryName"`
	Summary           string           `json:"summary"`
	Description       string           `json:"description"`
	Notes             []string         `json:"notes"`
	Links             []LinkView       `json:"links"`
	SoftwareTypes     []string         `json:"softwareTypes"`
	AvailabilityNote  string           `json:"availabilityNote"`
	OpenSource        bool             `json:"openSource"`
	HasFreeVersion    bool             `json:"hasFreeVersion"`
	ExternalActionURL string           `json:"externalActionUrl"`
	InstallActions    []ActionView     `json:"installActions"`
	UninstallActions  []ActionView     `json:"uninstallActions"`
	State             PackageStateView `json:"state"`
}

type PackageStateView struct {
	Installed          bool   `json:"installed"`
	InstallAvailable   bool   `json:"installAvailable"`
	UninstallAvailable bool   `json:"uninstallAvailable"`
	Actionable         bool   `json:"actionable"`
	ActionKind         string `json:"actionKind"`
	ActionURL          string `json:"actionUrl"`
	Mode               string `json:"mode"`
	StatusLabel        string `json:"statusLabel"`
	ActionLabel        string `json:"actionLabel"`
}

type LinkView struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type ActionView struct {
	Title string `json:"title"`
}

type JobEvent struct {
	Index       int    `json:"index"`
	Total       int    `json:"total"`
	PackageID   string `json:"packageId"`
	PackageName string `json:"packageName"`
	Mode        string `json:"mode"`
}

type ActionEvent struct {
	PackageID   string `json:"packageId"`
	PackageName string `json:"packageName"`
	Mode        string `json:"mode"`
	Title       string `json:"title"`
}

type OutputEvent struct {
	PackageID   string `json:"packageId"`
	PackageName string `json:"packageName"`
	Mode        string `json:"mode"`
	Title       string `json:"title"`
	Stream      string `json:"stream"`
	Message     string `json:"message"`
}

type ResultView struct {
	PackageID   string `json:"packageId"`
	PackageName string `json:"packageName"`
	Mode        string `json:"mode"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
}

type RunStartedEvent struct {
	Jobs    []JobEvent `json:"jobs"`
	Skipped []string   `json:"skipped"`
}

type RunFinishedEvent struct {
	Results []ResultView   `json:"results"`
	Catalog CatalogPayload `json:"catalog"`
}

func New(loaded *bootstrap.Resolved) *App {
	return &App{loaded: loaded}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetCatalog() CatalogPayload {
	return a.catalogPayload()
}

func (a *App) RefreshCatalog() CatalogPayload {
	return a.catalogPayload()
}

func (a *App) GetIconSettings() (IconSettingsPayload, error) {
	return loadIconSettings()
}

func (a *App) SetDesktopIcon(iconID string) (IconSettingsPayload, error) {
	requested := strings.TrimSpace(iconID)
	if requested == "" {
		requested = "appicon.png"
	}
	iconID = filepath.Base(requested)
	if iconID != requested || filepath.Ext(iconID) != ".png" {
		return IconSettingsPayload{}, fmt.Errorf("invalid icon selection: %s", requested)
	}

	paths, err := resolveAppIconPaths()
	if err != nil {
		return IconSettingsPayload{}, err
	}

	source := filepath.Join(paths.iconDir, iconID)
	if !isPathInside(paths.iconDir, source) {
		return IconSettingsPayload{}, fmt.Errorf("invalid icon selection: %s", iconID)
	}
	if _, err := os.Stat(source); err != nil {
		return IconSettingsPayload{}, fmt.Errorf("icon not found: %s", iconID)
	}

	if err := copyFile(source, paths.target); err != nil {
		return IconSettingsPayload{}, fmt.Errorf("could not apply desktop icon: %w", err)
	}
	refreshDesktopIconCache(paths.target)

	return loadIconSettings()
}

func (a *App) OpenLink(url string) error {
	if strings.TrimSpace(url) == "" {
		return fmt.Errorf("link URL is empty")
	}

	runtime.BrowserOpenURL(a.ctx, url)
	return nil
}

func (a *App) RunSelection(ids []string) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("an install run is already in progress")
	}

	jobs, skipped, err := a.resolveJobs(ids)
	if err != nil {
		a.mu.Unlock()
		return err
	}
	if len(jobs) == 0 {
		a.mu.Unlock()
		return fmt.Errorf("none of the selected packages are currently actionable")
	}

	a.running = true
	a.mu.Unlock()

	go a.run(jobs, skipped)
	return nil
}

func (a *App) catalogPayload() CatalogPayload {
	return CatalogPayload{
		Logo:       a.loaded.Logo,
		Categories: buildCategoryViews(a.loaded.Categories),
	}
}

func (a *App) resolveJobs(ids []string) ([]installer.Job, []string, error) {
	lookup := packageLookup(a.loaded.Categories)
	jobs := make([]installer.Job, 0, len(ids))
	skipped := make([]string, 0)
	seen := make(map[string]struct{}, len(ids))

	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		pkg, ok := lookup[id]
		if !ok {
			return nil, nil, fmt.Errorf("package id not found: %s", id)
		}

		state := installer.Detect(pkg)
		switch {
		case state.Installed && state.UninstallAvailable:
			jobs = append(jobs, installer.Job{Package: pkg, Mode: installer.ModeUninstall})
		case !state.Installed && state.InstallAvailable:
			jobs = append(jobs, installer.Job{Package: pkg, Mode: installer.ModeInstall})
		default:
			skipped = append(skipped, pkg.Name)
		}
	}

	return jobs, skipped, nil
}

func (a *App) run(jobs []installer.Job, skipped []string) {
	defer func() {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
	}()

	started := RunStartedEvent{
		Jobs:    make([]JobEvent, 0, len(jobs)),
		Skipped: skipped,
	}
	for index, job := range jobs {
		started.Jobs = append(started.Jobs, JobEvent{
			Index:       index + 1,
			Total:       len(jobs),
			PackageID:   job.Package.ID,
			PackageName: job.Package.Name,
			Mode:        string(job.Mode),
		})
	}
	runtime.EventsEmit(a.ctx, "installer:run-started", started)

	results := installer.RunWithOptions(jobs, installer.RunOptions{
		Interactive:         false,
		TransformActionExec: a.transformActionExec,
		OnJobStart: func(index int, total int, job installer.Job) {
			runtime.EventsEmit(a.ctx, "installer:job-started", JobEvent{
				Index:       index,
				Total:       total,
				PackageID:   job.Package.ID,
				PackageName: job.Package.Name,
				Mode:        string(job.Mode),
			})
		},
		OnActionStart: func(job installer.Job, action catalog.Action) {
			runtime.EventsEmit(a.ctx, "installer:action-started", ActionEvent{
				PackageID:   job.Package.ID,
				PackageName: job.Package.Name,
				Mode:        string(job.Mode),
				Title:       action.Title,
			})
		},
		OnActionOutput: func(job installer.Job, action catalog.Action, stream string, text string) {
			runtime.EventsEmit(a.ctx, "installer:action-output", OutputEvent{
				PackageID:   job.Package.ID,
				PackageName: job.Package.Name,
				Mode:        string(job.Mode),
				Title:       action.Title,
				Stream:      stream,
				Message:     text,
			})
		},
	})

	finished := RunFinishedEvent{
		Results: make([]ResultView, 0, len(results)),
		Catalog: a.catalogPayload(),
	}
	for _, result := range results {
		view := ResultView{
			PackageID:   result.PackageID,
			PackageName: result.PackageName,
			Mode:        string(result.Mode),
			Success:     result.Success,
		}
		if result.Error != nil {
			view.Error = result.Error.Error()
		}
		finished.Results = append(finished.Results, view)
	}

	runtime.EventsEmit(a.ctx, "installer:run-finished", finished)
}

func (a *App) transformActionExec(job installer.Job, _ catalog.Action, execArgs []string) ([]string, error) {
	if len(execArgs) == 0 || execArgs[0] != "sudo" {
		return execArgs, nil
	}

	pkexecPath, err := exec.LookPath("pkexec")
	if err != nil {
		return nil, fmt.Errorf("%s requires elevated privileges, but pkexec is not installed", job.Package.Name)
	}

	envPath, err := exec.LookPath("env")
	if err != nil {
		return nil, fmt.Errorf("could not locate env for pkexec wrapper: %w", err)
	}

	transformed := []string{pkexecPath, envPath}
	transformed = append(transformed, "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	transformed = append(transformed, desktopEnvironmentAssignments()...)
	if targetUser := currentDesktopUser(); targetUser != "" {
		transformed = append(transformed, "CARACAL_INSTALLER_TARGET_USER="+targetUser)
	}
	transformed = append(transformed, execArgs[1:]...)
	return transformed, nil
}

func desktopEnvironmentAssignments() []string {
	keys := []string{
		"DISPLAY",
		"WAYLAND_DISPLAY",
		"XAUTHORITY",
		"XDG_RUNTIME_DIR",
		"DBUS_SESSION_BUS_ADDRESS",
		"DESKTOP_SESSION",
		"XDG_CURRENT_DESKTOP",
	}
	assignments := make([]string, 0, len(keys))
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			assignments = append(assignments, key+"="+value)
		}
	}
	return assignments
}

func currentDesktopUser() string {
	if value := strings.TrimSpace(os.Getenv("CARACAL_INSTALLER_TARGET_USER")); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("SUDO_USER")); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("USER")); value != "" {
		return value
	}

	if current, err := user.Current(); err == nil {
		return strings.TrimSpace(current.Username)
	}

	return ""
}

type appIconPaths struct {
	iconDir string
	target  string
}

func loadIconSettings() (IconSettingsPayload, error) {
	paths, err := resolveAppIconPaths()
	if err != nil {
		return IconSettingsPayload{}, err
	}

	entries, err := os.ReadDir(paths.iconDir)
	if err != nil {
		return IconSettingsPayload{}, fmt.Errorf("could not read icon directory: %w", err)
	}

	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".png" {
			continue
		}
		ids = append(ids, entry.Name())
	}
	sort.Strings(ids)
	ids = prioritizeDefaultIcon(ids)
	if len(ids) == 0 {
		return IconSettingsPayload{}, fmt.Errorf("no PNG icons found in %s", paths.iconDir)
	}

	activeID := activeIconID(paths, ids)
	icons := make([]AppIconView, 0, len(ids))
	for _, id := range ids {
		icons = append(icons, AppIconView{
			ID:      id,
			Label:   iconLabel(id),
			Default: id == "appicon.png",
			Active:  id == activeID,
		})
	}

	return IconSettingsPayload{Icons: icons, ActiveID: activeID}, nil
}

func resolveAppIconPaths() (appIconPaths, error) {
	if envDir := strings.TrimSpace(os.Getenv("CARACAL_INSTALLER_ICON_DIR")); envDir != "" {
		iconDir := filepath.Clean(envDir)
		return appIconPaths{
			iconDir: iconDir,
			target:  resolveAppIconTarget(iconDir),
		}, nil
	}

	var candidates []string
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, candidateBuildIconDirs(wd)...)
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, candidateBuildIconDirs(filepath.Dir(exe))...)
	}
	candidates = append(candidates, "/usr/share/caracal-software-installer/build/icons")

	seen := make(map[string]struct{})
	for _, candidate := range candidates {
		iconDir := filepath.Clean(candidate)
		if _, ok := seen[iconDir]; ok {
			continue
		}
		seen[iconDir] = struct{}{}

		if info, err := os.Stat(iconDir); err == nil && info.IsDir() {
			return appIconPaths{
				iconDir: iconDir,
				target:  resolveAppIconTarget(iconDir),
			}, nil
		}
	}

	return appIconPaths{}, fmt.Errorf("could not find build/icons; set CARACAL_INSTALLER_ICON_DIR or run from the source tree")
}

func resolveAppIconTarget(iconDir string) string {
	if envTarget := strings.TrimSpace(os.Getenv("CARACAL_INSTALLER_ICON_TARGET")); envTarget != "" {
		return filepath.Clean(envTarget)
	}

	if strings.HasPrefix(filepath.Clean(iconDir), "/usr/share/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, ".local", "share", "icons", "hicolor", "256x256", "apps", "caracal-software-installer.png")
		}
	}

	return filepath.Join(filepath.Dir(iconDir), "appicon.png")
}

func candidateBuildIconDirs(start string) []string {
	var dirs []string
	for dir := filepath.Clean(start); ; dir = filepath.Dir(dir) {
		dirs = append(dirs, filepath.Join(dir, "build", "icons"))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return dirs
}

func prioritizeDefaultIcon(ids []string) []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "appicon.png" {
			result = append(result, id)
			break
		}
	}
	for _, id := range ids {
		if id != "appicon.png" {
			result = append(result, id)
		}
	}
	return result
}

func activeIconID(paths appIconPaths, ids []string) string {
	targetHash, err := fileSHA256(paths.target)
	if err != nil {
		return "appicon.png"
	}
	for _, id := range ids {
		iconHash, err := fileSHA256(filepath.Join(paths.iconDir, id))
		if err == nil && iconHash == targetHash {
			return id
		}
	}
	return "appicon.png"
}

func iconLabel(id string) string {
	if id == "appicon.png" {
		return "Default (appicon.png)"
	}
	name := strings.TrimSuffix(id, filepath.Ext(id))
	name = strings.TrimPrefix(name, "caracal-")
	words := strings.Fields(strings.ReplaceAll(name, "-", " "))
	for index, word := range words {
		if word == "" {
			continue
		}
		words[index] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ") + " (" + id + ")"
}

func fileSHA256(path string) (string, error) {
	file, err := openRegularFile(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func copyFile(source string, target string) error {
	cleanTarget, err := cleanWritablePNGPath(target)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(cleanTarget), 0o755); err != nil {
		return err
	}

	sourceFile, err := openRegularFile(source)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// #nosec G304 -- cleanWritablePNGPath rejects root/current-dir paths and requires a PNG target.
	targetFile, err := os.OpenFile(cleanTarget, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	_, err = io.Copy(targetFile, sourceFile)
	return err
}

func openRegularFile(path string) (*os.File, error) {
	clean, err := cleanExistingRegularFilePath(path)
	if err != nil {
		return nil, err
	}
	// #nosec G304 -- cleanExistingRegularFilePath rejects unsafe paths and requires an existing regular file.
	return os.Open(clean)
}

func cleanExistingRegularFilePath(path string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "." || clean == string(filepath.Separator) {
		return "", fmt.Errorf("refusing to open unsafe path %q", path)
	}

	info, err := os.Stat(clean)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", &fs.PathError{Op: "open", Path: clean, Err: fs.ErrInvalid}
	}
	return clean, nil
}

func cleanWritablePNGPath(path string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "." || clean == string(filepath.Separator) || filepath.Ext(clean) != ".png" {
		return "", fmt.Errorf("refusing to write unsafe icon path %q", path)
	}
	return clean, nil
}

func refreshDesktopIconCache(target string) {
	hicolorRoot := filepath.Clean(filepath.Join(filepath.Dir(target), "..", "..", ".."))
	if filepath.Base(hicolorRoot) != "hicolor" {
		return
	}

	if _, err := exec.LookPath("gtk-update-icon-cache"); err == nil {
		_ = exec.Command("gtk-update-icon-cache", "-q", "-t", "-f", hicolorRoot).Run()
	}
}

func isPathInside(parent string, child string) bool {
	parent, err := filepath.Abs(parent)
	if err != nil {
		return false
	}
	child, err = filepath.Abs(child)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func packageLookup(categories []*catalog.Category) map[string]*catalog.Package {
	lookup := make(map[string]*catalog.Package)
	for _, category := range categories {
		for _, subcategory := range category.Subcategories {
			for _, pkg := range subcategory.Packages {
				lookup[pkg.ID] = pkg
			}
		}
	}
	return lookup
}

func buildCategoryViews(categories []*catalog.Category) []CategoryView {
	views := make([]CategoryView, 0, len(categories))
	for _, category := range categories {
		view := CategoryView{
			ID:            category.ID,
			Name:          category.Name,
			Description:   category.Description,
			Accent:        category.Accent,
			Subcategories: make([]SubcategoryView, 0, len(category.Subcategories)),
		}

		for _, subcategory := range category.Subcategories {
			subView := SubcategoryView{
				ID:          subcategory.ID,
				Name:        subcategory.Name,
				Description: subcategory.Description,
				Packages:    make([]PackageView, 0, len(subcategory.Packages)),
			}

			for _, pkg := range subcategory.Packages {
				state := installer.Detect(pkg)
				subView.Packages = append(subView.Packages, PackageView{
					ID:                pkg.ID,
					Name:              pkg.Name,
					Vendor:            pkg.Vendor,
					CategoryName:      category.Name,
					SubcategoryName:   subcategory.Name,
					Summary:           pkg.Summary,
					Description:       pkg.Description,
					Notes:             append([]string(nil), pkg.Notes...),
					Links:             buildLinks(pkg.Links),
					SoftwareTypes:     append([]string(nil), pkg.SoftwareTypes...),
					OpenSource:        pkg.OpenSource,
					HasFreeVersion:    pkg.HasFreeVersion,
					ExternalActionURL: pkg.ExternalActionURL,
					AvailabilityNote:  pkg.AvailabilityNote,
					InstallActions:    buildActions(pkg.InstallActions),
					UninstallActions:  buildActions(pkg.UninstallActions),
					State:             buildPackageStateView(pkg, state),
				})
			}

			view.Subcategories = append(view.Subcategories, subView)
		}

		views = append(views, view)
	}

	return views
}

func buildLinks(links []catalog.Link) []LinkView {
	views := make([]LinkView, 0, len(links))
	for _, link := range links {
		views = append(views, LinkView{
			Label: link.Label,
			URL:   link.URL,
		})
	}
	return views
}

func buildActions(actions []catalog.Action) []ActionView {
	views := make([]ActionView, 0, len(actions))
	for _, action := range actions {
		views = append(views, ActionView{Title: action.Title})
	}
	return views
}

func buildPackageStateView(pkg *catalog.Package, state installer.PackageState) PackageStateView {
	view := PackageStateView{
		Installed:          state.Installed,
		InstallAvailable:   state.InstallAvailable,
		UninstallAvailable: state.UninstallAvailable,
		Actionable:         false,
		ActionKind:         "none",
		StatusLabel:        "Catalog only",
		ActionLabel:        "Unavailable",
	}

	switch {
	case state.Installed && state.UninstallAvailable:
		view.Actionable = true
		view.ActionKind = "uninstall"
		view.Mode = string(installer.ModeUninstall)
		view.StatusLabel = "Installed"
		view.ActionLabel = "Uninstall"
	case state.Installed:
		view.StatusLabel = "Installed"
	case pkg.ExternalActionURL != "":
		view.Actionable = true
		view.ActionKind = "link"
		view.ActionURL = pkg.ExternalActionURL
		view.StatusLabel = "Available on site"
		view.ActionLabel = "Get From Site"
	case state.InstallAvailable:
		view.Actionable = true
		view.ActionKind = "install"
		view.Mode = string(installer.ModeInstall)
		view.StatusLabel = "Available"
		view.ActionLabel = "Queue install"
	default:
		view.StatusLabel = "Catalog only"
	}

	return view
}
