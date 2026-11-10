// Copies the browser files the web UI uses into src/assets/vendor, which is embedded into the binary.
import { cpSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";

const root = join(dirname(new URL(import.meta.url).pathname), "..");
const outDir = join(root, "src/assets/vendor");

// flattened, since .gitignore excludes dist/; the icon stylesheet references fonts/ relatively
const files = {
  bootstrap: [
    ["dist/css/bootstrap.min.css", "bootstrap.min.css"],
    ["dist/js/bootstrap.bundle.min.js", "bootstrap.bundle.min.js"],
    ["LICENSE", "LICENSE"],
  ],
  "bootstrap-icons": [
    ["font/bootstrap-icons.min.css", "bootstrap-icons.min.css"],
    ["font/fonts/bootstrap-icons.woff2", "fonts/bootstrap-icons.woff2"],
    ["font/fonts/bootstrap-icons.woff", "fonts/bootstrap-icons.woff"],
    ["LICENSE", "LICENSE"],
  ],
  // the license is in the bundle's header; the package ships no separate file
  "masonry-layout": [["dist/masonry.pkgd.min.js", "masonry.pkgd.min.js"]],
  uppy: [
    ["dist/uppy.min.js", "uppy.min.js"],
    ["LICENSE", "LICENSE"],
  ],
};

rmSync(outDir, { recursive: true, force: true });
const versions = {};
for (const [pkg, pairs] of Object.entries(files)) {
  const pkgDir = join(root, "node_modules", pkg);
  versions[pkg] = JSON.parse(readFileSync(join(pkgDir, "package.json"), "utf8")).version;
  for (const [src, dest] of pairs) {
    const target = join(outDir, pkg, dest);
    mkdirSync(dirname(target), { recursive: true });
    cpSync(join(pkgDir, src), target);
  }
}
writeFileSync(join(outDir, "versions.json"), JSON.stringify(versions, null, 2) + "\n");
console.log("vendored:", Object.entries(versions).map(([p, v]) => `${p}@${v}`).join(", "));
