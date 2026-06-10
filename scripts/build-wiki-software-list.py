#!/usr/bin/env python3
"""Generate the Software-List.md wiki page from the catalog metadata.

Reads:
  - internal/catalog/catalog.go for category/subcategory layout and vendors
  - data/download-index.csv for license, formats, version, and install-source flags

Writes the assembled markdown to the path given on the command line (default:
caracal-software-installer.wiki/Software-List.md relative to the repo root).

Run from the repo root, or pass the output path explicitly. Used by the
release workflow to refresh the wiki page on tag pushes.
"""

from __future__ import annotations

import csv
import re
import sys
from collections import OrderedDict
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
CATALOG_GO = REPO_ROOT / "internal" / "catalog" / "catalog.go"
CSV_FILE = REPO_ROOT / "data" / "download-index.csv"
DEFAULT_OUT = REPO_ROOT / "caracal-software-installer.wiki" / "Software-List.md"


def load_entries() -> dict[str, dict[str, str]]:
    with CSV_FILE.open(newline="") as fh:
        reader = csv.DictReader(fh)
        return {row["id"]: row for row in reader}


def parse_catalog() -> list[dict[str, str]]:
    text = CATALOG_GO.read_text()
    start = text.index("categories := []*Category{")
    end = text.index("\n\tfor _, category := range categories {", start)
    body_lines = text[start:end].splitlines()

    cat_id_re = re.compile(r'^\t{3}ID:\s+"([^"]+)"')
    cat_name_re = re.compile(r'^\t{3}Name:\s+"([^"]+)"')
    sub_id_re = re.compile(r'^\t{5}ID:\s+"([^"]+)"')
    sub_name_re = re.compile(r'^\t{5}Name:\s+"([^"]+)"')
    inline_id_re = re.compile(r'^\t{7}ID:\s+"([^"]+)"')
    inline_name_re = re.compile(r'^\t{7}Name:\s+"([^"]+)"')
    inline_vendor_re = re.compile(r'^\t{7}Vendor:\s+"([^"]+)"')
    helper_re = re.compile(
        r'^\t{6}(appImagePackage|genericArchivePackage|alienDebPackage)\("([^"]+)",\s*"([^"]+)",\s*"([^"]+)"'
    )

    packages: list[dict[str, str]] = []
    cur_cat: tuple[str, str] | None = None
    cur_sub: tuple[str, str] | None = None

    i = 0
    while i < len(body_lines):
        line = body_lines[i]
        m = cat_id_re.match(line)
        if m:
            nxt = body_lines[i + 1] if i + 1 < len(body_lines) else ""
            nm = cat_name_re.match(nxt)
            cur_cat = (m.group(1), nm.group(1) if nm else m.group(1))
            cur_sub = None
            i += 1
            continue

        m = sub_id_re.match(line)
        if m:
            nxt = body_lines[i + 1] if i + 1 < len(body_lines) else ""
            nm = sub_name_re.match(nxt)
            cur_sub = (m.group(1), nm.group(1) if nm else m.group(1))
            i += 1
            continue

        if cur_cat and cur_sub:
            m = inline_id_re.match(line)
            if m:
                pkg_id = m.group(1)
                pkg_name = None
                vendor = None
                for j in range(i + 1, min(i + 6, len(body_lines))):
                    if pkg_name is None:
                        nm2 = inline_name_re.match(body_lines[j])
                        if nm2:
                            pkg_name = nm2.group(1)
                    if vendor is None:
                        vm = inline_vendor_re.match(body_lines[j])
                        if vm:
                            vendor = vm.group(1)
                packages.append(
                    {
                        "category_id": cur_cat[0],
                        "category_name": cur_cat[1],
                        "subcategory_id": cur_sub[0],
                        "subcategory_name": cur_sub[1],
                        "id": pkg_id,
                        "name": pkg_name or pkg_id,
                        "vendor": vendor or "",
                    }
                )
                i += 1
                continue

            m = helper_re.match(line)
            if m:
                _helper, pkg_id, vendor, _summary = m.groups()
                packages.append(
                    {
                        "category_id": cur_cat[0],
                        "category_name": cur_cat[1],
                        "subcategory_id": cur_sub[0],
                        "subcategory_name": cur_sub[1],
                        "id": pkg_id,
                        "name": "",
                        "vendor": vendor,
                    }
                )
                i += 1
                continue

        i += 1

    return packages


def bool_field(entry: dict[str, str], field: str, default: bool) -> bool:
    value = (entry.get(field) or "").strip().lower()
    if value in {"true", "1", "yes"}:
        return True
    if value in {"false", "0", "no"}:
        return False
    return default


