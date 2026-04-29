#!/usr/bin/env python3
"""Helpers for desktop release automation on GitHub Actions."""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from pathlib import Path
from typing import Any
from urllib import error as urllib_error
from urllib import request as urllib_request


REPO_ROOT = Path(__file__).resolve().parents[2]
SEMVER_PATTERN = re.compile(
    r"^(?P<major>0|[1-9]\d*)\."
    r"(?P<minor>0|[1-9]\d*)\."
    r"(?P<patch>0|[1-9]\d*)"
    r"(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)"
    r"(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?"
    r"(?:\+(?P<build>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$"
)


def write_workflow_output(name: str, value: str) -> None:
    """Append a single GitHub Actions output entry."""

    output_path = os.environ.get("GITHUB_OUTPUT", "").strip()
    if not output_path:
        raise RuntimeError("GITHUB_OUTPUT is not available.")

    with Path(output_path).open("a", encoding="utf-8", newline="\n") as handle:
        handle.write(f"{name}={value}\n")


def github_api_headers() -> dict[str, str]:
    """Build authenticated GitHub REST headers for mutating release state."""

    token = os.environ.get("GITHUB_TOKEN", "").strip()
    if not token:
        raise RuntimeError("GITHUB_TOKEN is required.")

    return {
        "Authorization": f"Bearer {token}",
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
    }


def github_api_request(method: str, url: str) -> Any | None:
    """Issue a GitHub REST request and return decoded JSON when present."""

    request = urllib_request.Request(url, method=method, headers=github_api_headers())
    try:
        with urllib_request.urlopen(request) as response:
            payload = response.read()
    except urllib_error.HTTPError as exc:
        if exc.code == 404:
            return None
        body = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"GitHub API {method} {url} failed with {exc.code}: {body}") from exc

    if not payload:
        return None

    return json.loads(payload.decode("utf-8"))


def assert_windows_installer_version_bounds(version: str, major: int, minor: int, patch: int) -> None:
    """Guard against MSI version fields that exceed Windows installer limits."""

    if major > 255 or minor > 255 or patch > 65535:
        raise RuntimeError(
            f"Version {version} exceeds MSI limits (major<=255, minor<=255, patch<=65535)."
        )


def detect_newline(text: str) -> str:
    """Preserve the dominant newline style of an existing manifest file."""

    return "\r\n" if "\r\n" in text else "\n"


def write_json_file(path: Path, payload: dict[str, Any]) -> None:
    """Rewrite a JSON file while preserving indentation and newline style."""

    existing = path.read_text(encoding="utf-8")
    newline = detect_newline(existing)
    rendered = json.dumps(payload, indent=2, ensure_ascii=False) + newline
    with path.open("w", encoding="utf-8", newline="") as handle:
        handle.write(rendered)


def get_package_version_from_cargo_toml(cargo_path: Path) -> str | None:
    """Read the package version from the Cargo.toml [package] section."""

    inside_package = False
    for line in cargo_path.read_text(encoding="utf-8").splitlines():
        if re.match(r"^\s*\[package\]\s*$", line):
            inside_package = True
            continue

        if inside_package and re.match(r"^\s*\[.*\]\s*$", line):
            break

        match = re.match(r'^\s*version\s*=\s*"([^"]+)"\s*$', line)
        if inside_package and match:
            return match.group(1)

    return None


def update_cargo_package_version(cargo_path: Path, version: str) -> None:
    """Update only the [package] version line inside Cargo.toml."""

    content = cargo_path.read_text(encoding="utf-8")
    newline = detect_newline(content)
    had_terminal_newline = content.endswith(("\n", "\r"))
    lines = re.split(r"\r?\n", content)
    inside_package = False
    package_version_found = False

    for index, line in enumerate(lines):
        if re.match(r"^\s*\[package\]\s*$", line):
            inside_package = True
            continue

        if inside_package and re.match(r"^\s*\[.*\]\s*$", line):
            inside_package = False

        if inside_package and re.match(r'^\s*version\s*=\s*".*"\s*$', line):
            lines[index] = re.sub(
                r'^(\s*version\s*=\s*")[^"]*("\s*)$',
                lambda match: f"{match.group(1)}{version}{match.group(2)}",
                line,
                count=1,
            )
            package_version_found = True

    if not package_version_found:
        raise RuntimeError("Failed to update version in Cargo.toml: [package] version line not found")

    rendered = newline.join(lines)
    if had_terminal_newline:
        rendered += newline

    with cargo_path.open("w", encoding="utf-8", newline="") as handle:
        handle.write(rendered)


def run_command(command: list[str], cwd: Path = REPO_ROOT) -> subprocess.CompletedProcess[str]:
    """Run a subprocess and surface stdout/stderr together on failure."""

    return subprocess.run(
        command,
        cwd=cwd,
        check=True,
        capture_output=True,
        text=True,
        encoding="utf-8",
    )


def resolve_metadata() -> None:
    """Emit release-mode-specific metadata for the workflow."""

    github_ref_type = os.environ.get("GITHUB_REF_TYPE", "").strip()
    github_ref_name = os.environ.get("GITHUB_REF_NAME", "").strip()
    github_ref = os.environ.get("GITHUB_REF", "").strip()

    if github_ref_type == "tag":
        version = github_ref_name[1:]
        match = SEMVER_PATTERN.fullmatch(version)
        if not match:
            raise RuntimeError(f"Invalid SemVer tag: {github_ref_name}")

        assert_windows_installer_version_bounds(
            version,
            int(match.group("major")),
            int(match.group("minor")),
            int(match.group("patch")),
        )

        is_prerelease = bool(match.group("prerelease"))
        bundles = "nsis" if is_prerelease else "nsis,msi"
        asset_name_pattern = "[name]_[version]_[arch][setup][ext]"

        write_workflow_output("release_mode", "tag")
        write_workflow_output("manifest_version", version)
        write_workflow_output("release_is_prerelease", str(is_prerelease).lower())
        write_workflow_output("release_bundles", bundles)
        write_workflow_output("release_asset_name_pattern", asset_name_pattern)
        return

    if github_ref == "refs/heads/main":
        write_workflow_output("release_mode", "tip")
        write_workflow_output("release_is_prerelease", "true")
        write_workflow_output("release_bundles", "nsis")
        write_workflow_output("release_asset_name_pattern", "[name]_tip_[arch][setup][ext]")
        return

    raise RuntimeError("Unsupported ref")


