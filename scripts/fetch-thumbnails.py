#!/usr/bin/env python3
"""Fetch screenshots from GitHub repos and convert to 800x440 webp thumbnails.
Uses `gh api` for authenticated access (higher rate limits)."""
import csv, io, re, json, base64, urllib.request, urllib.error, os, sys, time, subprocess
from pathlib import Path
from PIL import Image

THUMB_DIR = Path("frontend/dist/assets/images/thumbnails")
CSV_PATH = Path("data/download-index.csv")
SIZE = (800, 440)
BG_COLOR = (26, 25, 46)

def gh_api(path):
    """Call GitHub API via authenticated gh CLI."""
    try:
        result = subprocess.run(
            ["gh", "api", path, "--jq", "."],
            capture_output=True, text=True, timeout=30
        )
        if result.returncode != 0:
            return None
        return json.loads(result.stdout)
    except Exception:
        return None

def gh_api_raw(path):
    """Call GitHub API and return raw text output."""
    try:
        result = subprocess.run(
            ["gh", "api", path],
            capture_output=True, text=True, timeout=30
        )
        if result.returncode != 0:
            return None
        return result.stdout
    except Exception:
        return None

def find_images_in_markdown(content):
    """Extract image URLs from markdown/HTML content."""
    urls = []
    for m in re.finditer(r'!\[.*?\]\((https?://[^\s)]+)\)', content):
        urls.append(m.group(1))
    for m in re.finditer(r'<img[^>]+src="(https?://[^"]+)"', content):
        urls.append(m.group(1))
    for m in re.finditer(r'<img[^>]+src=\'(https?://[^\']+)\'', content):
        urls.append(m.group(1))
    
    img_exts = ('.png', '.jpg', '.jpeg', '.gif', '.webp')
    clean = []
    for u in urls:
        u2 = u.split('?')[0].split('#')[0]
        if not any(u2.lower().endswith(e) for e in img_exts):
            continue
        low = u.lower()
        if any(x in low for x in ['badge', 'paypal', 'donate', 'travis', 'coveralls',
                                   'codecov', 'avatars', 'shields.io', 'githubusercontent.com/.*/actions',
                                   'github-readme-stats', 'img.shields']):
            continue
        clean.append(u)
    return clean

def repo_images_via_tree(owner, repo, branch):
    """Get all image files in repo via git tree API."""
    tree = gh_api(f"repos/{owner}/{repo}/git/trees/{branch}?recursive=1")
    if not tree or 'tree' not in tree:
        return []
    
    img_exts = ('.png', '.jpg', '.jpeg', '.webp')
    candidates = []
    for item in tree['tree']:
        path = item.get('path', '')
        low = path.lower()
        if not any(low.endswith(e) for e in img_exts):
            continue
        # Skip obvious non-screenshots
        if any(x in low for x in ['/icon', '/logo', 'favicon', 'badge', 'button',
                                  'background', 'sprite', 'avatar', 'avatar/',
                                  '.github/', 'doc/logo', 'docs/img/logo']):
            continue
        # Prefer files that look like screenshots
        score = 0
        if 'screenshot' in low: score += 3
        if 'screen' in low: score += 2
        if 'ui' in low or 'gui' in low: score += 1
        if repo.lower() in low: score += 1
        candidates.append((score, path, item['size']))
    
    candidates.sort(key=lambda x: -x[0])
    return candidates

def find_repo_screenshot(owner, repo):
    """Find a screenshot image URL for a repo."""
    # 1. Try README images (most reliable - project's own docs)
    readme = gh_api_raw(f"repos/{owner}/{repo}/readme")
    if readme:
        try:
            data = json.loads(readme)
            content = base64.b64decode(data.get('content', '')).decode('utf-8', errors='replace')
            urls = find_images_in_markdown(content)
            if urls:
                # Convert github blob urls to raw
                converted = []
                for u in urls:
                    m = re.match(r'https://github\.com/[^/]+/[^/]+/blob/([^/]+)/(.*?)$', u)
                    if m:
                        u = f"https://raw.githubusercontent.com/{owner}/{repo}/{m.group(1)}/{m.group(2)}"
                    converted.append(u)
                return converted, "readme"
        except Exception:
            pass
    
    # 2. Search repo tree for screenshots
    branches = []
    repo_info = gh_api(f"repos/{owner}/{repo}")
    if repo_info and repo_info.get('default_branch'):
        branches.append(repo_info['default_branch'])
    branches.append('main')
    branches.append('master')
    
    seen = set()
    for branch in dict.fromkeys(branches):
        if branch in seen:
            continue
        seen.add(branch)
        try:
            candidates = repo_images_via_tree(owner, repo, branch)
            if candidates:
                urls = []
                for score, path, size in candidates[:5]:
                    urls.append(f"https://raw.githubusercontent.com/{owner}/{repo}/{branch}/{path}")
                return urls, "tree"
        except Exception:
            continue
    
    return [], None

def download_image(url, save_path):
    """Download an image to a path."""
    req = urllib.request.Request(url, headers={
        "User-Agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
        "Accept": "image/webp,image/apng,image/*,*/*;q=0.8"
    })
    try:
        resp = urllib.request.urlopen(req, timeout=30)
        data = resp.read()
        if len(data) < 500:
            return None
        save_path.write_bytes(data)
        return save_path
    except Exception:
        return None

