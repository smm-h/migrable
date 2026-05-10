const https = require("https");
const fs = require("fs");
const path = require("path");
const { execSync } = require("child_process");

const pkg = require("./package.json");
const version = pkg.version;

const PLATFORM_MAP = {
  linux: "linux",
  darwin: "darwin",
  win32: "windows",
};

const ARCH_MAP = {
  x64: "amd64",
  arm64: "arm64",
};

function main() {
  const osPlatform = process.platform;
  const osArch = process.arch;

  const os = PLATFORM_MAP[osPlatform];
  if (!os) {
    console.error(`Unsupported platform: ${osPlatform}`);
    process.exit(1);
  }

  const arch = ARCH_MAP[osArch];
  if (!arch) {
    console.error(`Unsupported architecture: ${osArch}`);
    process.exit(1);
  }

  const ext = os === "windows" ? "zip" : "tar.gz";
  const url = `https://github.com/smm-h/migrable/releases/download/v${version}/migrable_${version}_${os}_${arch}.${ext}`;

  const binName = os === "windows" ? "migrable.exe" : "migrable";
  const destPath = path.join(__dirname, binName);

  console.log(`Downloading migrable v${version} for ${os}/${arch}...`);

  download(url, (err, data) => {
    if (err) {
      console.error(`Failed to download migrable: ${err.message}`);
      process.exit(1);
    }

    if (ext === "tar.gz") {
      extractTarGz(data, binName, destPath);
    } else {
      extractZip(data, binName, destPath);
    }

    if (os !== "windows") {
      fs.chmodSync(destPath, 0o755);
    }

    console.log(`migrable v${version} installed successfully.`);
  });
}

function download(url, callback, redirects) {
  if (redirects === undefined) redirects = 0;
  if (redirects > 5) {
    callback(new Error("Too many redirects"));
    return;
  }

  const mod = url.startsWith("https") ? https : require("http");

  mod.get(url, (res) => {
    if (res.statusCode === 301 || res.statusCode === 302) {
      download(res.headers.location, callback, redirects + 1);
      return;
    }

    if (res.statusCode !== 200) {
      callback(new Error(`HTTP ${res.statusCode}: ${url}`));
      return;
    }

    const chunks = [];
    res.on("data", (chunk) => chunks.push(chunk));
    res.on("end", () => callback(null, Buffer.concat(chunks)));
    res.on("error", callback);
  }).on("error", callback);
}

function extractTarGz(data, binName, destPath) {
  // Write archive to a temp file, extract with tar
  const tmpArchive = path.join(__dirname, "_tmp_archive.tar.gz");
  const tmpDir = path.join(__dirname, "_tmp_extract");

  try {
    fs.writeFileSync(tmpArchive, data);
    fs.mkdirSync(tmpDir, { recursive: true });

    execSync(`tar xzf "${tmpArchive}" -C "${tmpDir}"`, { stdio: "pipe" });

    // Find the binary in the extracted files
    const extracted = findFile(tmpDir, binName);
    if (!extracted) {
      throw new Error(`Binary "${binName}" not found in archive`);
    }

    fs.copyFileSync(extracted, destPath);
  } finally {
    try { fs.unlinkSync(tmpArchive); } catch (_) {}
    try { fs.rmSync(tmpDir, { recursive: true }); } catch (_) {}
  }
}

function extractZip(data, binName, destPath) {
  // Write archive to a temp file, extract with PowerShell on Windows
  const tmpArchive = path.join(__dirname, "_tmp_archive.zip");
  const tmpDir = path.join(__dirname, "_tmp_extract");

  try {
    fs.writeFileSync(tmpArchive, data);
    fs.mkdirSync(tmpDir, { recursive: true });

    if (process.platform === "win32") {
      execSync(
        `powershell -NoProfile -Command "Expand-Archive -Path '${tmpArchive}' -DestinationPath '${tmpDir}' -Force"`,
        { stdio: "pipe" }
      );
    } else {
      execSync(`unzip -o "${tmpArchive}" -d "${tmpDir}"`, { stdio: "pipe" });
    }

    const extracted = findFile(tmpDir, binName);
    if (!extracted) {
      throw new Error(`Binary "${binName}" not found in archive`);
    }

    fs.copyFileSync(extracted, destPath);
  } finally {
    try { fs.unlinkSync(tmpArchive); } catch (_) {}
    try { fs.rmSync(tmpDir, { recursive: true }); } catch (_) {}
  }
}

function findFile(dir, name) {
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      const found = findFile(fullPath, name);
      if (found) return found;
    } else if (entry.name === name) {
      return fullPath;
    }
  }
  return null;
}

main();