def rewrite_version_manifests() -> None:
    """Apply a CI-only manifest version rewrite for semver tag builds."""

    version = os.environ.get("MANIFEST_VERSION", "").strip()
    if not version:
        raise RuntimeError("MANIFEST_VERSION is required.")

    package_json_path = REPO_ROOT / "apps/desktop/package.json"
    package_json = json.loads(package_json_path.read_text(encoding="utf-8"))
    package_json["version"] = version
    write_json_file(package_json_path, package_json)

    tauri_config_path = REPO_ROOT / "apps/desktop/src-tauri/tauri.conf.json"
    tauri_config = json.loads(tauri_config_path.read_text(encoding="utf-8"))
    tauri_config["version"] = version
    write_json_file(tauri_config_path, tauri_config)

    cargo_path = REPO_ROOT / "apps/desktop/src-tauri/Cargo.toml"
    update_cargo_package_version(cargo_path, version)


def verify_version_manifests() -> None:
    """Confirm that the CI-only manifest rewrite landed in every required file."""

    version = os.environ.get("MANIFEST_VERSION", "").strip()
    if not version:
        raise RuntimeError("MANIFEST_VERSION is required.")

    package_json_path = REPO_ROOT / "apps/desktop/package.json"
    if json.loads(package_json_path.read_text(encoding="utf-8"))["version"] != version:
        raise RuntimeError("package.json mismatch")

    tauri_config_path = REPO_ROOT / "apps/desktop/src-tauri/tauri.conf.json"
    if json.loads(tauri_config_path.read_text(encoding="utf-8"))["version"] != version:
        raise RuntimeError("tauri.conf.json mismatch")

    cargo_version = get_package_version_from_cargo_toml(REPO_ROOT / "apps/desktop/src-tauri/Cargo.toml")
    if cargo_version != version:
        raise RuntimeError("Cargo.toml [package] version mismatch")


def build_sidecar() -> None:
    """Build the Go sidecar and expose its target filename to the workflow."""

    rust_version = run_command(["rustc", "-vV"]).stdout
    host_line = next((line for line in rust_version.splitlines() if line.startswith("host:")), None)
    if not host_line:
        raise RuntimeError("Failed to detect Rust host triple.")

    triple = host_line.split(":", 1)[1].strip()
    extension = ".exe" if os.name == "nt" else ""
    name = f"local-service-{triple}{extension}"
    binaries_dir = REPO_ROOT / "apps/desktop/src-tauri/binaries"
    binaries_dir.mkdir(parents=True, exist_ok=True)

    subprocess.run(
        [
            "go",
            "build",
            "-trimpath",
            "-o",
            str(binaries_dir / name),
            "./services/local-service/cmd/server",
        ],
        cwd=REPO_ROOT,
        check=True,
    )

    write_workflow_output("sidecar_file_name", name)


def remove_release_by_tag() -> None:
    """Delete an existing GitHub release and tag ref before replacing tip."""

    tag_name = os.environ.get("RELEASE_TAG_NAME", "").strip()
    if not tag_name:
        raise RuntimeError("RELEASE_TAG_NAME is required.")

    github_api_url = os.environ.get("GITHUB_API_URL", "").strip()
    github_repository = os.environ.get("GITHUB_REPOSITORY", "").strip()
    if not github_api_url or not github_repository:
        raise RuntimeError("GITHUB_API_URL and GITHUB_REPOSITORY are required.")

    release = github_api_request(
        "GET", f"{github_api_url}/repos/{github_repository}/releases/tags/{tag_name}"
    )
    if release is not None:
        github_api_request("DELETE", f"{github_api_url}/repos/{github_repository}/releases/{release['id']}")

    tag_ref_url = f"{github_api_url}/repos/{github_repository}/git/refs/tags/{tag_name}"
    tag_ref = github_api_request("GET", tag_ref_url)
    if tag_ref is not None:
        github_api_request("DELETE", tag_ref_url)


def build_argument_parser() -> argparse.ArgumentParser:
    """Create the CLI surface used by GitHub Actions."""

    parser = argparse.ArgumentParser(description="Desktop release workflow helpers.")
    parser.add_argument(
        "action",
        choices=(
            "resolve-metadata",
            "rewrite-version-manifests",
            "verify-version-manifests",
            "build-sidecar",
            "remove-release-by-tag",
        ),
    )
    return parser


def main() -> int:
    """Dispatch the selected action and return an appropriate process exit code."""

    args = build_argument_parser().parse_args()
    actions = {
        "resolve-metadata": resolve_metadata,
        "rewrite-version-manifests": rewrite_version_manifests,
        "verify-version-manifests": verify_version_manifests,
        "build-sidecar": build_sidecar,
        "remove-release-by-tag": remove_release_by_tag,
    }
    actions[args.action]()
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:  # noqa: BLE001
        print(str(exc), file=sys.stderr)
        raise SystemExit(1) from exc
