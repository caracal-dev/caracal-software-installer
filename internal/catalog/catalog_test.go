package catalog

import (
	"slices"
	"testing"

	"github.com/caracal-dev/caracal-software-installer/internal/downloadindex"
)

func TestLinksForEntrySuppressesDownloadForProprietaryPackages(t *testing.T) {
	links := linksForEntry(downloadindex.Entry{
		"url":             "https://example.test/download.tar.gz",
		"project_website": "https://example.test/product",
		"repo_url":        "https://example.test/source",
		"open_source":     "false",
	})

	if len(links) != 1 {
		t.Fatalf("expected only the site link, got %#v", links)
	}
	if links[0].Label != "Site" || links[0].URL != "https://example.test/product" {
		t.Fatalf("unexpected proprietary links: %#v", links)
	}
}

func TestLinksForEntryKeepsDownloadAndSourceForOpenSourcePackages(t *testing.T) {
	links := linksForEntry(downloadindex.Entry{
		"url":             "https://example.test/download.tar.gz",
		"project_website": "https://example.test/product",
		"repo_url":        "https://example.test/source",
		"open_source":     "true",
	})

	labels := make([]string, 0, len(links))
	for _, link := range links {
		labels = append(labels, link.Label)
	}

	for _, want := range []string{"Site", "Download", "Source"} {
		if !slices.Contains(labels, want) {
			t.Fatalf("expected %q in labels %#v", want, labels)
		}
	}
}

func TestLicenseForEntryNormalizesGPLAliases(t *testing.T) {
	for raw, want := range map[string]string{
		"GPL":  "GPL-3.0",
		"GPL3": "GPL-3.0",
		"GPL2": "GPL-2.0",
	} {
		license := licenseForEntry(downloadindex.Entry{
			"license_type":    raw,
			"link_to_license": "https://example.test/license",
		}, true)
		if license == nil {
			t.Fatalf("expected license for %q", raw)
		}
		if license.Label != want {
			t.Fatalf("expected %q to normalize to %q, got %q", raw, want, license.Label)
		}
		if license.URL != "https://example.test/license" {
			t.Fatalf("unexpected license URL %q", license.URL)
		}
	}
}
