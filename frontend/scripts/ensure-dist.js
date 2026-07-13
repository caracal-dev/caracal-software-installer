const fs = require("node:fs");
const path = require("node:path");

const projectRoot = path.resolve(__dirname, "..");
const distDir = path.resolve(projectRoot, "dist");
const required = ["index.html", "main.css", "main.js"];

function statRequiredPath(targetPath, label) {
  const absolutePath = path.resolve(targetPath);
  const relative = path.relative(projectRoot, absolutePath);
  if (relative.startsWith("..") && !path.isAbsolute(relative)) {
    console.error(`Refusing to stat path outside project root: ${targetPath}`);
    process.exit(1);
  }

  // eslint-disable-next-line security/detect-non-literal-fs-filename -- validated above against traversal
  const stat = fs.statSync(absolutePath, { throwIfNoEntry: false });
  if (!stat) {
    console.error(`Missing ${label}: ${targetPath}`);
    process.exit(1);
  }
  return stat;
}

function requiredDistAsset(file) {
  const fullPath = path.resolve(distDir, file);
  const relative = path.relative(distDir, fullPath);
  if (relative.startsWith("..") || path.isAbsolute(relative)) {
    console.error(`Refusing unsafe frontend asset path: ${file}`);
    process.exit(1);
  }
  return fullPath;
}

if (!statRequiredPath(distDir, "frontend dist directory").isDirectory()) {
  console.error(`Missing frontend dist directory: ${distDir}`);
  process.exit(1);
}

for (const file of required) {
  const fullPath = requiredDistAsset(file);
  if (!statRequiredPath(fullPath, "required frontend asset").isFile()) {
    console.error(`Missing required frontend asset: ${fullPath}`);
    process.exit(1);
  }
}

console.log("Frontend dist is ready.");
