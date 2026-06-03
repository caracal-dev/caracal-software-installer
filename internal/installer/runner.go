package installer

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/caracal-dev/caracal-software-installer/internal/catalog"
)

type PackageState struct {
	Installed          bool
	InstallAvailable   bool
	UninstallAvailable bool
}

type Mode string

const (
	ModeInstall   Mode = "install"
	ModeUninstall Mode = "uninstall"
)

type Job struct {
	Package *catalog.Package
	Mode    Mode
}

type Result struct {
	PackageID   string
	PackageName string
	Mode        Mode
	Success     bool
	Error       error
}

type RunOptions struct {
	Interactive         bool
	TransformActionExec func(job Job, action catalog.Action, execArgs []string) ([]string, error)
	OnJobStart          func(index int, total int, job Job)
	OnActionStart       func(job Job, action catalog.Action)
	OnActionOutput      func(job Job, action catalog.Action, stream string, text string)
}

func Detect(pkg *catalog.Package) PackageState {
	state := PackageState{}

	for _, marker := range pkg.InstalledMarkers {
		if markerExists(marker) {
			state.Installed = true
			break
		}
	}

	state.InstallAvailable = actionsAvailable(pkg.InstallActions)
	state.UninstallAvailable = actionsAvailable(pkg.UninstallActions)

	return state
}

func Run(jobs []Job) []Result {
	return RunWithOptions(jobs, RunOptions{Interactive: true})
}

func RunWithOptions(jobs []Job, opts RunOptions) []Result {
	results := make([]Result, 0, len(jobs))

	if opts.Interactive {
		fmt.Println("Caracal Software Installer")
		fmt.Println("==========================")
		fmt.Println()
	}

	for index, job := range jobs {
		pkg := job.Package
		if opts.Interactive {
			fmt.Printf("[%d/%d] %s (%s)\n", index+1, len(jobs), pkg.Name, strings.ToUpper(string(job.Mode)))
		}
		if opts.OnJobStart != nil {
			opts.OnJobStart(index+1, len(jobs), job)
		}

		actions := pkg.InstallActions
		if job.Mode == ModeUninstall {
			actions = pkg.UninstallActions
		}

		var runErr error
		for _, action := range actions {
			if opts.Interactive {
				fmt.Printf("  -> %s\n", action.Title)
			}
			if opts.OnActionStart != nil {
				opts.OnActionStart(job, action)
			}
			if err := runAction(job, action, opts); err != nil {
				runErr = err
				break
			}
		}

		result := Result{
			PackageID:   pkg.ID,
			PackageName: pkg.Name,
			Mode:        job.Mode,
			Success:     runErr == nil,
			Error:       runErr,
		}
		results = append(results, result)

		if runErr != nil {
			if opts.Interactive {
				fmt.Printf("  !! %v\n", runErr)
			}
		} else if opts.Interactive {
			fmt.Println("  OK")
		}

		if opts.Interactive {
			fmt.Println()
		}
	}

	return results
}

func runAction(job Job, action catalog.Action, opts RunOptions) error {
	execArgs := append([]string(nil), action.Exec...)
	if len(execArgs) == 0 {
		return fmt.Errorf("%s has no command configured", action.Title)
	}

	if opts.TransformActionExec != nil {
		transformed, err := opts.TransformActionExec(job, action, execArgs)
		if err != nil {
			return fmt.Errorf("%s failed: %w", action.Title, err)
		}
		execArgs = transformed
		if len(execArgs) == 0 {
			return fmt.Errorf("%s failed: transformed command was empty", action.Title)
		}
	}

	cmd := exec.Command(execArgs[0], execArgs[1:]...)

	if opts.Interactive {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("%s failed: %w", action.Title, err)
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return fmt.Errorf("%s failed: %w", action.Title, err)
		}

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("%s failed: %w", action.Title, err)
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go streamOutput(&wg, stdout, "stdout", job, action, opts.OnActionOutput)
		go streamOutput(&wg, stderr, "stderr", job, action, opts.OnActionOutput)
		wg.Wait()

		if err := cmd.Wait(); err != nil {
			return fmt.Errorf("%s failed: %w", action.Title, err)
		}
		return nil
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", action.Title, err)
	}

	return nil
}

func streamOutput(wg *sync.WaitGroup, reader io.Reader, stream string, job Job, action catalog.Action, emit func(job Job, action catalog.Action, stream string, text string)) {
	defer wg.Done()

	if emit == nil {
		_, _ = io.Copy(io.Discard, reader)
		return
	}

	scanner := bufio.NewScanner(reader)
	buffer := make([]byte, 0, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		emit(job, action, stream, scanner.Text())
	}
}

func actionsAvailable(actions []catalog.Action) bool {
	if len(actions) == 0 {
		return false
	}

	for _, action := range actions {
		if len(action.Exec) == 0 {
			return false
		}

		if !commandExists(action.Exec[0]) {
			return false
		}

		for _, arg := range action.Exec[1:] {
			if looksLikePath(arg) {
				if _, err := os.Stat(arg); err != nil {
					return false
				}
			}
		}
	}

	return true
}

func markerExists(marker string) bool {
	target := marker
	if !filepath.IsAbs(marker) {
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		target = filepath.Join(home, marker)
	}

	if strings.ContainsAny(target, "*?[") {
		matches, err := filepath.Glob(target)
		return err == nil && len(matches) > 0
	}

	_, err := os.Stat(target)
	return err == nil
}

func commandExists(command string) bool {
	if looksLikePath(command) {
		_, err := os.Stat(command)
		return err == nil
	}

	_, err := exec.LookPath(command)
	return err == nil
}

func looksLikePath(value string) bool {
	if strings.Contains(value, "://") {
		return false
	}

	return strings.Contains(value, string(os.PathSeparator)) || strings.HasPrefix(value, ".")
}
