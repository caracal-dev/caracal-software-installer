# Caracal Software Installer

`caracal-software-installer` is a Go TUI built with `tview` for guided post-install setup on Caracal OS. It presents optional DAWs, instruments, plugins, and audio utilities in browsable categories and lets the user queue multiple installs in one pass.

## Current catalog

- DAWs
  - REAPER
  - Renoise
  - Bitwig Studio
- Virtual Instruments
  - Warmplace: SunVox, Virtual ANS, Fractal Bits
  - Open Synths: Cardinal, Surge XT, Wavetable, OB-Xf, Odin2, TAL-Noisemaker, Yoshimi
  - Samplers & Players: Loopino, Decent Sampler
  - rncbc Instruments: Synthv1, Samplv1, Padhv1
  - Drums & Percussion: jDrummer, Drumkv1, Drum Locker, Drum Groove Pro, Black Widow Drums
- Effects
  - Amp & Guitar: Amp Locker, BYOD, Neural Amp Modeler, AIDA-X
  - Mixing & Channel Strip: Mix Locker, The Trick, Polarity, NineStrip
  - Reverb & Spatial: Dragonfly Reverb, WetDelay, WetReverb
  - Creative & Utility: INTERSECT, Spectrus, WarpCore, Zam Plugin Suite
- Utilities
  - RTCQS

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
go run ./cmd/caracal-software-installer
```

The app looks for installer scripts in:

1. `CARACAL_INSTALLER_SCRIPT_DIR`
2. `/usr/lib/caracal-software-installer/scripts`
3. `scripts/` in the current repo or a parent directory

Most package installs write to `/opt`, `/usr/local`, or the current user's home directory so they work on an atomic Caracal system without rpm layering.

## Contributing
Pull requests welcome. Just create a feature branch and submit a pull request with details about the change, what software you are adding to the catalog etc.

If you are currently running a Fedora Atomic image, you can clone this repo and run it locally and see if the installation you added work.

## TODO
1. ~~Add an uninstall option~~
2. Add sudo password and installation processing to happen within the program program so the user does not exit and then return.
3. Move all the Caracal default plugins and music software here and embed install into Caracal OS
4. Add more plugins
5. Fix general jank
