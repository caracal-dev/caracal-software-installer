package catalog

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/caracal-os/caracal-software-installer/internal/downloadindex"
)

type Action struct {
	Title string
	Exec  []string
}

type Link struct {
	Label string
	URL   string
}

type Package struct {
	ID               string
	Name             string
	Vendor           string
	Summary          string
	Description      string
	Notes            []string
	Links            []Link
	AvailabilityNote string
	InstalledMarkers []string
	InstallActions   []Action
	UninstallActions []Action
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
			entry["primary_bundle_name"],
			entry["formats"],
			entry["data_target_name"],
		})
		return append([]string{"bash", filepath.Join(scriptDir, "uninstall-plugin-archive.sh")}, args...)
	}
	sourceInstall := func(id string, projectName string) []string {
		return script("install-source-plugin.sh", id, mustEntry(id)["name"], projectName)
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
		markers := make([]string, 0, len(formats)+1)

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
		entry := mustEntry(id)
		var links []Link
		if entry["url"] != "" {
			links = append(links, Link{Label: "Download", URL: entry["url"]})
		}
		if entry["repo_url"] != "" {
			links = append(links, Link{Label: "Source", URL: entry["repo_url"]})
		}
		return links
	}
	genericArchivePackage := func(id string, vendor string, summary string) *Package {
		entry := mustEntry(id)
		name := entry["name"]
		return &Package{
			ID:          id,
			Name:        name,
			Vendor:      vendor,
			Summary:     summary,
			Description: fmt.Sprintf("Downloads the upstream Linux archive and installs the contained %s into the current user's plugin directories.", archiveTargets(id)),
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

	return []*Category{
		{
			ID:          "daws",
			Name:        "DAWs",
			Description: "Commercial workstation installs that complement the default Caracal toolset.",
			Accent:      "#7dd3fc",
			Subcategories: []*Subcategory{
				{
					ID:          "commercial-daws",
					Name:        "Commercial DAWs",
					Description: "Optional workstation installs that currently live in Caracal's post-install flow.",
					Packages: []*Package{
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
			},
		},
		{
			ID:          "virtual-instruments",
			Name:        "Virtual Instruments",
			Description: "Synths, modular environments, drums, and sample players available as optional installs.",
			Accent:      "#f59e0b",
			Subcategories: []*Subcategory{
				{
					ID:          "warmplace",
					Name:        "Warmplace",
					Description: "Portable desktop synths and experimental tools from Alexander Zolotov.",
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
							ID:          "fractal-bits",
							Name:        "Fractal Bits",
							Vendor:      "Warmplace",
							Summary:     "Fractal drum synth desktop build currently distributed through a paid upstream post.",
							Description: "The desktop Linux build is listed by Warmplace, but the current upstream download is a purchase-gated Boosty post rather than a direct public ZIP archive.",
							Notes: []string{
								"Listed here so the Warmplace section reflects the broader lineup.",
								"Once a stable public ZIP URL or a purchase flow is defined, this can be turned into a first-class installer entry.",
							},
							AvailabilityNote: "Current desktop download is purchase-gated upstream, so there is no unattended installer script yet.",
						},
					},
				},
				{
					ID:          "open-synths",
					Name:        "Open Synths",
					Description: "Native instruments that fit well into the Caracal plugin path layout.",
					Packages: []*Package{
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
						{
							ID:          "wavetable",
							Name:        "Wavetable",
							Vendor:      "FigBug",
							Summary:     "Two-oscillator wavetable synth with VST, VST3, and LV2 targets.",
							Description: "Downloads the upstream Linux archive and installs the contained VST2, VST3, and LV2 payloads into the current user's plugin directories.",
							Notes: []string{
								"Does not require sudo.",
								"Installed as a user-local plugin set so it works cleanly on immutable systems.",
							},
							Links: linkForID("wavetable"),
							InstalledMarkers: []string{
								".vst/Wavetable.so",
								".vst3/Wavetable.vst3",
								".lv2/Wavetable.lv2",
							},
							InstallActions: []Action{
								{Title: "Install Wavetable", Exec: archiveInstall("wavetable")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Wavetable", Exec: archiveUninstall("wavetable")},
							},
						},
						{
							ID:          "ob-xf",
							Name:        "OB-Xf",
							Vendor:      "Surge Synth Team",
							Summary:     "Open-source OB-style synth distributed as Linux plugin bundles.",
							Description: "Downloads the upstream Linux archive and installs the contained VST3 and LV2 bundles into the current user's plugin directories.",
							Notes: []string{
								"Does not require sudo.",
								"Installed as a user-local plugin so it works cleanly on immutable systems.",
							},
							Links: linkForID("ob-xf"),
							InstalledMarkers: []string{
								".vst3/OB-Xf.vst3",
								".lv2/OB-Xf.lv2",
							},
							InstallActions: []Action{
								{Title: "Install OB-Xf", Exec: archiveInstall("ob-xf")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall OB-Xf", Exec: archiveUninstall("ob-xf")},
							},
						},
						{
							ID:          "odin2",
							Name:        "Odin2",
							Vendor:      "TheWaveWarden",
							Summary:     "Hybrid synth distributed as a Linux archive with CLAP and VST3 targets.",
							Description: "Downloads the upstream Linux archive and installs the contained CLAP and VST3 bundles into the current user's plugin directories.",
							Notes: []string{
								"Does not require sudo.",
								"Installed as a user-local plugin so it works cleanly on immutable systems.",
							},
							Links: linkForID("odin2"),
							InstalledMarkers: []string{
								".clap/Odin2.clap",
								".vst3/Odin2.vst3",
							},
							InstallActions: []Action{
								{Title: "Install Odin2", Exec: archiveInstall("odin2")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Odin2", Exec: archiveUninstall("odin2")},
							},
						},
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
								{Title: "Install Yoshimi", Exec: sourceInstall("yoshimi", "yoshimi")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Yoshimi", Exec: sourceUninstall("yoshimi", "Yoshimi")},
							},
						},
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
					},
				},
				{
					ID:          "rncbc-instruments",
					Name:        "rncbc Instruments",
					Description: "Classic Linux instrument plugins built from source into the current user's home directory.",
					Packages: []*Package{
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
								{Title: "Install Synthv1", Exec: sourceInstall("synthv1", "synthv1")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Synthv1", Exec: sourceUninstall("synthv1", "Synthv1")},
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
								{Title: "Install Samplv1", Exec: sourceInstall("samplv1", "samplv1")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Samplv1", Exec: sourceUninstall("samplv1", "Samplv1")},
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
								{Title: "Install Padhv1", Exec: sourceInstall("padhv1", "padhv1")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Padhv1", Exec: sourceUninstall("padhv1", "Padhv1")},
							},
						},
					},
				},
				{
					ID:          "drums-and-percussion",
					Name:        "Drums & Percussion",
					Description: "Drum machines, drum instruments, and groove-oriented tools.",
					Packages: []*Package{
						{
							ID:          "jdrummer",
							Name:        "jDrummer",
							Vendor:      "jmantra",
							Summary:     "Drum instrument distributed as a Linux VST3 archive.",
							Description: "Downloads the upstream jDrummer Linux archive and installs its VST3 bundle into the current user's plugin directories.",
							Notes: []string{
								"Does not require sudo.",
								"Installed as a user-local plugin so it works cleanly on immutable systems.",
							},
							Links: linkForID("jdrummer"),
							InstalledMarkers: []string{
								".vst3/jdrummer.vst3",
							},
							InstallActions: []Action{
								{Title: "Install jDrummer", Exec: archiveInstall("jdrummer")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall jDrummer", Exec: archiveUninstall("jdrummer")},
							},
						},
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
								{Title: "Install Drumkv1", Exec: sourceInstall("drumkv1", "drumkv1")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Drumkv1", Exec: sourceUninstall("drumkv1", "Drumkv1")},
							},
						},
						{
							ID:          "drum-locker",
							Name:        "Drum Locker",
							Vendor:      "Audio Assault",
							Summary:     "Drum and groove production plugin installed from the official Linux archive.",
							Description: "Downloads Drum Locker and installs its VST3 and LV2 bundles plus its Audio Assault data pack into the current user's home directory.",
							Notes: []string{
								"Does not require sudo.",
								"Installed as a user-local plugin so it works cleanly on immutable systems.",
							},
							Links: linkForID("drum-locker"),
							InstalledMarkers: []string{
								".vst3/Drum Locker.vst3",
								".lv2/Drum Locker.lv2",
								"Audio Assault/PluginData/Audio Assault/DrumLockerData",
							},
							InstallActions: []Action{
								{Title: "Install Drum Locker", Exec: archiveInstall("drum-locker")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Drum Locker", Exec: archiveUninstall("drum-locker")},
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
						{
							ID:          "amp-locker",
							Name:        "Amp Locker",
							Vendor:      "Audio Assault",
							Summary:     "Amp sim platform installed from the official Linux archive.",
							Description: "Downloads Amp Locker and installs its VST3 and LV2 bundles plus its Audio Assault data pack into the current user's home directory.",
							Notes: []string{
								"Does not require sudo.",
								"Installed as a user-local plugin so it works cleanly on immutable systems.",
							},
							Links: linkForID("amp-locker"),
							InstalledMarkers: []string{
								".vst3/Amp Locker.vst3",
								".lv2/Amp Locker.lv2",
								"Audio Assault/PluginData/Audio Assault/AmpLockerData",
							},
							InstallActions: []Action{
								{Title: "Install Amp Locker", Exec: archiveInstall("amp-locker")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Amp Locker", Exec: archiveUninstall("amp-locker")},
							},
						},
						{
							ID:          "byod",
							Name:        "BYOD",
							Vendor:      "Chowdhury DSP",
							Summary:     "Modular pedalboard and amp chain plugin distributed as a Linux archive.",
							Description: "Downloads the upstream Linux package and installs the contained CLAP, VST3, and LV2 bundles into the current user's plugin directories.",
							Notes: []string{
								"Does not require sudo.",
								"Installed as a user-local plugin so it works cleanly on immutable systems.",
							},
							Links: linkForID("byod"),
							InstalledMarkers: []string{
								".clap/BYOD.clap",
								".vst3/BYOD.vst3",
								".lv2/BYOD.lv2",
							},
							InstallActions: []Action{
								{Title: "Install BYOD", Exec: archiveInstall("byod")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall BYOD", Exec: archiveUninstall("byod")},
							},
						},
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
					},
				},
				{
					ID:          "mixing-and-channel-strip",
					Name:        "Mixing & Channel Strip",
					Description: "Mix-focused processors and channel-strip style tools.",
					Packages: []*Package{
						{
							ID:          "mix-locker",
							Name:        "Mix Locker",
							Vendor:      "Audio Assault",
							Summary:     "Channel-strip and mix processing platform installed from the official Linux archive.",
							Description: "Downloads Mix Locker and installs its VST3 and LV2 bundles plus its Audio Assault data pack into the current user's home directory.",
							Notes: []string{
								"Does not require sudo.",
								"Installed as a user-local plugin so it works cleanly on immutable systems.",
							},
							Links: linkForID("mix-locker"),
							InstalledMarkers: []string{
								".vst3/Mix Locker.vst3",
								".lv2/Mix Locker.lv2",
								"Audio Assault/PluginData/Audio Assault/MixLockerData",
							},
							InstallActions: []Action{
								{Title: "Install Mix Locker", Exec: archiveInstall("mix-locker")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall Mix Locker", Exec: archiveUninstall("mix-locker")},
							},
						},
						genericArchivePackage("the-trick", "Mouse Plugins", "Focused EQ processor distributed as a Linux VST3 archive."),
						genericArchivePackage("polarity", "Polarity", "Spectral compressor plugin packaged with CLAP and VST3 targets."),
						genericArchivePackage("nine-strip", "blablack", "Channel-strip processor distributed as Linux VST3 and LV2 bundles."),
					},
				},
				{
					ID:          "reverb-and-spatial",
					Name:        "Reverb & Spatial",
					Description: "Spatial processors and reverb suites.",
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
						genericArchivePackage("wet-delay", "yonie", "Delay plugin distributed as a Linux VST3 archive."),
						genericArchivePackage("wet-reverb", "yonie", "Reverb plugin distributed as a Linux VST3 archive."),
					},
				},
				{
					ID:          "creative-and-utility",
					Name:        "Creative & Utility",
					Description: "Sound-design tools, utilities, and unusual processors.",
					Packages: []*Package{
						{
							ID:          "intersect",
							Name:        "INTERSECT",
							Vendor:      "tucktuckg00se",
							Summary:     "Sample slicer instrument packaged as a VST3 archive.",
							Description: "Downloads INTERSECT and installs its VST3 bundle into the current user's plugin directories.",
							Notes: []string{
								"Does not require sudo.",
								"Installed as a user-local plugin so it works cleanly on immutable systems.",
							},
							Links: linkForID("intersect"),
							InstalledMarkers: []string{
								".vst3/INTERSECT.vst3",
							},
							InstallActions: []Action{
								{Title: "Install INTERSECT", Exec: archiveInstall("intersect")},
							},
							UninstallActions: []Action{
								{Title: "Uninstall INTERSECT", Exec: archiveUninstall("intersect")},
							},
						},
						genericArchivePackage("spectrus", "Morphulus", "Multi-effect processor distributed as Linux VST3 and LV2 bundles."),
						genericArchivePackage("warp-core", "Manas World", "Pitch-focused processor distributed as Linux LV2 and VST3 bundles."),
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