LICENSE_TABLE = {
    "": "",
    "GPL": "GPL-3.0",
    "GPL3": "GPL-3.0",
    "GPL3.0": "GPL-3.0",
    "GPL-3": "GPL-3.0",
    "GPL-3.0": "GPL-3.0",
    "GPL2": "GPL-2.0",
    "GPL-2": "GPL-2.0",
    "GPL-2.0": "GPL-2.0",
    "AGPL": "AGPL-3.0",
    "AGPL3": "AGPL-3.0",
    "AGPL3.0": "AGPL-3.0",
    "AGPL-3": "AGPL-3.0",
    "AGPL-3.0": "AGPL-3.0",
    "LGPL": "LGPL-3.0",
    "LGPL3": "LGPL-3.0",
    "LGPL3.0": "LGPL-3.0",
    "LGPL-3": "LGPL-3.0",
    "LGPL-3.0": "LGPL-3.0",
    "APACHE": "Apache-2.0",
    "APACHE2": "Apache-2.0",
    "APACHE-2": "Apache-2.0",
    "APACHE-2.0": "Apache-2.0",
    "MIT": "MIT",
    "BSD": "BSD",
    "VARIOUS": "Various",
}


def normalize_license(raw: str) -> str:
    return LICENSE_TABLE.get(raw.strip().upper(), raw.strip())


FORMAT_LABELS = {"clap": "CLAP", "vst": "VST2", "vst3": "VST3", "lv2": "LV2"}


def fmt_formats(raw: str) -> str:
    if not raw:
        return ""
    parts = [FORMAT_LABELS.get(p.strip(), p.strip().upper()) for p in raw.split(",") if p.strip()]
    return ", ".join(parts)


def yn(value: bool) -> str:
    return "Yes" if value else "No"


def build_markdown(packages: list[dict[str, str]], entries: dict[str, dict[str, str]]) -> tuple[str, int]:
    groups: "OrderedDict[tuple[str, str], OrderedDict[tuple[str, str], list[dict[str, str]]]]" = OrderedDict()
    for pkg in packages:
        cat = (pkg["category_id"], pkg["category_name"])
        sub = (pkg["subcategory_id"], pkg["subcategory_name"])
        groups.setdefault(cat, OrderedDict()).setdefault(sub, []).append(pkg)

    lines: list[str] = []
    lines.append("# Software Catalog\n")
    lines.append(
        "Generated list of every package shipped in Caracal Software Installer's catalog. "
        "Columns describe each app's licensing, install mechanism, and supported plugin targets.\n"
    )
    lines.append("## Column key\n")
    lines.append("- **Open Source** — upstream project ships under a recognized open-source license.")
    lines.append("- **Free Version** — at least one no-cost build is available (may be a demo or feature-limited).")
    lines.append(
        "- **Installs In-App** — Caracal can install it directly. `No` means the catalog opens the developer's website so you can download it yourself."
    )
    lines.append("- **Formats** — plugin targets installed (CLAP / VST2 / VST3 / LV2). Empty for standalone-only apps.")
    lines.append("- **License** — SPDX-style identifier when known.")
    lines.append("- **Version** — pinned upstream version, when the index pins one.\n")

    total = 0
    for (_, cat_name), subs in groups.items():
        lines.append(f"## {cat_name}\n")
        for (_, sub_name), pkgs in subs.items():
            lines.append(f"### {sub_name}\n")
            lines.append("| Name | Vendor | Open Source | Free Version | Installs In-App | Formats | License | Version | Site |")
            lines.append("|------|--------|-------------|--------------|-----------------|---------|---------|---------|------|")
            for pkg in pkgs:
                entry = entries.get(pkg["id"], {})
                open_source = bool_field(entry, "open_source", False)
                has_free = bool_field(entry, "has_free_version", True)
                dl_in_app = bool_field(entry, "dl_within_app", True)
                formats = fmt_formats(entry.get("formats") or "")
                license_label = normalize_license(entry.get("license_type") or "") if open_source else ""
                license_url = (entry.get("link_to_license") or "").strip()
                if license_label and license_url:
                    license_cell = f"[{license_label}]({license_url})"
                else:
                    license_cell = license_label or "—"
                version = (entry.get("version") or "").strip() or "—"
                site = (entry.get("project_website") or "").strip()
                site_cell = f"[link]({site})" if site else "—"
                name = pkg["name"] or entry.get("name") or pkg["id"]
                vendor = pkg["vendor"] or "—"
                lines.append(
                    f"| {name} | {vendor} | {yn(open_source)} | {yn(has_free)} | {yn(dl_in_app)} "
                    f"| {formats or '—'} | {license_cell} | {version} | {site_cell} |"
                )
                total += 1
            lines.append("")

    lines.append(f"---\n_Total: {total} packages._")
    return "\n".join(lines) + "\n", total


def main(argv: list[str]) -> int:
    out_path = Path(argv[1]).resolve() if len(argv) > 1 else DEFAULT_OUT
    entries = load_entries()
    packages = parse_catalog()
    if not packages:
        print("error: no packages parsed from catalog.go", file=sys.stderr)
        return 1

    markdown, total = build_markdown(packages, entries)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(markdown)
    print(f"Wrote {out_path} ({total} packages)")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
