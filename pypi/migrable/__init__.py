import io
import os
import platform
import subprocess
import sys
import tarfile
import urllib.request
import zipfile


VERSION = "0.2.1"
_BIN_DIR = os.path.join(os.path.dirname(__file__), "_bin")


def main():
    bin_path = _ensure_binary()
    result = subprocess.run([bin_path] + sys.argv[1:])
    sys.exit(result.returncode)


def _ensure_binary():
    """Download the binary on first run if not present."""
    name = "migrable.exe" if platform.system() == "Windows" else "migrable"
    bin_path = os.path.join(_BIN_DIR, name)
    if os.path.exists(bin_path):
        return bin_path

    os.makedirs(_BIN_DIR, exist_ok=True)

    os_name = _detect_os()
    arch = _detect_arch()
    ext = "zip" if os_name == "windows" else "tar.gz"

    url = (
        f"https://github.com/smm-h/migrable/releases/download/v{VERSION}/"
        f"migrable_{VERSION}_{os_name}_{arch}.{ext}"
    )

    print(f"Downloading migrable v{VERSION} for {os_name}/{arch}...", file=sys.stderr)

    try:
        response = urllib.request.urlopen(url)
        data = response.read()
    except Exception as e:
        print(f"Failed to download migrable: {e}", file=sys.stderr)
        sys.exit(1)

    if ext == "tar.gz":
        with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as tar:
            for member in tar.getmembers():
                if member.name == "migrable" or member.name.endswith("/migrable"):
                    member.name = name
                    tar.extract(member, _BIN_DIR)
                    break
    else:
        with zipfile.ZipFile(io.BytesIO(data)) as zf:
            for zi in zf.infolist():
                if zi.filename == "migrable.exe" or zi.filename.endswith(
                    "/migrable.exe"
                ):
                    zi_data = zf.read(zi)
                    with open(bin_path, "wb") as f:
                        f.write(zi_data)
                    break

    if platform.system() != "Windows":
        os.chmod(bin_path, 0o755)

    return bin_path


def _detect_os():
    s = platform.system().lower()
    if s == "linux":
        return "linux"
    if s == "darwin":
        return "darwin"
    if s == "windows":
        return "windows"
    raise RuntimeError(f"Unsupported OS: {s}")


def _detect_arch():
    m = platform.machine().lower()
    if m in ("x86_64", "amd64"):
        return "amd64"
    if m in ("arm64", "aarch64"):
        return "arm64"
    raise RuntimeError(f"Unsupported architecture: {m}")
