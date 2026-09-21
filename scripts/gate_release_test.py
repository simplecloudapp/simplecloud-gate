import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import gate_release


class GateReleaseTest(unittest.TestCase):
    def test_only_stable_tags(self):
        self.assertEqual(gate_release.stable_version("v0.74.9"), (0, 74, 9))
        for tag in ["main", "v0.74.9-rc.1", "v0.74.9;echo bad", "0.74.9"]:
            with self.assertRaises(ValueError):
                gate_release.stable_version(tag)

    def test_tag_must_match_dependency(self):
        with patch.object(gate_release, "gate_version", return_value="v0.74.9"):
            gate_release.check_tag("v0.74.9")
            with self.assertRaises(ValueError):
                gate_release.check_tag("v0.75.0")

    def test_platform_names_include_windows_and_musl(self):
        names = ["gate_0.74.9_linux_386_musl", "gate_0.74.9_windows_arm64.exe", "gate_0.74.9_darwin_amd64"]
        release = {"assets": [{"name": name} for name in names + ["checksums.txt", "other.zip"]]}
        self.assertEqual(gate_release.binary_names(release, "v0.74.9"), set(names))

    def test_missing_and_corrupted_binaries_are_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            name = "gate_0.74.9_linux_amd64"
            (directory / name).write_bytes(b"binary")
            digest = hashlib.sha256(b"binary").hexdigest()
            (directory / "checksums.txt").write_text(f"{digest}  {name}\n")
            gate_release.verify_files(directory, {name})
            with self.assertRaises(ValueError):
                gate_release.verify_files(directory, {name, "gate_0.74.9_linux_arm64"})
            (directory / name).write_bytes(b"corrupt")
            with self.assertRaises(ValueError):
                gate_release.verify_files(directory, {name})

    def test_update_does_not_downgrade(self):
        with patch.object(gate_release, "gate_version", return_value="v0.75.0"), patch.object(
            gate_release, "run", return_value='{"tag_name":"v0.74.9"}'
        ):
            with self.assertRaisesRegex(ValueError, "downgrade"):
                gate_release.update()

    def test_binary_archives_use_metadata_paths(self):
        with tempfile.TemporaryDirectory() as temporary:
            dist = Path(temporary)
            source = dist / "gate_linux_amd64_v1" / "gate"
            source.parent.mkdir()
            source.write_bytes(b"binary")
            name = "gate_0.74.9_linux_amd64"
            (dist / "artifacts.json").write_text(json.dumps([
                {"type": "Binary", "name": "gate", "path": str(source)},
                {"type": "Binary", "name": name, "path": str(source)},
            ]))
            digest = hashlib.sha256(b"binary").hexdigest()
            (dist / "checksums.txt").write_text(f"{digest}  {name}\n")
            destination = dist / "release"
            gate_release.collect_binaries(dist, destination, {name})
            self.assertEqual((destination / name).read_bytes(), b"binary")


if __name__ == "__main__":
    unittest.main()