def process_image(src_path, dst_path):
    """Convert image to 800x440 webp with dark background."""
    try:
        img = Image.open(src_path)
        img.load()
        
        # Create dark background canvas
        canvas = Image.new("RGB", SIZE, BG_COLOR)
        
        # Resize to fit within 760x400 maintaining aspect ratio
        img.thumbnail((760, 400), Image.LANCZOS)
        
        x = (SIZE[0] - img.width) // 2
        y = (SIZE[1] - img.height) // 2
        
        if img.mode in ('RGBA', 'LA') or (img.mode == 'P' and 'transparency' in img.info):
            img = img.convert('RGBA')
            canvas.paste(img, (x, y), img)
        else:
            img = img.convert('RGB')
            canvas.paste(img, (x, y))
        
        canvas.save(dst_path, "WEBP", quality=85, method=6)
        return True
    except Exception:
        return False

def process_entry(eid, name, github_repo, url):
    """Process a single entry."""
    thumb_path = THUMB_DIR / f"{eid}.webp"
    if thumb_path.exists():
        return (eid, "EXISTS", "", "")
    
    print(f"\n=== {eid} ({name}) ===")
    
    if github_repo:
        owner, repo = github_repo.split('/', 1)
        print(f"  GitHub: {owner}/{repo}")
        
        images, source = find_repo_screenshot(owner, repo)
        if images:
            print(f"  Found {len(images)} image(s) via {source}")
            for i, img_url in enumerate(images):
                tmp = THUMB_DIR / f".tmp_{eid}_{i}.download"
                try:
                    result = download_image(img_url, tmp)
                    if result and tmp.stat().st_size > 1000:
                        img = Image.open(tmp)
                        w, h = img.size
                        if w < 50 or h < 50:
                            continue
                        print(f"  Downloaded: {w}x{h} from {img_url[:90]}")
                        
                        if process_image(tmp, thumb_path):
                            tmp.unlink(missing_ok=True)
                            return (eid, "DONE", f"{w}x{h}", img_url[:70])
                        tmp.unlink(missing_ok=True)
                except Exception as e:
                    print(f"  Failed: {e}")
                    if tmp.exists():
                        tmp.unlink()
        
        print(f"  NO IMAGE FOUND")
        return (eid, "NONE", "", "")
    else:
        # Non-GitHub: try website og:image
        print(f"  Non-GitHub: {url[:70]}")
        try:
            req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
            resp = urllib.request.urlopen(req, timeout=15)
            html = resp.read().decode('utf-8', errors='replace')
            m = re.search(r'<meta\s+property="og:image"\s+content="([^"]+)"', html, re.I)
            if not m:
                m = re.search(r'<meta\s+name="twitter:image"\s+content="([^"]+)"', html, re.I)
            if m:
                img_url = m.group(1)
                if img_url.startswith('/'):
                    from urllib.parse import urljoin
                    img_url = urljoin(url, img_url)
                tmp = THUMB_DIR / f".tmp_{eid}.download"
                result = download_image(img_url, tmp)
                if result and tmp.stat().st_size > 1000:
                    img = Image.open(tmp)
                    if process_image(tmp, thumb_path):
                        tmp.unlink(missing_ok=True)
                        return (eid, "DONE", f"{img.size[0]}x{img.size[1]}", img_url[:70])
                    tmp.unlink(missing_ok=True)
        except Exception as e:
            print(f"  Failed: {e}")
        return (eid, "NONE", "", "")

def main():
    THUMB_DIR.mkdir(parents=True, exist_ok=True)
    
    # Only process entries in argv if given, else all
    only = sys.argv[1:] if len(sys.argv) > 1 else None
    
    raw = open(CSV_PATH).read()
    reader = csv.DictReader(io.StringIO(raw))
    
    existing = set()
    for f in THUMB_DIR.glob("*.webp"):
        existing.add(f.stem)
    
    entries_to_process = []
    for row in reader:
        eid = row['id']
        if eid in existing:
            continue
        if only and eid not in only:
            continue
        
        url = row['url']
        website = row.get('project_website', '') or ''
        name = row.get('name', '') or ''
        
        github_repo = None
        for u in [url, website]:
            m = re.match(r'https://github\.com/([^/]+/[^/]+)', u)
            if m:
                github_repo = m.group(1)
                break
            m2 = re.search(r'github\.com/([^/]+/[^/]+)/releases/', u)
            if m2:
                github_repo = m2.group(1)
                break
        
        entries_to_process.append((eid, name, github_repo, url))
    
    print(f"Entries to process: {len(entries_to_process)}")
    
    results = []
    for eid, name, gh, url in entries_to_process:
        r = process_entry(eid, name, gh, url)
        if r:
            results.append(r)
    
    print(f"\n\n=== RESULTS ===")
    print(f"{'ID':<25} {'Status':<7} {'Size':<10} {'Source'}")
    print("-" * 80)
    for eid, status, size, source in results:
        print(f"{eid:<25} {status:<7} {size:<10} {source}")

if __name__ == "__main__":
    main()