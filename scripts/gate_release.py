"""Keep the Gate dependency, build matrix, release tags, and assets in sync."""

import base64
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

UPSTREAM = "minekube/gate"
STABLE_TAG = re.compile(r"v(\d+)\.(\d+)\.(\d+)")


def run(*args):
    return subprocess.check_output(args, text=True).strip()


def stable_version(tag):
    match = STABLE_TAG.fullmatch(tag)
    if match is None:
        raise ValueError(f"Expected a stable Gate tag vX.Y.Z, got {tag!r}")
    return tuple(int(part) for part in match.groups())


def gate_version():
    return run("go", "list", "-m", "-f", "{{.Version}}", "go.minekube.com/gate")


def check_tag(tag):
    stable_version(tag)
    dependency = gate_version()
    if tag != dependency:
        raise ValueError(f"Release tag {tag} does not match Gate dependency {dependency}")


def output(**values):
    for name, value in values.items():
        print(f"{name}={value}")
    if path := os.environ.get("GITHUB_OUTPUT"):
        with open(path, "a") as file:
            for name, value in values.items():
                file.write(f"{name}={value}\n")


def update():
    release = json.loads(run("gh", "api", f"repos/{UPSTREAM}/releases/latest"))
    tag = release["tag_name"]
    if release.get("draft") or release.get("prerelease"):
        raise ValueError("Upstream latest release must be stable and published")
    if stable_version(tag) < stable_version(gate_version()):
        raise ValueError("Refusing to downgrade the Gate dependency")
    tags = run("git", "tag", "--list", tag)
    if tags:
        if gate_version() != tag:
            raise ValueError(f"Tag {tag} already exists but go.mod uses another Gate version")
        output(changed="false", tag=tag)
        return

    # Match the build matrix of this stable release, not upstream master.
    # GoReleaser builds this repository's main package, which registers plugins.
    source = json.loads(run("gh", "api", f"repos/{UPSTREAM}/contents/.goreleaser.yml?ref={tag}"))
    config = base64.b64decode(source["content"], validate=False).decode()
    config = re.sub(r"(?m)^project_name:.*\n", "", config)
    Path(".goreleaser.yml").write_text("project_name: gate\n" + config)
    subprocess.run(["go", "get", f"go.minekube.com/gate@{tag}"], check=True)
    subprocess.run(["go", "mod", "tidy"], check=True)
    output(changed="true", tag=tag)


def binary_names(release, tag):
    stable_version(tag)
    pattern = re.compile(
        rf"gate_{re.escape(tag[1:])}_(linux|darwin|windows)_"
        r"(amd64|arm64|386|armv[5-7])(_musl)?(\.exe)?"
    )
    names = {asset["name"] for asset in release["assets"] if pattern.fullmatch(asset["name"])}
    if not names:
        raise ValueError(f"Upstream {tag} has no recognized release binaries")
    return names


def verify_files(directory, expected):
    actual = {path.name for path in directory.glob("gate_*") if path.is_file()}
    if actual != expected:
        raise ValueError(f"Release platform mismatch: missing={expected - actual}, extra={actual - expected}")
    manifest = {}
    for line in (directory / "checksums.txt").read_text().splitlines():
        digest, name = line.split(maxsplit=1)
        name = name.lstrip("*")
        if name in manifest or name not in expected:
            raise ValueError(f"Unexpected or duplicate checksum entry: {name}")
        manifest[name] = digest
    if set(manifest) != expected:
        raise ValueError("Checksum manifest does not cover every release binary")
    for name, digest in manifest.items():
        path = directory / name
        if path.stat().st_size == 0 or hashlib.sha256(path.read_bytes()).hexdigest() != digest:
            raise ValueError(f"Invalid release binary or checksum: {name}")


def collect_binaries(dist, destination, expected):
    # Binary archives keep their original build path. GoReleaser's metadata
    # supplies the public filename; it does not create dist/<public filename>.
    artifacts = json.loads((dist / "artifacts.json").read_text())
    exported = {
        artifact["name"]: Path(artifact["path"])
        for artifact in artifacts
        if artifact.get("type") == "Binary" and artifact["name"].startswith("gate_")
    }
    if set(exported) != expected:
        raise ValueError(f"Release platform mismatch: missing={expected - set(exported)}, extra={set(exported) - expected}")
    destination.mkdir(exist_ok=False)
    for name, source in exported.items():
        shutil.copy2(source, destination / name)
    shutil.copy2(dist / "checksums.txt", destination / "checksums.txt")
    verify_files(destination, expected)


def prepare(tag):
    check_tag(tag)
    upstream = json.loads(run("gh", "api", f"repos/{UPSTREAM}/releases/tags/{tag}"))
    expected = binary_names(upstream, tag)
    destination = Path("release")
    collect_binaries(Path("dist"), destination, expected)
    (destination / "release-notes.md").write_text(
        f"[Gate {tag}](https://github.com/{UPSTREAM}/releases/tag/{tag}) "
        "with SimpleCloud's Connection plugin built in.\n\n"
        "Binary names, platforms, and version match the upstream Gate release. "
        "Verify downloads using `checksums.txt`.\n\n"
        "Copy your version 2 connection configs into `simplecloud-connection/`. "
        "See the [configuration guide](https://github.com/simplecloudapp/"
        f"simplecloud-gate/blob/{tag}/docs/configuration.md) for setup and compatibility.\n"
    )


if __name__ == "__main__":
    try:
        if sys.argv[1:] == ["update"]:
            update()
        elif len(sys.argv) == 3 and sys.argv[1] == "check-tag":
            check_tag(sys.argv[2])
        elif len(sys.argv) == 3 and sys.argv[1] == "prepare":
            prepare(sys.argv[2])
        else:
            raise ValueError("Usage: gate_release.py update | check-tag TAG | prepare TAG")
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        sys.exit(str(error))
