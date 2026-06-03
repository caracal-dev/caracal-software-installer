package main

import (
	"encoding/csv"
	"log"
	"os"

	"github.com/caracal-dev/caracal-software-installer/internal/catalog"
	"github.com/caracal-dev/caracal-software-installer/internal/downloadindex"
)

func main() {
	lookup, err := downloadindex.Load("data/download-index.csv")
	if err != nil {
		log.Fatal(err)
	}

	writer := csv.NewWriter(os.Stdout)

	if err := writer.Write([]string{"category_id", "category_name", "subcategory_id", "subcategory_name", "package_id", "package_name", "vendor", "link_label", "url"}); err != nil {
		log.Fatal(err)
	}

	for _, categoryEntry := range catalog.Build("", lookup) {
		for _, subcategoryEntry := range categoryEntry.Subcategories {
			for _, pkg := range subcategoryEntry.Packages {
				for _, link := range pkg.Links {
					record := []string{
						categoryEntry.ID,
						categoryEntry.Name,
						subcategoryEntry.ID,
						subcategoryEntry.Name,
						pkg.ID,
						pkg.Name,
						pkg.Vendor,
						link.Label,
						link.URL,
					}
					if err := writer.Write(record); err != nil {
						log.Fatal(err)
					}
				}
			}
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Fatal(err)
	}
}
