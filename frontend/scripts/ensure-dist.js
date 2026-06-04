const fs = require("node:fs");
const path = require("node:path");

const distDir = path.resolve(__dirname, "..", "dist");
const required = ["index.html", "main.css", "main.js"];

function statRequiredPath(targetPath, label) {
  const stat = fs.statSync(targetPath, { throwIfNoEntry: false });
  if (!stat) {
    console.error(`Missing ${label}: ${targetPath}`);
    process.exit(1);
  }
  return stat;
}

function requiredDistAsset(file) {
  const fullPath = path.resolve(distDir, file);
  if (path.dirname(fullPath) !== distDir) {
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
