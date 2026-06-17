[![Codacy Badge](https://app.codacy.com/project/badge/Grade/59615cb4f27f4332b067abca3ca7c12f)](https://app.codacy.com/gh/caracal-dev/caracal-software-installer/dashboard?utm_source=gh&utm_medium=referral&utm_content=&utm_campaign=Badge_grade)

# Caracal Software Installer

`caracal-software-installer` is a Go installer app for guided post-install setup on Caracal OS. It presents optional DAWs, instruments, plugins, and audio utilities in browsable categories and lets the user queue multiple installs in one pass.

The repo now includes:

- a command-line frontend at `cmd/caracal`
- a `tview` terminal UI at `cmd/caracal-software-installer`
- a Wails desktop GUI at the repo root / `main.go`

## Screenshots

### Desktop GUI

![Caracal Software Installer desktop GUI](assets/screenshots/gui.png)

### Terminal UI

![Caracal Software Installer terminal UI](assets/screenshots/tui.png)

## Current catalog

- DAWs
  - REAPER
  - Renoise
  - Bitwig Studio
- Virtual Instruments
  - Open Synths: SunVox, Virtual ANS, Cardinal, Surge XT, Wavetable, OB-Xf, Odin2, TAL-Noisemaker, Dexed, JuceOPL, OBXD, Vex, Wolpertinger, Yoshimi, Ensoniq SD 1, KR106, TB4006, Suboctb, Floe (VST3), Floe (CLAP)
  - Samplers & Players: Loopino, Decent Sampler
  - rncbc Instruments: Synthv1, Samplv1, Padhv1
  - Drums & Percussion: jDrummer, Drumkv1, Drum Locker, Drum Groove Pro, Black Widow Drums
- Effects
  - Amp & Guitar: Amp Locker, BYOD, Neural Amp Modeler, AIDA-X
  - Mixing & Channel Strip: Mix Locker, The Trick, Polarity, NineStrip, LUFS Meter, Luftikus
  - Reverb & Spatial: Dragonfly Reverb, KlangFalter, MVerb, Pitched Delay, WetDelay, WetReverb
  - Creative & Utility: Noise Repellent, EasySSP, Stereo Source Separator, DPF Plugins, Arctican Plugins, dRowAudio Plugins, Juced Plugins, NDC Plugins, TAL Plugins, INTERSECT, Spectrus, WarpCore, Zam Plugin Suite
- Utilities
  - Creative & Desktop: MuseScore Studio, Declick
  - System Tuning: RTCQS

The UI is catalog-driven, and download URLs plus related archive metadata now live in `data/download-index.csv`. The catalog and helper scripts resolve package metadata from that index so link updates stay spreadsheet-friendly.

`catalog-links.csv` is generated from the same catalog metadata and can be refreshed with:

```bash
env GOCACHE=/tmp/go-build-cache GOMODCACHE=/tmp/go-mod-cache go run ./cmd/export-catalog-links > catalog-links.csv
```

The download index can be validated from a repo checkout with:

```bash
scripts/download-index validate
scripts/download-index validate --check-urls
```

## Development

```bash
go mod tidy
go run ./cmd/caracal help
go run ./cmd/caracal-software-installer
```

Packaged command layout:

- `caracal install <id>` installs software by catalog id
- `caracal scan` checks the download index for broken links
- `caracal launch` starts the Wails desktop frontend
- `caracal list` lists available software ids, with filters such as `--daws`, `--effects`, `--synths`, `--format vst`, `--open-source`, and `--installed`
- `caracal-software-installer` launches the terminal UI
- `caracal-software-installer-gui` launches the Wails desktop frontend
- the `.desktop` launcher targets the GUI build

To run the Wails desktop frontend manually from Go:

```bash
go run -tags dev .
```

To use the normal Wails workflow:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails dev
wails build
```

The repo-level `wails.json` points Wails at `frontend/` and `frontend/dist/`, and the frontend package uses a minimal `npm run build` check so the static GUI assets work with Wails without needing a separate SPA toolchain.

On Fedora Atomic / Universal Blue style systems, these helper wrappers are the safer entrypoint because they add the `webkit2_41` tag automatically when the host exposes the newer WebKit package name:

```bash
./scripts/wails-dev.sh
./scripts/wails-build.sh
```

Switch the packaged desktop icon by copying one of the PNGs in `build/icons/` to `build/appicon.png`:

```bash
./scripts/switch-app-icon
./scripts/switch-app-icon caracal-lakers.png
```

On Linux, root-requiring installer actions are routed through `pkexec` so they can prompt graphically instead of assuming an interactive terminal.

The app looks for installer scripts in:

1. `CARACAL_INSTALLER_SCRIPT_DIR`
2. `/usr/lib/caracal-software-installer/scripts`
3. `scripts/` in the current repo or a parent directory

Most package installs write to `/opt`, `/usr/local`, or the current user's home directory so they work on an atomic Caracal system without rpm layering.

## Contributing

Pull requests welcome. Just create a feature branch and submit a pull request with details about the change, what software you are adding to the catalog etc.

If you are currently running a Fedora Atomic image, you can clone this repo and run it locally and see if the installation you added work.

### Adding generic software

For simple additions, the installer already has generic scripts for plugin archives, Debian packages, and AppImages. More custom installs may need their own script, but please use the generic path when the upstream package is a normal Linux VST, CLAP, LV2, AppImage, or `.deb` payload.

1. Add a row to `data/download-index.csv`.
   - Required basics: `id`, `name`, `url`, `project_website`, `dl_within_app`, `open_source`, and `has_free_version`.
   - For plugins, set `formats` to any comma-separated mix of `clap`, `vst`, `vst3`, and `lv2`.
   - For plugin archives and `.deb` files, set `primary_bundle_name` when the installed bundle name differs from the package id. Add `data_dir_name` and `data_target_name` if the plugin ships a required data folder.
   - Add `version`, `category`, `link_to_license`, and `license_type` when known.
2. Add the catalog entry in `internal/catalog/catalog.go` under the right category.
   - Use `genericArchivePackage("id", "Vendor", "Summary.")` for generic VST, VST3, CLAP, or LV2 archives/direct plugin downloads.
   - Use `alienDebPackage("id", "Vendor", "Summary.")` for a `.deb` that can be extracted into user-local plugin paths.
   - Use `appImagePackage("id", "Vendor", "Summary.", "search-token")` for an AppImage that can be integrated with AppImageLauncher.
3. Validate the index:

```bash
scripts/download-index validate
```

Use `scripts/download-index validate --check-urls` when you also want to check that upstream URLs still resolve.

Proprietary software needs extra care. We can't redistribute proprietary software so are not allowed to download the package directly, so set `dl_within_app` to `false`, provide a `project_website` link so the UI can launch the vendor website or download page, and include an install/uninstall script path for the user's manual download when needed. Proprietary entries should still explain whether there is a free/demo version via `has_free_version`.

Contributions of more generic installation scripts are welcome too. Build-from-source helpers would be especially useful; the current generic coverage is mostly plugin archives, extracted `.deb` plugin payloads, and AppImages.
