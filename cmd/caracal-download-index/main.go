package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/caracal-os/caracal-software-installer/internal/downloadindex"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("caracal-download-index", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	indexPath := fs.String("index", defaultIndexPath(), "path to the CSV download index")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	rest := fs.Args()
	if len(rest) == 0 {
		usage()
		return 2
	}

	switch rest[0] {
	case "get":
		if len(rest) != 3 {
			usage()
			return 2
		}

		value, err := downloadindex.Get(*indexPath, rest[1], rest[2], false)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(value)
		return 0
	case "get-optional":
		if len(rest) != 3 {
			usage()
			return 2
		}

		value, err := downloadindex.Get(*indexPath, rest[1], rest[2], true)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(value)
		return 0
	case "validate":
		return runValidate(*indexPath, rest[1:])
	default:
		usage()
		return 2
	}
}

func runValidate(indexPath string, args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	checkURLs := fs.Bool("check-urls", false, "confirm that each download URL responds")
	timeout := fs.Duration("timeout", 20*time.Second, "maximum time for each URL probe")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	count, err := downloadindex.Validate(indexPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[error] %v\n", err)
		return 1
	}

	if !*checkURLs {
		fmt.Printf("Validated %d download index entries.\n", count)
		return 0
	}

	failures, checked, err := downloadindex.CheckURLs(indexPath, *timeout, os.Stdout)
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

	fmt.Printf("Validated %d download index entries and confirmed all URLs responded.\n", checked)
	return 0
}

func defaultIndexPath() string {
	return "data/download-index.csv"
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  caracal-download-index [--index path] get <package-id> <field>")
	fmt.Fprintln(os.Stderr, "  caracal-download-index [--index path] get-optional <package-id> <field>")
	fmt.Fprintln(os.Stderr, "  caracal-download-index [--index path] validate [--check-urls] [--timeout 20s]")
}
