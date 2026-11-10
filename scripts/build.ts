// Builds the web UI files embedded into the binary: bundled page scripts into src/assets/js,
// stylesheets and fonts into src/assets/vendor.
import { cpSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";

const root = join(import.meta.dir, "..");
const assets = join(root, "src/assets");

// flattened, since .gitignore excludes dist/; the icon stylesheet references fonts/ relatively
const vendored: Record<string, [string, string][]> = {
  bootstrap: [
    ["dist/css/bootstrap.min.css", "bootstrap.min.css"],
    ["LICENSE", "LICENSE"],
  ],
  "bootstrap-icons": [
    ["font/bootstrap-icons.min.css", "bootstrap-icons.min.css"],
    ["font/fonts/bootstrap-icons.woff2", "fonts/bootstrap-icons.woff2"],
    ["font/fonts/bootstrap-icons.woff", "fonts/bootstrap-icons.woff"],
    ["LICENSE", "LICENSE"],
  ],
};

const vendorDir = join(assets, "vendor");
rmSync(vendorDir, { recursive: true, force: true });
const versions: Record<string, string> = {};
for (const [pkg, files] of Object.entries(vendored)) {
  const pkgDir = join(root, "node_modules", pkg);
  versions[pkg] = JSON.parse(readFileSync(join(pkgDir, "package.json"), "utf8")).version;
  for (const [src, dest] of files) {
    const target = join(vendorDir, pkg, dest);
    mkdirSync(dirname(target), { recursive: true });
    cpSync(join(pkgDir, src), target);
  }
}
writeFileSync(join(vendorDir, "versions.json"), JSON.stringify(versions, null, 2) + "\n");

const jsDir = join(assets, "js");
rmSync(jsDir, { recursive: true, force: true });
const common = { outdir: jsDir, target: "browser", minify: true, naming: "[name].js", metafile: true } as const;
const builds = [
  Bun.build({
    ...common,
    entrypoints: ["index", "rename", "twofa", "install"].map((name) => join(root, "src/web", `${name}.ts`)),
    format: "esm",
  }),
  // runs in <head> before the page renders, so it can't be a module
  Bun.build({ ...common, entrypoints: [join(root, "src/web/theme.ts")], format: "iife" }),
];
const bundled = new Set<string>();
for (const result of await Promise.all(builds)) {
  if (!result.success) {
    for (const log of result.logs) console.error(log);
    process.exit(1);
  }
  for (const input of Object.keys(result.metafile?.inputs ?? {})) {
    const pkg = input.match(/node_modules\/((?:@[^/]+\/)?[^/]+)/)?.[1];
    if (pkg) bundled.add(pkg);
  }
}

// minifying drops most license headers
const notices = [...bundled].sort().map((pkg) => {
  const pkgDir = join(root, "node_modules", pkg);
  const { version, license } = JSON.parse(readFileSync(join(pkgDir, "package.json"), "utf8"));
  const licenseFile = readdirSync(pkgDir).find((f) => /^licen[cs]e/i.test(f));
  const text = licenseFile ? readFileSync(join(pkgDir, licenseFile), "utf8").trim() : `License: ${license}`;
  return `${pkg}@${version}\n${"-".repeat(pkg.length + version.length + 1)}\n${text}\n`;
});
writeFileSync(join(jsDir, "THIRD_PARTY_LICENSES.txt"), notices.join("\n"));

console.log(
  `built: js/ (bundling ${bundled.size} packages) and vendor/`,
  Object.entries(versions)
    .map(([p, v]) => `${p}@${v}`)
    .join(", "),
);
