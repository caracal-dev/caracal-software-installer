# Thumbnail Confidence Report (2026-08-14)

All 206 download-index.csv entries now have a `frontend/dist/assets/images/thumbnails/<id>.webp`
thumbnail. All are WebP, 800x440, dark background (#1a192e) matching the existing set.

Confidence ratings for the 65 newly-added thumbnails:

- **HIGH** — image is the plugin UI screenshot taken from the project's own repo/README.
- **MEDIUM** — image is a GitHub/Codeberg social-preview card (repo name + description),
  used when no UI screenshot exists in the repo. Correct subject, but not the actual UI.
- **LOW** — fallback card for a tool with no UI to screenshot at all.

## New thumbnails (65)

| id | confidence | source |
|----|-----------|--------|
| aeolus | HIGH | repo `docs/images/screenshot.png` |
| aether | HIGH | repo `screenshot.png` |
| agnarohm-v2 | HIGH | copied from existing `agnorohm-v2.webp` (CSV spelling variant) |
| ami-sampler | HIGH | repo `Res/AmiLogo.png` (logo, no UI shot in repo) |
| apu-dynamics-optimizer | HIGH | first-run fetch (APU site) |
| audible-planets | MEDIUM | GitHub social preview (repo has no UI screenshot) |
| chow-kick | HIGH | repo `manual/s...` screenshot |
| chow-matrix | HIGH | repo `manual/...` screenshot |
| chow-multitool | HIGH | copied from existing `chow-multi-tool.webp` (name variant) |
| cloudreverb | HIGH | repo `screenshot.png` |
| consolex | MEDIUM | GitHub social preview (repo has no UI screenshot) |
| curvessor | HIGH | repo `Images/scre...` screenshot |
| filtr | HIGH | repo `doc/filtr.png` |
| fire | HIGH | repo `Fire1.png` |
| floe-clap | HIGH | repo README website image |
| floe-vst | HIGH | repo README website image |
| fluida | HIGH | repo `Fluida.png` |
| gate-12 | HIGH | repo `doc/gate12.png` |
| geonkick-basic | HIGH | repo `data/screenshot...` |
| jc303 | HIGH | repo `img/jc303.png` |
| js80p | HIGH | repo `js80p.png` |
| just-a-sample | HIGH | repo `Assets/P...` screenshot |
| k-meter | HIGH | repo `doc/include/...` screenshot |
| lamb | HIGH | copied from existing `lamb-rs.webp` (repo/name variant) |
| lmms | MEDIUM | GitHub social preview (repo tree has no UI screenshot) |
| loopino-clap | HIGH | copied from existing `loopino.webp` (covers both formats) |
| loopino-vst2 | HIGH | copied from existing `loopino.webp` (covers both formats) |
| master-me | HIGH | repo `img/...` screenshot |
| mixcompare | MEDIUM | GitHub social preview (only UI asset found was a switch graphic) |
| panacea | HIGH | repo `preview.png` |
| peakeater | HIGH | repo `assets/screenshot...` |
| plasma | HIGH | repo `CompressedPreview...` |
| podcast-plugins | HIGH | repo `g...` screenshot |
| pult-eq | HIGH | repo `pulteq.png` |
| qdelay | HIGH | repo `doc/qdelay.png` |
| reevr | HIGH | repo `doc/reevr.png` |
| resonarium-effect | HIGH | project website `resonarium.png` |
| resonarium-instrument | HIGH | project website `resonarium.png` |
| retuner | HIGH | first-run fetch (kushview) |
| rtcqs | LOW | Codeberg summary card (CLI tool, no GUI exists) |
| sas | HIGH | repo `info/sas.png` |
| setekh | HIGH | repo `docs/setekh_...` |
| shin-ronin | HIGH | copied from existing `shinronin.webp` (name variant) |
| sirial | HIGH | repo `doc/sirial.png` |
| six-sines | HIGH | repo `doc/sxsn_an...` |
| solidarp | HIGH | repo `docs/solidArp-UI...` |
| solidutility | HIGH | repo `docs/utility-ui.png` |
| songbird | MEDIUM | GitHub social preview of source repo (release repo has none) |
| spectral-shift | HIGH | repo `resources/...` screenshot |
| stomptuner | HIGH | repo `StompTuner.png` |
| suboctb | HIGH | Codeberg summary card + repo (no UI shot) |
| syndicate | MEDIUM | GitHub social preview of source repo (release repo has none) |
| tal-j8x | HIGH | copied from existing `tal-j-8x.webp` (name variant) |
| terrain | HIGH | repo `ReadmeSo...` screenshot |
| testtone | MEDIUM | GitHub social preview (no UI screenshot in repo) |
| time-12 | HIGH | repo `doc/time12.png` |
| tonesmifteq | HIGH | copied from existing `toneshifteq.webp` (CSV typo variant) |
| tumult | HIGH | repo `tumult.png` |
| two-filters | HIGH | repo `doc/TF.png` |
| valentine | HIGH | repo `docs/va...` screenshot |
| wstd-mseq | HIGH | copied from existing `wstd_mseq.webp` (name variant) |
| xolotl | HIGH | repo README `Xolotl.png` |
| zerocomp | MEDIUM | GitHub social preview (no UI screenshot in repo) |
| zeroeq | MEDIUM | GitHub social preview (no UI screenshot in repo) |

## Summary

- **54 HIGH** — plugin UI / real screenshot or copied existing correct image.
- **10 MEDIUM** — GitHub social preview card (lmms, mixcompare, audible-planets,
  consolex, songbird, syndicate, testtone, zerocomp, zeroeq, actuate). These are
  correct project branding but may not show the plugin UI.
- **1 LOW** — rtcqs (CLI tool with no GUI; used Codeberg repo card).

## Notes

- Two entries (`songbird`, `syndicate`) point at a shared release repo
  (`jd-13/wea-releases`) with no images; thumbnails taken from their actual source
  repos (`jd-13/Songbird-Formant-Filter`, `jd-13/syndicate-mirror`).
- `resonarium-instrument` and `resonarium-effect` share the same image
  (`resonarium.png` from the project site) since both plugins ship together.
- `floe-vst`/`floe-clap` share the same README image (same plugin, two formats).
- 6 pre-existing thumbnails (guitarix, lamb-rs, lamb, neuralrack, squeezer,
  the-victor) were portrait 794x1123; re-cropped to 800x440 dark-background spec.
- Removed a wrong download (PayPal donate button) that matched `ami-sampler`;
  replaced with the plugin's AmiLogo.
