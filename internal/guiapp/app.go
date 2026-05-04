package guiapp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"sync"

	"github.com/caracal-os/caracal-software-installer/internal/bootstrap"
	"github.com/caracal-os/caracal-software-installer/internal/catalog"
	"github.com/caracal-os/caracal-software-installer/internal/installer"
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
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Vendor           string           `json:"vendor"`
	CategoryName     string           `json:"categoryName"`
	SubcategoryName  string           `json:"subcategoryName"`
	Summary          string           `json:"summary"`
	Description      string           `json:"description"`
	Notes            []string         `json:"notes"`
	Links            []LinkView       `json:"links"`
	AvailabilityNote string           `json:"availabilityNote"`
	InstallActions   []ActionView     `json:"installActions"`
	UninstallActions []ActionView     `json:"uninstallActions"`
	State            PackageStateView `json:"state"`
}

type PackageStateView struct {
	Installed          bool   `json:"installed"`
	InstallAvailable   bool   `json:"installAvailable"`
	UninstallAvailable bool   `json:"uninstallAvailable"`
	Actionable         bool   `json:"actionable"`
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
	if targetUser := currentDesktopUser(); targetUser != "" {
		transformed = append(transformed, "CARACAL_INSTALLER_TARGET_USER="+targetUser)
	}
	transformed = append(transformed, execArgs[1:]...)
	return transformed, nil
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
					ID:               pkg.ID,
					Name:             pkg.Name,
					Vendor:           pkg.Vendor,
					CategoryName:     category.Name,
					SubcategoryName:  subcategory.Name,
					Summary:          pkg.Summary,
					Description:      pkg.Description,
					Notes:            append([]string(nil), pkg.Notes...),
					Links:            buildLinks(pkg.Links),
					AvailabilityNote: pkg.AvailabilityNote,
					InstallActions:   buildActions(pkg.InstallActions),
					UninstallActions: buildActions(pkg.UninstallActions),
					State:            buildPackageStateView(state),
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

func buildPackageStateView(state installer.PackageState) PackageStateView {
	view := PackageStateView{
		Installed:          state.Installed,
		InstallAvailable:   state.InstallAvailable,
		UninstallAvailable: state.UninstallAvailable,
		Actionable:         false,
		StatusLabel:        "Catalog only",
		ActionLabel:        "Unavailable",
	}

	switch {
	case state.Installed && state.UninstallAvailable:
		view.Actionable = true
		view.Mode = string(installer.ModeUninstall)
		view.StatusLabel = "Installed"
		view.ActionLabel = "Queue uninstall"
	case state.Installed:
		view.StatusLabel = "Installed"
	case state.InstallAvailable:
		view.Actionable = true
		view.Mode = string(installer.ModeInstall)
		view.StatusLabel = "Available"
		view.ActionLabel = "Queue install"
	default:
		view.StatusLabel = "Catalog only"
	}

	return view
}
