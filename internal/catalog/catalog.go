package catalog

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/caracal-dev/caracal-software-installer/internal/downloadindex"
)

type Action struct {
	Title string
	Exec  []string
}

type Link struct {
	Label string
	URL   string
}

type License struct {
	Label string
	URL   string
	Kind  string
}

type Package struct {
	ID                string
	Name              string
	Vendor            string
	Summary           string
	Description       string
	Notes             []string
	Links             []Link
	ExternalActionURL string
	SoftwareTypes     []string
	OpenSource        bool
	HasFreeVersion    bool
	License           *License
	AvailabilityNote  string
	InstalledMarkers  []string
	InstallActions    []Action
	UninstallActions  []Action
}

type Subcategory struct {
	ID          string
	Name        string
	Description string
	Packages    []*Package
}

type Category struct {
	ID            string
	Name          string
	Description   string
	Accent        string
	Subcategories []*Subcategory
}

func Build(scriptDir string, downloadLookup map[string]downloadindex.Entry) []*Category {
	script := func(name string, args ...string) []string {
		exec := []string{"bash", filepath.Join(scriptDir, name)}
		return append(exec, args...)
	}
	sudoScript := func(name string) []string {
		return []string{"sudo", "bash", filepath.Join(scriptDir, name)}
	}
	mustEntry := func(id string) downloadindex.Entry {
		entry, ok := downloadLookup[id]
		if !ok {
			panic(fmt.Sprintf("download index entry not found for package id %s", id))
		}
		return entry
	}
	trimTrailingEmpty := func(values []string) []string {
		last := len(values) - 1
		for last >= 0 && values[last] == "" {
			last--
		}
		return values[:last+1]
	}
	archiveInstall := func(id string) []string {
		entry := mustEntry(id)
		args := trimTrailingEmpty([]string{
			id,
			entry["name"],
			entry["url"],
			entry["primary_bundle_name"],
			entry["formats"],
			entry["data_dir_name"],
			entry["data_target_name"],
		})
		return append([]string{"bash", filepath.Join(scriptDir, "install-plugin-archive.sh")}, args...)
	}
	archiveUninstall := func(id string) []string {
		entry := mustEntry(id)
		args := trimTrailingEmpty([]string{
			id,
			entry["primary_bundle_name"],
			entry["formats"],
			entry["data_target_name"],
		})
		return append([]string{"bash", filepath.Join(scriptDir, "uninstall-plugin-archive.sh")}, args...)
	}
	sourceInstall := func(id string) []string {
		return script("install-source-plugin.sh", id, mustEntry(id)["name"])
	}
	sourceUninstall := func(projectName string, displayName string) []string {
		return script("uninstall-source-plugin.sh", projectName, displayName)
	}
	splitFormats := func(raw string) []string {
		if raw == "" {
			return nil
		}

		parts := strings.Split(raw, ",")
		formats := make([]string, 0, len(parts))
		for _, part := range parts {
			format := strings.TrimSpace(part)
			if format == "" {
				continue
			}
			formats = append(formats, format)
		}
		return formats
	}
	formatLabel := func(format string) string {
		switch format {
		case "clap":
			return "CLAP"
		case "vst":
			return "VST2"
		case "vst3":
			return "VST3"
		case "lv2":
			return "LV2"
		default:
			return strings.ToUpper(format)
		}
	}
	addSoftwareType := func(seen map[string]struct{}, ordered *[]string, kind string) {
		if kind == "" {
			return
		}
		if _, ok := seen[kind]; ok {
			return
		}
		seen[kind] = struct{}{}
		*ordered = append(*ordered, kind)
	}
	joinLabels := func(values []string) string {
		switch len(values) {
		case 0:
			return ""
		case 1:
			return values[0]
		case 2:
			return values[0] + " and " + values[1]
		default:
			return strings.Join(values[:len(values)-1], ", ") + ", and " + values[len(values)-1]
		}
	}
	archiveTargets := func(id string) string {
		entry := mustEntry(id)
		formats := splitFormats(entry["formats"])
		if len(formats) == 0 {
			return "plugin payloads"
		}

		labels := make([]string, 0, len(formats))
		for _, format := range formats {
			labels = append(labels, formatLabel(format))
		}

		suffix := " plugin targets"
		if len(labels) == 1 {
			suffix = " plugin target"
		}

		return joinLabels(labels) + suffix
	}
	archiveInstalledMarkers := func(id string) []string {
		entry := mustEntry(id)
		primaryBundleName := entry["primary_bundle_name"]
		formats := splitFormats(entry["formats"])
		markers := make([]string, 0, len(formats)+2)
		markers = append(markers, ".local/share/caracal-software-installer/manifests/"+id+".txt")

		if primaryBundleName != "" {
			for _, format := range formats {
				switch format {
				case "clap":
					markers = append(markers, ".clap/"+primaryBundleName+".clap")
				case "vst":
					markers = append(markers, ".vst/"+primaryBundleName+".so")
				case "vst3":
					markers = append(markers, ".vst3/"+primaryBundleName+".vst3")
				case "lv2":
					markers = append(markers, ".lv2/"+primaryBundleName+".lv2")
				}
			}
		}

		dataTargetName := entry["data_target_name"]
		if dataTargetName == "" {
			dataTargetName = entry["data_dir_name"]
		}
		if dataTargetName != "" {
			markers = append(markers, "Audio Assault/PluginData/Audio Assault/"+dataTargetName)
		}

		return markers
	}
	linkForID := func(id string) []Link {
		return linksForEntry(mustEntry(id))
	}
	downloadWithinApp := func(id string) bool {
		value := strings.TrimSpace(mustEntry(id)["dl_within_app"])
		if value == "" {
			return true
		}
		return !strings.EqualFold(value, "false")
	}
	externalActionURLForID := func(id string) string {
		entry := mustEntry(id)
		if entry["url"] != "" {
			return entry["url"]
		}
		return entry["project_website"]
	}
	boolFieldForID := func(id string, field string, defaultValue bool) bool {
		return boolField(mustEntry(id), field, defaultValue)
	}
	softwareTypesForPackage := func(pkg *Package) []string {
		entry := mustEntry(pkg.ID)
		seen := make(map[string]struct{}, 4)
		ordered := make([]string, 0, 4)

		for _, format := range splitFormats(entry["formats"]) {
			switch format {
			case "clap":
				addSoftwareType(seen, &ordered, "clap")
			case "vst", "vst3":
				addSoftwareType(seen, &ordered, "vst")
			case "lv2":
				addSoftwareType(seen, &ordered, "lv2")
			}
		}

		for _, marker := range pkg.InstalledMarkers {
			switch {
			case strings.Contains(marker, ".clap") || strings.Contains(marker, "/clap/"):
				addSoftwareType(seen, &ordered, "clap")
			case strings.Contains(marker, ".vst3") || strings.Contains(marker, ".vst/") || strings.Contains(marker, "/vst/"):
				addSoftwareType(seen, &ordered, "vst")
			case strings.Contains(marker, ".lv2") || strings.Contains(marker, "/lv2/"):
				addSoftwareType(seen, &ordered, "lv2")
			}
			if strings.Contains(marker, "/bin/") || strings.Contains(marker, ".desktop") {
				addSoftwareType(seen, &ordered, "standalone")
			}
		}

		switch pkg.ID {
		case "cardinal", "surge-xt":
			addSoftwareType(seen, &ordered, "clap")
			addSoftwareType(seen, &ordered, "lv2")
		case "yoshimi":
			addSoftwareType(seen, &ordered, "lv2")
		}

		if len(ordered) == 0 && strings.TrimSpace(entry["formats"]) == "" {
			addSoftwareType(seen, &ordered, "standalone")
		}

		preferredOrder := []string{"standalone", "clap", "vst", "lv2"}
		reordered := make([]string, 0, len(ordered))
		for _, kind := range preferredOrder {
			if _, ok := seen[kind]; ok {
				reordered = append(reordered, kind)
			}
		}
		return reordered
	}
	genericArchivePackage := func(id string, vendor string, summary string) *Package {
		entry := mustEntry(id)
		name := entry["name"]
		installedMarkers := archiveInstalledMarkers(id)
		if !downloadWithinApp(id) {
			return &Package{
				ID:          id,
				Name:        name,
				Vendor:      vendor,
				Summary:     summary,
				Description: "Opens the upstream project page so you can download the current Linux build directly from the developer.",
				Notes: []string{
					"Download is handled on the developer website rather than inside the installer.",
				},
				Links:             linkForID(id),
				ExternalActionURL: externalActionURLForID(id),
				AvailabilityNote:  "Use Get From Site to open the upstream download page in your browser.",
				InstalledMarkers:  installedMarkers,
				UninstallActions: []Action{
					{Title: fmt.Sprintf("Uninstall %s", name), Exec: archiveUninstall(id)},
				},
			}
		}
		return &Package{
			ID:          id,
			Name:        name,
			Vendor:      vendor,
			Summary:     summary,
			Description: fmt.Sprintf("Downloads the upstream Linux payload and installs the contained %s into the current user's plugin directories.", archiveTargets(id)),
			Notes: []string{
				"Does not require sudo.",
				"Installed as a user-local plugin so it works cleanly on immutable systems.",
			},
			Links:            linkForID(id),
			InstalledMarkers: installedMarkers,
			InstallActions: []Action{
				{Title: fmt.Sprintf("Install %s", name), Exec: archiveInstall(id)},
			},
			UninstallActions: []Action{
				{Title: fmt.Sprintf("Uninstall %s", name), Exec: archiveUninstall(id)},
			},
		}
	}
	alienDebPackage := func(id string, vendor string, summary string) *Package {
		entry := mustEntry(id)
		name := entry["name"]
		return &Package{
			ID:          id,
			Name:        name,
			Vendor:      vendor,
			Summary:     summary,
			Description: fmt.Sprintf("Downloads the upstream Debian package, extracts it, and installs the contained %s into the current user's plugin directories.", archiveTargets(id)),
			Notes: []string{
				"Does not require sudo.",
				"Installed as a user-local plugin so it works cleanly on immutable systems.",
			},
			Links:            linkForID(id),
			InstalledMarkers: archiveInstalledMarkers(id),
			InstallActions: []Action{
				{Title: fmt.Sprintf("Install %s", name), Exec: archiveInstall(id)},
			},
			UninstallActions: []Action{
				{Title: fmt.Sprintf("Uninstall %s", name), Exec: archiveUninstall(id)},
			},
		}
	}
	appImagePackage := func(id string, vendor string, summary string, searchToken string) *Package {
		entry := mustEntry(id)
		name := entry["name"]
		return &Package{
			ID:          id,
			Name:        name,
			Vendor:      vendor,
			Summary:     summary,
			Description: "Downloads the upstream AppImage, then uses AppImageLauncher's ail-cli to integrate it into the desktop session without GUI interaction. If AppImageLauncher is unavailable or fails, installs the AppImage manually with a user desktop entry.",
			Notes: []string{
				"Does not require sudo.",
				"Uses ail-cli for desktop integration when available.",
				"Falls back to a manual install in ~/Applications.",
			},
			Links: linkForID(id),
			InstalledMarkers: []string{
				"Applications/" + id + ".appimage",
				"Applications/*" + searchToken + "*.appimage",
				"AppImages/" + id + ".appimage",
				"AppImages/*" + searchToken + "*.appimage",
				".local/share/applications/*" + searchToken + "*.desktop",
			},
			InstallActions: []Action{
				{Title: fmt.Sprintf("Install %s", name), Exec: script("install-appimage-with-ail-cli.sh", id, name, entry["url"])},
			},
			UninstallActions: []Action{
				{Title: fmt.Sprintf("Uninstall %s", name), Exec: script("uninstall-appimage-with-ail-cli.sh", id, searchToken)},
			},
		}
	}
	portableZipInstall := func(id string, executableName string, wrapperName string, desktopID string, comment string) []string {
		entry := mustEntry(id)
		return script("install-portable-zip-app.sh", id, entry["name"], entry["version"], entry["url"], executableName, wrapperName, desktopID, comment)
	}
	portableZipUninstall := func(id string, wrapperName string, desktopID string) []string {
		return script("uninstall-portable-zip-app.sh", id, wrapperName, desktopID)
	}
	categories := []*Category{
		{
			ID:          "daws",
			Name:        "DAWs",
			Description: "Workstation and sequencer installs that complement the default Caracal toolset.",
			Accent:      "#7dd3fc",
			Subcategories: []*Subcategory{
				{
					ID:          "commercial-daws",
					Name:        "Commercial DAWs",
					Description: "Optional commercial workstation installs that currently live in Caracal's post-install flow.",
					Packages: []*Package{
						{
							ID:          "mixbus",
							Name:        "Mixbus",
							Vendor:      "Solid State Logic",
							Summary:     "Full-featured DAW with analog-style mixing workflow.",
							Description: "Downloads the upstream Mixbus tarball, extracts the bundled .run installer, and executes it as a system installer.",
							Notes: []string{
								"Requires sudo because the upstream installer writes system paths.",
								"The upstream .run installer may still present its own prompts.",
							},
							Links: linkForID("mixbus"),
							InstalledMarkers: []string{
								"/opt/Mixbus*",
								"/usr/local/share/applications/*mixbus*.desktop",
								"/usr/share/applications/*mixbus*.desktop",
							},
							InstallActions: []Action{
								{Title: "Install Mixbus", Exec: sudoScript("install-mixbus.sh")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Mixbus", Exec: sudoScript("uninstall-mixbus.sh")},
							},
						},
						{
							ID:          "reaper",
							Name:        "REAPER",
							Vendor:      "Cockos",
							Summary:     "Fast commercial DAW with unrestricted evaluation.",
							Description: "Installs REAPER into /opt/REAPER and publishes a system desktop entry and icon in /usr/local. The installer also seeds plugin search paths in the target user's REAPER config when sudo is used.",
							Notes: []string{
								"Requires sudo because it writes to /opt and /usr/local.",
								"Preserves compatibility with Caracal's existing REAPER install approach.",
							},
							Links: linkForID("reaper"),
							InstalledMarkers: []string{
								"/opt/REAPER/reaper",
								"/usr/local/share/applications/cockos-reaper.desktop",
							},
							InstallActions: []Action{
								{Title: "Install REAPER", Exec: sudoScript("install-reaper.sh")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall REAPER", Exec: sudoScript("uninstall-reaper.sh")},
							},
						},
						{
							ID:          "renoise",
							Name:        "Renoise",
							Vendor:      "Renoise",
							Summary:     "Tracker-style DAW with demo-mode installer.",
							Description: "Installs the current Renoise demo into /opt/renoise, adds a wrapper command, desktop integration, MIME metadata, and icons in /usr/local.",
							Notes: []string{
								"Requires sudo because it writes to /opt and /usr/local.",
								"The shipped installer targets the demo build until license activation happens inside Renoise.",
							},
							Links: linkForID("renoise"),
							InstalledMarkers: []string{
								"/opt/renoise/renoise",
								"/usr/local/share/applications/renoise.desktop",
							},
							InstallActions: []Action{
								{Title: "Install Renoise", Exec: sudoScript("install-renoise.sh")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Renoise", Exec: sudoScript("uninstall-renoise.sh")},
							},
						},
						{
							ID:          "bitwig-studio",
							Name:        "Bitwig Studio",
							Vendor:      "Bitwig",
							Summary:     "Commercial DAW with native Linux support.",
							Description: "Downloads the official Bitwig .deb, extracts it into /opt/bitwig-studio, and publishes desktop integration through /usr/local so it survives immutable image updates.",
							Notes: []string{
								"Requires sudo because it writes to /opt and /usr/local.",
								"Bitwig itself still requires a valid upstream license.",
							},
							Links: linkForID("bitwig-studio"),
							InstalledMarkers: []string{
								"/opt/bitwig-studio/bitwig-studio",
								"/usr/local/share/applications/bitwig-studio.desktop",
							},
							InstallActions: []Action{
								{Title: "Install Bitwig Studio", Exec: sudoScript("install-bitwig.sh")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Bitwig Studio", Exec: sudoScript("uninstall-bitwig.sh")},
							},
						},
					},
				},
				{
					ID:          "open-and-free-daws",
					Name:        "Open & Free DAWs",
					Description: "Sequencers and workstation-style tools available as free or open-source Linux builds.",
					Packages: []*Package{
						appImagePackage("helio", "Helio", "Open-source music sequencer for desktop and mobile platforms.", "helio"),
						appImagePackage("shoopdaloop", "Sander Vocke", "Live looping application with DAW elements distributed as an AppImage.", "shoopdaloop"),
						appImagePackage("stargate", "Stargate", "Cross-platform all-in-one DAW and plugin suite distributed as an AppImage.", "stargate"),
						{
							ID:          "zrythm",
							Name:        "Zrythm",
							Vendor:      "Zrythm",
							Summary:     "Highly automated and intuitive digital audio workstation.",
							Description: "Downloads the upstream Zrythm trial installer ZIP and installs its payload into /opt with desktop integration in /usr/local.",
							Notes: []string{
								"Requires sudo because it writes to /opt and /usr/local.",
								"Runs a non-interactive Caracal wrapper instead of the upstream install.sh prompt flow.",
							},
							Links: linkForID("zrythm"),
							InstalledMarkers: []string{
								"/opt/zrythm",
								"/opt/zrythm-trial-1.0.0/bin/zrythm_launch",
								"/usr/local/share/applications/org.zrythm.Zrythm-installer.desktop",
							},
							InstallActions: []Action{
								{Title: "Install Zrythm", Exec: sudoScript("install-zrythm.sh")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Zrythm", Exec: sudoScript("uninstall-zrythm.sh")},
							},
						},
					},
				},
			},
		},
		{
			ID:          "virtual-instruments",
			Name:        "Virtual Instruments",
			Description: "Synths, modular environments, drums, and sample players available as optional installs.",
			Accent:      "#f59e0b",
			Subcategories: []*Subcategory{
				{
					ID:          "open-synths",
					Name:        "Open Synths",
					Description: "Native instruments that fit well into the Caracal plugin path layout.",
					Packages: []*Package{
						{
							ID:          "sunvox",
							Name:        "SunVox",
							Vendor:      "Warmplace",
							Summary:     "Modular tracker and synth studio distributed as a portable ZIP archive.",
							Description: "Downloads the official SunVox Linux ZIP, installs it under /opt/caracal/warmplace/sunvox, and creates a wrapper plus desktop entry in /usr/local.",
							Notes: []string{
								"Requires sudo because it writes to /opt and /usr/local.",
								"SunVox uses a portable archive layout rather than a distro-native package.",
							},
							Links: linkForID("sunvox"),
							InstalledMarkers: []string{
								"/usr/local/bin/sunvox",
								"/usr/local/share/applications/sunvox.desktop",
							},
							InstallActions: []Action{
								{Title: "Install SunVox", Exec: sudoScript("install-sunvox.sh")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall SunVox", Exec: sudoScript("uninstall-sunvox.sh")},
							},
						},
						{
							ID:          "virtual-ans",
							Name:        "Virtual ANS",
							Vendor:      "Warmplace",
							Summary:     "Spectral drawing synthesizer distributed as a portable ZIP archive.",
							Description: "Downloads the official Virtual ANS Linux ZIP, installs it under /opt/caracal/warmplace/virtual-ans, and creates a wrapper plus desktop entry in /usr/local.",
							Notes: []string{
								"Requires sudo because it writes to /opt and /usr/local.",
								"Distributed upstream as a portable archive with the Linux launcher inside the extracted folder.",
							},
							Links: linkForID("virtual-ans"),
							InstalledMarkers: []string{
								"/usr/local/bin/virtual-ans",
								"/usr/local/share/applications/virtual-ans.desktop",
							},
							InstallActions: []Action{
								{Title: "Install Virtual ANS", Exec: sudoScript("install-virtual-ans.sh")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Virtual ANS", Exec: sudoScript("uninstall-virtual-ans.sh")},
							},
						},
						{
							ID:          "cardinal",
							Name:        "Cardinal",
							Vendor:      "DISTRHO",
							Summary:     "VCV Rack-derived modular environment with standalone and plugin targets.",
							Description: "Downloads the official Cardinal Linux bundle and installs the standalone binaries plus VST, VST3, LV2, and CLAP targets into /usr/local for immutable-system compatibility.",
							Notes: []string{
								"Requires sudo because it writes to /usr/local/bin and /usr/local/lib64.",
								"This replaces the previous image-baked Cardinal install path.",
							},
							Links: linkForID("cardinal"),
							InstalledMarkers: []string{
								"/usr/local/bin/Cardinal",
								"/usr/local/lib64/vst3/Cardinal.vst3",
							},
							InstallActions: []Action{
								{Title: "Install Cardinal", Exec: sudoScript("install-cardinal.sh")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Cardinal", Exec: sudoScript("uninstall-cardinal.sh")},
							},
						},
						{
							ID:          "vcv-rack-2",
							Name:        "VCV Rack 2",
							Vendor:      "VCV",
							Summary:     "Open-source virtual Eurorack environment distributed as a portable ZIP archive.",
							Description: "Downloads the upstream VCV Rack Free ZIP into the current user's local Caracal app directory, then creates a user-local wrapper and desktop entry.",
							Notes: []string{
								"Does not require sudo.",
								"Runs from a user-local portable app directory so it works cleanly on immutable systems.",
							},
							Links: linkForID("vcv-rack-2"),
							InstalledMarkers: []string{
								".local/share/caracal-software-installer/apps/vcv-rack-2/current/Rack",
								".local/bin/vcv-rack-2",
								".local/share/applications/vcv-rack-2.desktop",
							},
							InstallActions: []Action{
								{Title: "Install VCV Rack 2", Exec: portableZipInstall("vcv-rack-2", "Rack", "vcv-rack-2", "vcv-rack-2", "Open-source virtual Eurorack DAW")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall VCV Rack 2", Exec: portableZipUninstall("vcv-rack-2", "vcv-rack-2", "vcv-rack-2")},
							},
						},
						{
							ID:          "mod-desktop",
							Name:        "MOD Desktop",
							Vendor:      "MOD Audio",
							Summary:     "MOD's modular pedalboard environment reimagined as a standalone desktop application.",
							Description: "Downloads the official MOD Desktop tarball, extracts the bundled app folder into /opt/mod-desktop (writable on atomic Fedora via /var/opt), and publishes a system wrapper plus desktop entry in /usr/local.",
							Notes: []string{
								"Requires sudo because it writes to /opt and /usr/local.",
								"Ships its own Python runtime, jackd, and libjack, so the install does not pull in extra OS packages on the immutable image.",
								"Builds custom pedalboards, chains plugins, and sculpts tones without needing a MOD hardware device.",
							},
							Links: linkForID("mod-desktop"),
							InstalledMarkers: []string{
								"/opt/mod-desktop/mod-desktop/mod-desktop",
								"/usr/local/bin/mod-desktop",
								"/usr/local/share/applications/mod-desktop.desktop",
							},
							InstallActions: []Action{
								{Title: "Install MOD Desktop", Exec: sudoScript("install-mod-desktop.sh")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall MOD Desktop", Exec: sudoScript("uninstall-mod-desktop.sh")},
							},
						},
						{
							ID:          "surge-xt",
							Name:        "Surge XT",
							Vendor:      "Surge Synth Team",
							Summary:     "Open-source hybrid synthesizer installed from the upstream RPM payload.",
							Description: "Downloads the upstream Surge XT RPM, extracts its payload without layering the OS image, and mirrors the relevant binaries, plugins, and desktop files into /usr/local.",
							Notes: []string{
								"Requires sudo because it writes into /usr/local.",
								"Uses archive extraction rather than dnf layering so it works as a post-install action on Caracal.",
							},
							Links: linkForID("surge-xt"),
							InstalledMarkers: []string{
								"/usr/local/bin/*surge*",
								"/usr/local/lib64/vst3/*Surge*",
							},
							InstallActions: []Action{
								{Title: "Install Surge XT", Exec: sudoScript("install-surge-xt.sh")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Surge XT", Exec: sudoScript("uninstall-surge-xt.sh")},
							},
						},
						genericArchivePackage("wavetable", "FigBug", "Two-oscillator wavetable synth with VST, VST3, and LV2 targets."),
						genericArchivePackage("ob-xf", "Surge Synth Team", "Open-source OB-style synth distributed as Linux plugin bundles."),
						genericArchivePackage("odin2", "TheWaveWarden", "Hybrid synth distributed as a Linux archive with CLAP and VST3 targets."),
						{
							ID:          "tal-noisemaker",
							Name:        "TAL-Noisemaker",
							Vendor:      "TAL Software",
							Summary:     "Free virtual analog synth installed from TAL's Linux archive.",
							Description: "Downloads TAL-Noisemaker and installs the contained CLAP, VST3, VST2, and LV2 payloads into the current user's plugin directories.",
							Notes: []string{
								"Does not require sudo.",
								"Installed as a user-local plugin set so it works cleanly on immutable systems.",
							},
							Links: linkForID("tal-noisemaker"),
							InstalledMarkers: []string{
								".clap/TAL-NoiseMaker.clap",
								".vst3/TAL-NoiseMaker.vst3",
								".vst/libTAL-NoiseMaker.so",
								".lv2/TAL-NoiseMaker.lv2",
							},
							InstallActions: []Action{
								{Title: "Install TAL-Noisemaker", Exec: archiveInstall("tal-noisemaker")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall TAL-Noisemaker", Exec: archiveUninstall("tal-noisemaker")},
							},
						},
						genericArchivePackage("dexed", "DISTRHO Ports", "DX7-inspired synth distributed as a Linux LV2 bundle."),
						genericArchivePackage("juce-opl", "DISTRHO Ports", "FM synth inspired by classic sound cards and packaged as a Linux LV2 bundle."),
						genericArchivePackage("obxd", "DISTRHO Ports", "OB-inspired synth distributed as a Linux LV2 bundle."),
						genericArchivePackage("vex", "DISTRHO Ports", "Three-oscillator subtractive synth distributed as a Linux LV2 bundle."),
						genericArchivePackage("wolpertinger", "DISTRHO Ports", "Polyphonic subtractive synth distributed as a Linux LV2 bundle."),
						genericArchivePackage("ripplerx", "tiagolr", "Physical modeling synth distributed as Linux VST3 and LV2 bundles."),
						{
							ID:          "yoshimi",
							Name:        "Yoshimi",
							Vendor:      "Yoshimi",
							Summary:     "Open-source synth built from source into ~/.local.",
							Description: "Downloads the current Yoshimi source archive, builds it locally, and installs its standalone and plugin payloads into the current user's ~/.local tree.",
							Notes: []string{
								"Does not require sudo.",
								"Builds from source, so make and the required development libraries must already be available.",
							},
							Links: linkForID("yoshimi"),
							InstalledMarkers: []string{
								".local/bin/yoshimi",
							},
							InstallActions: []Action{
								{Title: "Install Yoshimi", Exec: sourceInstall("yoshimi")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Yoshimi", Exec: sourceUninstall("yoshimi", "Yoshimi")},
							},
						},
						{
							ID:          "synthv1",
							Name:        "Synthv1",
							Vendor:      "rncbc",
							Summary:     "Subtractive synth built from source into ~/.local.",
							Description: "Downloads the current Synthv1 source archive, builds it locally, and installs its binary and plugin bundles into the current user's ~/.local tree.",
							Notes: []string{
								"Does not require sudo.",
								"Builds from source, so make and the required development libraries must already be available.",
							},
							Links: linkForID("synthv1"),
							InstalledMarkers: []string{
								".local/lib/lv2/synthv1.lv2",
								".local/bin/synthv1",
							},
							InstallActions: []Action{
								{Title: "Install Synthv1", Exec: sourceInstall("synthv1")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Synthv1", Exec: sourceUninstall("synthv1", "Synthv1")},
							},
						},
						{
							ID:          "padhv1",
							Name:        "Padhv1",
							Vendor:      "rncbc",
							Summary:     "Pad-oriented synth built from source into ~/.local.",
							Description: "Downloads the current Padhv1 source archive, builds it locally, and installs its binary and plugin bundles into the current user's ~/.local tree.",
							Notes: []string{
								"Does not require sudo.",
								"Builds from source, so make and the required development libraries must already be available.",
							},
							Links: linkForID("padhv1"),
							InstalledMarkers: []string{
								".local/lib/lv2/padhv1.lv2",
								".local/bin/padhv1",
							},
							InstallActions: []Action{
								{Title: "Install Padhv1", Exec: sourceInstall("padhv1")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Padhv1", Exec: sourceUninstall("padhv1", "Padhv1")},
							},
						},
						genericArchivePackage("ensoniq", "sojusrecords", "Ensoniq SD-1 inspired synth packaged as a Linux VST3 archive."),
						genericArchivePackage("kr106", "kayrockscreenprinting", "Vintage-inspired synth distributed as Linux VST3 and LV2 bundles."),
						genericArchivePackage("tb4006", "Robot Planet", "Bassline synth distributed as a Linux VST3 archive."),
						genericArchivePackage("suboctb", "yimrakhee", "Sub-octave focused synth packaged with CLAP and VST3 targets."),
						genericArchivePackage("floe-vst", "floe audio", "Synth voice distributed as a Linux VST3 archive."),
						genericArchivePackage("floe-clap", "floe audio", "Synth voice distributed as a Linux CLAP archive."),
						genericArchivePackage("termenflux", "Hergezod", "Open-source Theremin-style instrument distributed as a Linux LV2 archive."),
						genericArchivePackage("commodore-64-sid", "Socalabs", "Commodore 64 SID chip emulation distributed as a Linux Debian package."),
						genericArchivePackage("nes-rp2a03", "Socalabs", "Nintendo Entertainment System RP2A03 chip emulation distributed as a Linux Debian package."),
						genericArchivePackage("nintendo-gameboy-papu", "Socalabs", "Nintendo Gameboy PAPU chip emulation distributed as a Linux Debian package."),
						genericArchivePackage("socalabs-organ", "Socalabs", "Classic tonewheel organ emulation distributed as a Linux Debian package."),
					},
				},
				{
					ID:          "commercial-instruments",
					Name:        "Commercial Instruments",
					Description: "Commercial Linux instrument plugins installed into user-local plugin paths.",
					Packages: []*Package{
						genericArchivePackage("tal-j8x", "TAL Software", "Jupiter-8-inspired synth plugin distributed as Linux CLAP, VST3, and VST2 targets."),
						genericArchivePackage("tal-pha", "TAL Software", "Alpha Juno-inspired synth plugin distributed as Linux CLAP, VST3, and VST2 targets."),
						genericArchivePackage("tal-j-8", "TAL Software", "Jupiter-8 emulation distributed as Linux CLAP, VST3, and VST2 targets."),
						genericArchivePackage("tal-u-no-lx-v2", "TAL Software", "Juno-60-inspired synth plugin distributed as Linux CLAP, VST3, and VST2 targets."),
						genericArchivePackage("tal-bassline-101", "TAL Software", "SH-101-inspired bass synth distributed as Linux CLAP, VST3, and VST2 targets."),
						genericArchivePackage("tal-mod", "TAL Software", "Virtual analog synth distributed as Linux CLAP, VST3, and VST2 targets."),
					},
				},
				{
					ID:          "samplers-and-players",
					Name:        "Samplers & Players",
					Description: "Sample playback tools that round out the base system.",
					Packages: []*Package{
						{
							ID:          "loopino",
							Name:        "Loopino",
							Vendor:      "brummer10",
							Summary:     "Live looper instrument built from source as user-local CLAP and VST2 plugins.",
							Description: "Clones the Loopino repository, initializes submodules, builds the CLAP and VST2 plugin targets from source, and installs them into the current user's plugin directories. The standalone target is intentionally skipped.",
							Notes: []string{
								"Does not require sudo.",
								"Builds from source, so git, make, and a working native build toolchain are required on the target system.",
								"Installs only CLAP and VST2, matching the current Caracal post-install preference for Loopino.",
							},
							Links: linkForID("loopino"),
							InstalledMarkers: []string{
								".clap/*Loopino*",
								".vst/*Loopino*",
							},
							InstallActions: []Action{
								{Title: "Install Loopino", Exec: script("install-loopino.sh")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Loopino", Exec: script("uninstall-loopino.sh")},
							},
						},
						{
							ID:          "decent-sampler",
							Name:        "Decent Sampler",
							Vendor:      "Decent Samples",
							Summary:     "Lightweight standalone and plugin sample player.",
							Description: "Downloads the static Decent Sampler bundle and installs the standalone binary plus VST and VST3 targets into /usr/local.",
							Notes: []string{
								"Requires sudo because it writes into /usr/local.",
								"This replaces the previous image-baked Decent Sampler install path.",
							},
							Links: linkForID("decent-sampler"),
							InstalledMarkers: []string{
								"/usr/local/bin/DecentSampler",
								"/usr/local/lib64/vst3/DecentSampler.vst3",
							},
							InstallActions: []Action{
								{Title: "Install Decent Sampler", Exec: sudoScript("install-decent-sampler.sh")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Decent Sampler", Exec: sudoScript("uninstall-decent-sampler.sh")},
							},
						},
						{
							ID:          "samplv1",
							Name:        "Samplv1",
							Vendor:      "rncbc",
							Summary:     "Sample-based instrument built from source into ~/.local.",
							Description: "Downloads the current Samplv1 source archive, builds it locally, and installs its binary and plugin bundles into the current user's ~/.local tree.",
							Notes: []string{
								"Does not require sudo.",
								"Builds from source, so make and the required development libraries must already be available.",
							},
							Links: linkForID("samplv1"),
							InstalledMarkers: []string{
								".local/lib/lv2/samplv1.lv2",
								".local/bin/samplv1",
							},
							InstallActions: []Action{
								{Title: "Install Samplv1", Exec: sourceInstall("samplv1")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Samplv1", Exec: sourceUninstall("samplv1", "Samplv1")},
							},
						},
						genericArchivePackage("looper-pedal", "rbmannchued", "Looper plugin distributed as a Linux LV2 bundle."),
						genericArchivePackage("tal-sampler", "TAL Software", "Analog-modeled sampler instrument distributed as Linux CLAP, VST3, and VST2 targets."),
					},
				},
				{
					ID:          "drums-and-percussion",
					Name:        "Drums & Percussion",
					Description: "Drum machines, drum instruments, and groove-oriented tools.",
					Packages: []*Package{
						genericArchivePackage("jdrummer", "jmantra", "Drum instrument distributed as a Linux VST3 archive."),
						{
							ID:          "drumkv1",
							Name:        "Drumkv1",
							Vendor:      "rncbc",
							Summary:     "Drum sampler instrument built from source into ~/.local.",
							Description: "Downloads the current Drumkv1 source archive, builds it locally, and installs its binary and plugin bundles into the current user's ~/.local tree.",
							Notes: []string{
								"Does not require sudo.",
								"Builds from source, so make and the required development libraries must already be available.",
							},
							Links: linkForID("drumkv1"),
							InstalledMarkers: []string{
								".local/lib/lv2/drumkv1.lv2",
								".local/bin/drumkv1",
							},
							InstallActions: []Action{
								{Title: "Install Drumkv1", Exec: sourceInstall("drumkv1")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Drumkv1", Exec: sourceUninstall("drumkv1", "Drumkv1")},
							},
						},
						genericArchivePackage("drum-locker", "Audio Assault", "Drum and groove production plugin installed from the official Linux archive."),
						genericArchivePackage("avl-drumkits", "x42", "AVL Drumkits sample-player plugin distributed as a Linux LV2 bundle."),
						genericArchivePackage("tal-drum", "TAL Software", "Drum instrument distributed as Linux CLAP, VST3, and VST2 targets."),
						{
							ID:          "drumlabooh",
							Name:        "Drumlabooh",
							Vendor:      "Petr Semiletov",
							Summary:     "Drum instrument with Hydrogen, Drumlabooh/Drumrox, and SFZ kit support.",
							Description: "Downloads the pinned Drumlabooh LV2 and VST3 plugin ZIPs plus drum_sklad kits, then installs them into the current user's plugin and kit directories.",
							Notes: []string{
								"Does not require sudo.",
								"Uses a Caracal wrapper around the upstream net-installer flow so it avoids root and records uninstall paths.",
							},
							Links: linkForID("drumlabooh"),
							InstalledMarkers: []string{
								".lv2/drumlabooh.lv2",
								".lv2/drumlabooh-multi.lv2",
								".vst3/drumlabooh.vst3",
								".vst3/drumlabooh-multi.vst3",
								"drum_sklad",
							},
							InstallActions: []Action{
								{Title: "Install Drumlabooh", Exec: script("install-drumlabooh.sh")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Drumlabooh", Exec: script("uninstall-drumlabooh.sh")},
							},
						},
						genericArchivePackage("drum-groove-pro", "InToEtherion", "Drum performance plugin distributed as a Linux VST3 archive."),
						genericArchivePackage("black-widow-drums", "odoare", "Drum instrument packaged as a Linux VST3 bundle."),
					},
				},
			},
		},
		{
			ID:          "effects",
			Name:        "Effects",
			Description: "Optional processor installs grouped by what they do instead of by brand.",
			Accent:      "#34d399",
			Subcategories: []*Subcategory{
				{
					ID:          "amp-and-guitar",
					Name:        "Amp & Guitar",
					Description: "Amp sims, pedalboards, and guitar-focused processors.",
					Packages: []*Package{
						genericArchivePackage("amp-locker", "Audio Assault", "Amp sim platform installed from the official Linux archive."),
						alienDebPackage("byod", "Chowdhury DSP", "Modular pedalboard and amp chain plugin distributed as a Linux Debian package."),
						{
							ID:          "neural-amp-model",
							Name:        "Neural Amp Modeler",
							Vendor:      "Mike Oliphant",
							Summary:     "Neural-amp-model LV2 build distributed as a Linux archive.",
							Description: "Downloads the upstream Linux archive and installs the contained LV2 bundle into the current user's plugin directories.",
							Notes: []string{
								"Does not require sudo.",
								"The archive bundle naming is inconsistent upstream, so uninstall uses a safe wildcard cleanup path.",
							},
							Links: linkForID("neural-amp-model"),
							InstalledMarkers: []string{
								".lv2/neural_amp_modeler.lv2",
							},
							InstallActions: []Action{
								{Title: "Install Neural Amp Modeler", Exec: archiveInstall("neural-amp-model")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Neural Amp Modeler", Exec: script("uninstall-neural-amp-model.sh")},
							},
						},
						{
							ID:          "aida-x",
							Name:        "AIDA-X",
							Vendor:      "AidaDSP",
							Summary:     "Amp capture and guitar processing plugin distributed as a Linux archive.",
							Description: "Downloads the upstream Linux archive and installs the contained CLAP, VST3, and LV2 bundles into the current user's plugin directories.",
							Notes: []string{
								"Does not require sudo.",
								"Installed as a user-local plugin so it works cleanly on immutable systems.",
							},
							Links: linkForID("aida-x"),
							InstalledMarkers: []string{
								".clap/AIDA-X.clap",
								".vst3/AIDA-X.vst3",
								".lv2/AIDA-X.lv2",
							},
							InstallActions: []Action{
								{Title: "Install AIDA-X", Exec: archiveInstall("aida-x")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall AIDA-X", Exec: archiveUninstall("aida-x")},
							},
						},
						genericArchivePackage("neampmod-the-tweed-vst3", "danielwray", "Tweed-style amp sim distributed as a direct Linux VST3 download."),
						genericArchivePackage("neampmod-the-tweed-clap", "danielwray", "Tweed-style amp sim distributed as a direct Linux CLAP download."),
						genericArchivePackage("chow-centaur", "Chowdhury DSP", "Klon-style overdrive pedal distributed as Linux VST3 and LV2 bundles."),
						genericArchivePackage("ratatouille", "brummer10", "Neural model and impulse response mixer distributed as a Linux LV2 bundle."),
						alienDebPackage("ts-m1n3", "GuitarML", "TS-9 Tubescreamer-style overdrive plugin distributed as a Linux Debian package."),
						alienDebPackage("chameleon", "GuitarML", "Neural vintage amp-head plugin with three distinct sounds."),
						alienDebPackage("smartamp", "GuitarML", "Machine-learning guitar amp plugin for tube amplifier tones."),
						alienDebPackage("smartpedal", "GuitarML", "Neural guitar pedal plugin compatible with PedalNetRT models."),
						alienDebPackage("proteus", "GuitarML", "GuitarML capture plugin focused on lower CPU use while preserving capture quality."),
						alienDebPackage("epochamp", "GuitarML", "Amp-modeling plugin for realistic guitar tones."),
						alienDebPackage("neuralpi", "GuitarML", "Neural amp and pedal emulation plugin based on the NeuralPi project."),
						alienDebPackage("the-prince", "GuitarML", "Transparent-overdrive-style pedal plugin cloned with neural networks."),
					},
				},
				{
					ID:          "mixing-and-channel-strip",
					Name:        "Mixing & Channel Strip",
					Description: "Mix-focused processors and channel-strip style tools.",
					Packages: []*Package{
						genericArchivePackage("mix-locker", "Audio Assault", "Channel-strip and mix processing platform installed from the official Linux archive."),
						genericArchivePackage("acmt-plugin-suite", "ACMT", "Commercial analogue-modeled plugin suite available from the upstream Linux download page."),
						genericArchivePackage("the-trick", "Mouse Plugins", "Focused EQ processor distributed as a Linux VST3 archive."),
						genericArchivePackage("polarity", "Polarity", "Spectral compressor plugin packaged with CLAP and VST3 targets."),
						genericArchivePackage("nine-strip", "blablack", "Channel-strip processor distributed as Linux VST3 and LV2 bundles."),
						genericArchivePackage("lufs-meter", "DISTRHO Ports", "Loudness metering plugin distributed as a Linux LV2 bundle."),
						genericArchivePackage("luftikus", "DISTRHO Ports", "Analog-inspired EQ distributed as a Linux LV2 bundle."),
						genericArchivePackage("zl-compressor", "ZL Audio", "Open-source compressor plugin distributed as Linux VST3 and LV2 bundles."),
						genericArchivePackage("zl-equalizer", "ZL Audio", "Open-source equalizer plugin distributed as Linux VST3 and LV2 bundles."),
						genericArchivePackage("4k-eq", "dusk audio", "EQ processor distributed as Linux LV2 and VST3 bundles."),
						genericArchivePackage("multi-comp", "dusk audio", "Compressor distributed as Linux LV2 and VST3 bundles."),
						genericArchivePackage("multi-q", "dusk audio", "Surgical EQ distributed as Linux LV2 and VST3 bundles."),
						genericArchivePackage("tal-eq", "TAL Software", "Commercial equalizer plugin distributed as Linux CLAP, VST3, and VST2 targets."),
						genericArchivePackage("tdr-infrasonic", "Tokyo Dawn Labs", "Specialized filter with minimum and mixed phase modes, variable slope control, dry mix, and filtering loss compensation."),
						genericArchivePackage("tdr-elliptical", "Tokyo Dawn Labs", "Low-frequency stereo-width control using elliptical filtering on the stereo difference channel."),
						genericArchivePackage("tdr-ultrasonic", "Tokyo Dawn Labs", "Filter designed to control ultrasonic content in oversampled recording, processing, and playback chains."),
						genericArchivePackage("tdr-arbiter", "Tokyo Dawn Labs", "Frequency-specific spectral balancer for mixing, restoration, and mastering workflows."),
						genericArchivePackage("tdr-limiter6-ge", "Tokyo Dawn Labs", "Modern dynamics compression and limiting toolkit with six specialized reorderable modules."),
						genericArchivePackage("tdr-prism", "Tokyo Dawn Labs", "Frequency analyzer focused on human audio perception with straightforward configuration."),
						genericArchivePackage("wstd-mseq", "Wasted Audio", "Pay-what-you-want 3-band mid/side EQ processor available from the upstream site."),
					},
				},
				{
					ID:          "reverb-and-spatial",
					Name:        "Time & Spatial",
					Description: "Reverbs, delays, modulation, and spatial processors.",
					Packages: []*Package{
						{
							ID:          "dragonfly",
							Name:        "Dragonfly Reverb",
							Vendor:      "Michael Willis",
							Summary:     "Open-source reverb suite distributed as Linux plugin bundles.",
							Description: "Downloads the upstream Linux archive and installs the contained CLAP, VST3, and LV2 bundles into the current user's plugin directories.",
							Notes: []string{
								"Does not require sudo.",
								"The suite ships multiple Dragonfly bundles, so uninstall uses wildcard cleanup across the supported plugin directories.",
							},
							Links: linkForID("dragonfly"),
							InstalledMarkers: []string{
								".clap/Dragonfly*.clap",
								".vst3/Dragonfly*.vst3",
								".lv2/Dragonfly*.lv2",
							},
							InstallActions: []Action{
								{Title: "Install Dragonfly Reverb", Exec: archiveInstall("dragonfly")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Dragonfly Reverb", Exec: script("uninstall-dragonfly.sh")},
							},
						},
						genericArchivePackage("klangfalter", "DISTRHO Ports", "Convolution processor distributed as a Linux LV2 bundle."),
						genericArchivePackage("mverb", "DISTRHO", "Studio reverb distributed as a Linux LV2 bundle."),
						genericArchivePackage("pitched-delay", "DISTRHO Ports", "Pitch-shifting delay processor distributed as a Linux LV2 bundle."),
						alienDebPackage("chow-phaser", "Chowdhury DSP", "Phaser effect distributed as a Linux Debian package."),
						genericArchivePackage("room-reverb", "ElephantDSP", "Room reverb plugin distributed as Linux CLAP, LV2, and VST3 bundles."),
						genericArchivePackage("del2", "magnetophon", "Delay processor distributed as Linux VST3 and CLAP bundles."),
						genericArchivePackage("panoramatone", "PilCAki", "Vibrato processor distributed as a Linux VST3 bundle."),
						genericArchivePackage("tentacles", "PilCAki", "Tentacle-inspired vibrato processor available from the project site."),
						genericArchivePackage("aelapse", "smiarx", "Delay and reverb processor distributed as Linux VST3 and LV2 bundles."),
						genericArchivePackage("tapemachine", "dusk audio", "Tape delay processor distributed as Linux LV2 and VST3 bundles."),
						genericArchivePackage("duskverb", "dusk audio", "Reverb processor distributed as Linux LV2 and VST3 bundles."),
						genericArchivePackage("wet-delay", "yonie", "Delay plugin distributed as a Linux VST3 archive."),
						genericArchivePackage("wet-reverb", "yonie", "Reverb plugin distributed as a Linux VST3 archive."),
						genericArchivePackage("tal-g-verb", "TAL Software", "Commercial reverb plugin distributed as Linux CLAP, VST3, and VST2 targets."),
						genericArchivePackage("tal-dub-x", "TAL Software", "Commercial delay plugin distributed as Linux CLAP, VST3, and VST2 targets."),
					},
				},
				{
					ID:          "creative-and-utility",
					Name:        "Creative & Utility",
					Description: "Sound-design tools, utilities, and unusual processors.",
					Packages: []*Package{
						genericArchivePackage("noise-repellent", "lucianodato", "Noise-reduction LV2 suite distributed as a Linux archive."),
						genericArchivePackage("easyssp", "DISTRHO Ports", "Visualization utility distributed as a Linux LV2 bundle."),
						genericArchivePackage("stereo-source-separator", "DISTRHO Ports", "Stereo source-separation processor distributed as a Linux LV2 bundle."),
						genericArchivePackage("dpf-plugins", "DISTRHO", "Multi-plugin bundle distributed as a Linux archive with CLAP, VST3, and LV2 targets."),
						genericArchivePackage("arctican-plugins", "DISTRHO Ports", "Utility plugin suite distributed as a Linux LV2 archive."),
						genericArchivePackage("drowaudio-plugins", "DISTRHO Ports", "Plugin suite distributed as a Linux LV2 archive."),
						genericArchivePackage("juced-plugins", "DISTRHO Ports", "Legacy plugin suite distributed as a Linux LV2 archive."),
						genericArchivePackage("ndc-plugins", "DISTRHO", "Creative effect suite distributed as a Linux LV2 archive."),
						genericArchivePackage("tal-plugins", "DISTRHO Ports", "Legacy TAL bundle distributed as a Linux LV2 archive."),
						alienDebPackage("chow-tape-model", "Chowdhury DSP", "Analog tape model distributed as a Linux Debian package."),
						alienDebPackage("chow-multitool", "Chowdhury DSP", "Utility plugin collection distributed as a Linux Debian package."),
						genericArchivePackage("mpe-emulator", "Attila M. Magyar", "MIDI processor for adding MPE-style expression mappings to ordinary controllers."),
						genericArchivePackage("zl-splitter", "ZL Audio", "Open-source splitter plugin distributed as Linux VST3 and LV2 bundles."),
						genericArchivePackage("tal-dac", "TAL Software", "Commercial lo-fi converter plugin distributed as Linux CLAP, VST3, and VST2 targets."),
						genericArchivePackage("intersect", "tucktuckg00se", "Sample slicer instrument packaged as a VST3 archive."),
						genericArchivePackage("spectrus", "Morphulus", "Multi-effect processor distributed as Linux VST3 and LV2 bundles."),
						genericArchivePackage("warp-core", "Manas World", "Pitch-focused processor distributed as Linux LV2 and VST3 bundles."),
						genericArchivePackage("chord-analyzer", "dusk audio", "Chord analysis plugin distributed as Linux LV2 and VST3 bundles."),
						{
							ID:          "zam-plugins",
							Name:        "Zam Plugin Suite",
							Vendor:      "ZamAudio",
							Summary:     "LV2 effect suite distributed as a Linux archive with multiple plugin bundles.",
							Description: "Downloads the upstream Linux archive and installs the contained LV2 plugin bundles into the current user's plugin directories.",
							Notes: []string{
								"Does not require sudo.",
								"The suite ships multiple LV2 bundles, so uninstall uses wildcard cleanup across the installed Zam plugin directories.",
							},
							Links: linkForID("zam-plugins"),
							InstalledMarkers: []string{
								".lv2/Zam*.lv2",
							},
							InstallActions: []Action{
								{Title: "Install Zam Plugin Suite", Exec: archiveInstall("zam-plugins")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Zam Plugin Suite", Exec: script("uninstall-zam-plugins.sh")},
							},
						},
					},
				},
			},
		},
		{
			ID:          "utilities",
			Name:        "Utilities",
			Description: "Audio-adjacent helpers and diagnostics.",
			Accent:      "#c084fc",
			Subcategories: []*Subcategory{
				{
					ID:          "creative-and-desktop",
					Name:        "Creative & Desktop",
					Description: "Standalone composition and utility apps that complement the plugin catalog.",
					Packages: []*Package{
						appImagePackage("ossia-score", "ossia", "Free, open-source intermedia sequencer for scripting interactive scenarios.", "ossia"),
						{
							ID:          "bambootracker",
							Name:        "BambooTracker",
							Vendor:      "BambooTracker",
							Summary:     "Music tracker for the Yamaha YM2608 sound chip.",
							Description: "Downloads the upstream BambooTracker ZIP into the current user's local Caracal app directory, then creates a user-local wrapper and desktop entry.",
							Notes: []string{
								"Does not require sudo.",
								"Runs from a user-local portable app directory so it works cleanly on immutable systems.",
							},
							Links: linkForID("bambootracker"),
							InstalledMarkers: []string{
								".local/share/caracal-software-installer/apps/bambootracker/current/bin/BambooTracker",
								".local/bin/bambootracker",
								".local/share/applications/bambootracker.desktop",
							},
							InstallActions: []Action{
								{Title: "Install BambooTracker", Exec: portableZipInstall("bambootracker", "BambooTracker", "bambootracker", "bambootracker", "Cross-platform music tracker for the Yamaha YM2608 sound chip")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall BambooTracker", Exec: portableZipUninstall("bambootracker", "bambootracker", "bambootracker")},
							},
						},
						appImagePackage("milkytracker", "MilkyTracker", "Music creation tool inspired by Fast Tracker 2.", "milky"),
						appImagePackage("musescore-studio", "MuseScore", "Standalone notation and scoring app distributed as a Linux AppImage.", "musescore"),
						{
							ID:          "declick",
							Name:        "Declick",
							Vendor:      "Michael Wahl",
							Summary:     "Audio restoration utility built from the upstream source tarball into /usr/local/bin.",
							Description: "Downloads the upstream Declick source tarball, builds it with make, and installs the resulting command-line utility into /usr/local/bin.",
							Notes: []string{
								"Requires sudo because upstream installs to /usr/local/bin.",
								"Builds from source, so make and a working native build toolchain are required on the target system.",
							},
							Links: linkForID("declick"),
							InstalledMarkers: []string{
								"/usr/local/bin/declick",
							},
							InstallActions: []Action{
								{Title: "Install Declick", Exec: sudoScript("install-declick.sh")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Declick", Exec: sudoScript("uninstall-declick.sh")},
							},
						},
					},
				},
				{
					ID:          "system-tuning",
					Name:        "System Tuning",
					Description: "Checks and helpers for realtime audio setup.",
					Packages: []*Package{
						{
							ID:          "rtcqs",
							Name:        "RTCQS",
							Vendor:      "rtcqs",
							Summary:     "Realtime Configuration Quick Scan CLI and GUI.",
							Description: "Creates a user-local virtualenv under ~/.local/share/caracal-os/rtcqs, publishes wrapper commands in ~/.local/bin, and adds a desktop launcher for rtcqs_gui.",
							Notes: []string{
								"Does not require sudo.",
								"Installs into the current user's home directory.",
							},
							Links: linkForID("rtcqs"),
							InstalledMarkers: []string{
								".local/bin/rtcqs",
								".local/share/applications/rtcqs-gui.desktop",
							},
							InstallActions: []Action{
								{Title: "Install RTCQS", Exec: script("install-rtcqs.sh")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall RTCQS", Exec: script("uninstall-rtcqs.sh")},
							},
						},
					},
				},
			},
		},
	}

	for _, category := range categories {
		for _, subcategory := range category.Subcategories {
			for _, pkg := range subcategory.Packages {
				entry := mustEntry(pkg.ID)
				pkg.SoftwareTypes = softwareTypesForPackage(pkg)
				pkg.OpenSource = boolFieldForID(pkg.ID, "open_source", false)
				pkg.HasFreeVersion = boolFieldForID(pkg.ID, "has_free_version", true)
				pkg.License = licenseForEntry(entry, pkg.OpenSource)
				pkg.Links = linksForEntry(entry)
				if !pkg.OpenSource {
					pkg.InstallActions = nil
					pkg.ExternalActionURL = strings.TrimSpace(entry["project_website"])
					if pkg.ExternalActionURL != "" {
						pkg.AvailabilityNote = "This proprietary package opens the developer website instead of downloading from Caracal."
					} else {
						pkg.AvailabilityNote = "This proprietary package is listed for reference only until a developer website is added."
					}
				}
			}
		}
	}

	return categories
}

func boolField(entry downloadindex.Entry, field string, defaultValue bool) bool {
	value := strings.TrimSpace(strings.ToLower(entry[field]))
	if value == "" {
		return defaultValue
	}
	switch value {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return defaultValue
	}
}

func linksForEntry(entry downloadindex.Entry) []Link {
	openSource := boolField(entry, "open_source", false)
	var links []Link
	if entry["project_website"] != "" {
		links = append(links, Link{Label: "Site", URL: entry["project_website"]})
	}
	if openSource && entry["url"] != "" {
		links = append(links, Link{Label: "Download", URL: entry["url"]})
	}
	if openSource && entry["repo_url"] != "" {
		links = append(links, Link{Label: "Source", URL: entry["repo_url"]})
	}
	return links
}

func licenseForEntry(entry downloadindex.Entry, openSource bool) *License {
	if !openSource {
		return nil
	}

	label := normalizeLicenseLabel(entry["license_type"])
	if label == "" {
		return nil
	}

	return &License{
		Label: label,
		URL:   strings.TrimSpace(entry["link_to_license"]),
		Kind:  licenseKind(label),
	}
}

func normalizeLicenseLabel(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "":
		return ""
	case "GPL", "GPL3", "GPL3.0", "GPL-3", "GPL-3.0":
		return "GPL-3.0"
	case "GPL2", "GPL-2", "GPL-2.0":
		return "GPL-2.0"
	case "AGPL", "AGPL3", "AGPL3.0", "AGPL-3", "AGPL-3.0":
		return "AGPL-3.0"
	case "LGPL", "LGPL3", "LGPL3.0", "LGPL-3", "LGPL-3.0":
		return "LGPL-3.0"
	case "APACHE", "APACHE2", "APACHE-2", "APACHE-2.0":
		return "Apache-2.0"
	case "MIT":
		return "MIT"
	case "BSD":
		return "BSD"
	case "VARIOUS":
		return "Various"
	default:
		return strings.TrimSpace(raw)
	}
}

func licenseKind(label string) string {
	kind := strings.ToLower(label)
	kind = strings.ReplaceAll(kind, ".", "-")
	kind = strings.ReplaceAll(kind, " ", "-")
	return kind
}

func CountPackages(categories []*Category) int {
	total := 0
	for _, category := range categories {
		for _, subcategory := range category.Subcategories {
			total += len(subcategory.Packages)
		}
	}
	return total
}
