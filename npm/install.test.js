// npm/install.test.js
const assert = require("assert");

// Platform mapping (must match install.js)
const PLATFORM_MAP = { linux: "linux", darwin: "darwin", win32: "windows" };
const ARCH_MAP = { x64: "amd64", arm64: "arm64" };

function getDownloadUrl(version, platform, arch) {
  const os = PLATFORM_MAP[platform];
  const goarch = ARCH_MAP[arch];
  if (!os) throw new Error(`Unsupported platform: ${platform}`);
  if (!goarch) throw new Error(`Unsupported architecture: ${arch}`);
  const ext = os === "windows" ? "zip" : "tar.gz";
  return `https://github.com/smm-h/migrable/releases/download/v${version}/migrable_${version}_${os}_${goarch}.${ext}`;
}

// Test platform mapping
assert.strictEqual(getDownloadUrl("0.2.0", "linux", "x64"),
  "https://github.com/smm-h/migrable/releases/download/v0.2.0/migrable_0.2.0_linux_amd64.tar.gz");
assert.strictEqual(getDownloadUrl("0.2.0", "darwin", "arm64"),
  "https://github.com/smm-h/migrable/releases/download/v0.2.0/migrable_0.2.0_darwin_arm64.tar.gz");
assert.strictEqual(getDownloadUrl("0.2.0", "win32", "x64"),
  "https://github.com/smm-h/migrable/releases/download/v0.2.0/migrable_0.2.0_windows_amd64.zip");

// Test unsupported platform
assert.throws(() => getDownloadUrl("0.2.0", "freebsd", "x64"), /Unsupported platform/);

// Test unsupported arch
assert.throws(() => getDownloadUrl("0.2.0", "linux", "ia32"), /Unsupported architecture/);

// Test Windows gets zip, others get tar.gz
assert.ok(getDownloadUrl("0.2.0", "win32", "x64").endsWith(".zip"));
assert.ok(getDownloadUrl("0.2.0", "linux", "x64").endsWith(".tar.gz"));
assert.ok(getDownloadUrl("0.2.0", "darwin", "arm64").endsWith(".tar.gz"));

console.log("All npm wrapper tests passed");
