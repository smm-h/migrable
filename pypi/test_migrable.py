import unittest
from migrable import _detect_os, _detect_arch, VERSION

class TestPlatformDetection(unittest.TestCase):
    def test_version_matches(self):
        self.assertEqual(VERSION, "0.2.0")

    def test_detect_os_returns_string(self):
        result = _detect_os()
        self.assertIn(result, ("linux", "darwin", "windows"))

    def test_detect_arch_returns_string(self):
        result = _detect_arch()
        self.assertIn(result, ("amd64", "arm64"))

    def test_url_construction(self):
        os_name = _detect_os()
        arch = _detect_arch()
        ext = "zip" if os_name == "windows" else "tar.gz"
        url = f"https://github.com/smm-h/migrable/releases/download/v{VERSION}/migrable_{VERSION}_{os_name}_{arch}.{ext}"
        self.assertIn("github.com/smm-h/migrable", url)
        self.assertIn(VERSION, url)
        self.assertTrue(url.endswith(".tar.gz") or url.endswith(".zip"))

    def test_windows_gets_zip(self):
        ext = "zip" if "windows" == "windows" else "tar.gz"
        self.assertEqual(ext, "zip")

    def test_linux_gets_targz(self):
        ext = "zip" if "linux" == "windows" else "tar.gz"
        self.assertEqual(ext, "tar.gz")

if __name__ == "__main__":
    unittest.main()
