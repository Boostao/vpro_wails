"""Fail-closed release packaging, CPU witnesses and CI-only version preparation."""

import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import plistlib
import re
import shutil
import struct
import subprocess
import tarfile
import tempfile
import zipfile


ARCHES = ("amd64", "arm64")
GROUPS = ("windows-amd64", "windows-arm64", "macos-universal",
          "linux-amd64", "linux-arm64")
ASSETS = {
    **{f"windows-{arch}": tuple(f"vpro-windows-{arch}{suffix}"
                              for suffix in (".exe", ".zip", "-installer.exe"))
       for arch in ARCHES},
    "macos-universal": ("vpro-macos-universal.dmg", "vpro-macos-universal.zip"),
    **{f"linux-{arch}": tuple(f"vpro-linux-{arch}{suffix}"
                            for suffix in (".AppImage", ".deb", ".tar.gz"))
       for arch in ARCHES},
}
REQUIRED = frozenset(name for names in ASSETS.values() for name in names)
MANIFEST = "SHA256SUMS.txt"
METADATA = ("build/config.yml", "build/windows/info.json",
            "build/windows/nsis/wails_tools.nsh", "build/darwin/Info.plist",
            "build/linux/nfpm/nfpm.yaml")
WINDOWS_TOOLCHAINS = {
    # DuckDB's supported amd64 prebuilt archives require GNU libstdc++/pthread.
    "amd64": {
        "name": "gcc-14.2.0-ucrt",
        "url": "https://github.com/brechtsanders/winlibs_mingw/releases/download/"
               "14.2.0posix-12.0.0-ucrt-r3/"
               "winlibs-x86_64-posix-seh-gcc-14.2.0-mingw-w64ucrt-12.0.0-r3.zip",
        "sha256": "88868d745b807f083a117ff69348d8bc021ad7389aa503379dbed1866efcaeb9",
        "directory": "mingw64",
        "cc": "gcc.exe",
        "cxx": "g++.exe",
    },
    "arm64": {
        "name": "llvm-mingw-20250709-ucrt-x86_64",
        "url": "https://github.com/mstorsjo/llvm-mingw/releases/download/20250709/"
               "llvm-mingw-20250709-ucrt-x86_64.zip",
        "sha256": "82babcd6aae4dc3606e8e0471d816989c384b4bf86a139f184a8b4a1b2c2758d",
        "directory": "llvm-mingw-20250709-ucrt-x86_64",
        "cc": "aarch64-w64-mingw32-clang.exe",
        "cxx": "aarch64-w64-mingw32-clang++.exe",
    },
}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def windows_toolchain(arch):
    require(arch in WINDOWS_TOOLCHAINS, f"Unsupported Windows cgo target: {arch}")
    return dict(WINDOWS_TOOLCHAINS[arch])


def version(raw):
    require(isinstance(raw, str) and len(raw) <= 128, "Invalid release version")
    match = re.fullmatch(
        r"v?((0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*))"
        r"(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?"
        r"(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?", raw, re.ASCII)
    require(match is not None, "Expected SemVer, optionally prefixed with v")
    require(all(int(part) <= 65535 for part in match.groups()[1:4]),
            "Version components exceed the NSIS 16-bit limit")
    if match[5]:
        require(all(not part.isdigit() or part == "0" or not part.startswith("0")
                    for part in match[5].split(".")),
                "Numeric prerelease identifiers must not have leading zeroes")
    return raw.removeprefix("v"), match[1]


def release_version():
    return version(os.environ["RELEASE_VERSION"])


def validate_source_tag(output, tag, commit):
    require(re.fullmatch(r"[0-9a-f]{40}", commit) is not None,
            "Expected the exact GitHub checkout commit")
    name = f"refs/tags/{tag}"
    refs = {}
    for line in output.splitlines():
        fields = line.split("\t")
        require(len(fields) == 2 and re.fullmatch(r"[0-9a-f]{40}", fields[0]) is not None,
                "Malformed remote tag response")
        digest, ref = fields
        require(ref in (name, name + "^{}") and ref not in refs,
                "Unexpected or duplicate remote tag reference")
        refs[ref] = digest
    if not refs:
        return "absent"
    require(name in refs, "Annotated tag has no direct reference")
    require(refs.get(name + "^{}", refs[name]) == commit,
            "Existing release tag points to different source than the built checkout")
    return "matching"


def check_source_tag():
    full, _ = release_version()
    tag = f"v{full}"
    require(os.environ["RELEASE_TAG"] == tag, "Release tag differs from validated version")
    result = subprocess.run(
        ["git", "ls-remote", "--tags", "origin", f"refs/tags/{tag}", f"refs/tags/{tag}^{{}}"],
        check=True, capture_output=True, text=True)
    print("Release source tag:", validate_source_tag(result.stdout, tag, os.environ["GITHUB_SHA"]))


def replace_once(text, pattern, replacement):
    result, count = re.subn(pattern, lambda _: replacement, text, flags=re.MULTILINE)
    require(count == 1, f"Expected one metadata field matching {pattern!r}, got {count}")
    return result


def prepare_metadata(root):
    require(os.environ.get("GITHUB_ACTIONS") == "true" and
            os.environ.get("CI") == "true", "Metadata preparation is CI-only")
    require(root.resolve() == Path(os.environ["GITHUB_WORKSPACE"]).resolve(),
            "Metadata preparation must use the fresh Actions checkout")
    run("git", "diff", "--exit-code", "HEAD", "--", *METADATA, cwd=root)
    full, numeric = release_version()
    changes = {}
    for relative, pattern, replacement in (
        ("build/config.yml", r'^  version: "[^"]*" .*$', f'  version: "{numeric}" # CI release version'),
        ("build/linux/nfpm/nfpm.yaml", r'^version: "[^"]*"$', f'version: "{full}"'),
        ("build/windows/nsis/wails_tools.nsh",
         r'^    !define INFO_PRODUCTVERSION "[^"]*"$',
         f'    !define INFO_PRODUCTVERSION "{numeric}"'),
    ):
        path = root / relative
        changes[path] = replace_once(path.read_text(encoding="utf-8"), pattern, replacement)
    path = root / "build/windows/info.json"
    info = json.loads(path.read_text(encoding="utf-8"))
    require("file_version" in info["fixed"] and "ProductVersion" in info["info"]["0000"],
            "Missing Windows version metadata")
    info["fixed"]["file_version"] = numeric
    info["info"]["0000"]["ProductVersion"] = full
    changes[path] = json.dumps(info, ensure_ascii=False, indent=2) + "\n"
    path = root / "build/darwin/Info.plist"
    info = plistlib.loads(path.read_bytes(), fmt=plistlib.FMT_XML)
    for key in ("CFBundleVersion", "CFBundleShortVersionString"):
        require(key in info, f"Missing macOS metadata: {key}")
        info[key] = numeric
    changes[path] = plistlib.dumps(info, sort_keys=False).decode("utf-8")
    for path, content in changes.items():
        path.write_text(content, encoding="utf-8")


def run(*args, cwd=None):
    subprocess.run(args, cwd=cwd, check=True)


def sha256(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def cpu(data, kind, arch):
    require(arch in (*ARCHES, "universal"), f"Unsupported CPU: {arch}")
    if kind == "pe":
        require(arch in ARCHES and data[:2] == b"MZ" and len(data) >= 64,
                "Invalid PE executable")
        offset = struct.unpack_from("<I", data, 60)[0]
        require(offset + 26 <= len(data) and data[offset:offset + 4] == b"PE\0\0",
                "Missing PE header")
        machine, = struct.unpack_from("<H", data, offset + 4)
        require(machine == {"amd64": 0x8664, "arm64": 0xAA64}[arch],
                f"PE CPU mismatch: {machine:#x}, expected {arch}")
        require(struct.unpack_from("<H", data, offset + 24)[0] == 0x20B,
                "Expected PE32+ application")
    elif kind == "elf":
        require(arch in ARCHES and len(data) >= 64 and
                data[:6] == b"\x7fELF\x02\x01", "Expected little-endian ELF64")
        require(struct.unpack_from("<H", data, 18)[0] ==
                {"amd64": 62, "arm64": 183}[arch], f"ELF CPU mismatch: expected {arch}")
    elif kind == "macho":
        require(arch == "universal" and len(data) >= 8, "Expected universal Mach-O")
        formats = {b"\xca\xfe\xba\xbe": (">", False),
                   b"\xca\xfe\xba\xbf": (">", True),
                   b"\xbe\xba\xfe\xca": ("<", False),
                   b"\xbf\xba\xfe\xca": ("<", True)}
        require(data[:4] in formats, "Expected fat Mach-O, not a host-only app")
        endian, wide = formats[data[:4]]
        count, = struct.unpack_from(endian + "I", data, 4)
        require(count == 2, "Universal app must have exactly two CPU slices")
        stride = 32 if wide else 20
        require(len(data) >= 8 + count * stride, "Truncated fat Mach-O table")
        found, ranges = set(), []
        for index in range(count):
            entry = 8 + index * stride
            machine, = struct.unpack_from(endian + "I", data, entry)
            offset, size = struct.unpack_from(endian + ("QQ" if wide else "II"),
                                             data, entry + 8)
            require(size >= 32 and offset >= 8 + count * stride and
                    offset + size <= len(data), "Invalid Mach-O slice bounds")
            require(data[offset:offset + 4] == b"\xcf\xfa\xed\xfe" and
                    struct.unpack_from("<I", data, offset + 4)[0] == machine,
                    "Fat CPU table does not match its Mach-O slice")
            require(machine not in found, "Duplicate Mach-O CPU slice")
            found.add(machine)
            ranges.append((offset, offset + size))
        require(found == {0x01000007, 0x0100000C}, "Missing x86_64/arm64 universal slices")
        ranges.sort()
        require(ranges[0][1] <= ranges[1][0], "Overlapping Mach-O slices")
    else:
        raise ValueError(f"Unsupported executable format: {kind}")


def check_cpu(path, kind, arch):
    cpu(path.read_bytes(), kind, arch)


def exact_files(directory, expected):
    require(directory.is_dir(), f"Missing artifact directory: {directory}")
    entries = list(directory.iterdir())
    require(all(path.is_file() and not path.is_symlink() and path.stat().st_size > 0
                for path in entries), "Artifacts must be nonempty regular files")
    actual = {path.name for path in entries}
    require(actual == set(expected),
            f"Artifact set mismatch: missing={sorted(set(expected) - actual)}, "
            f"unexpected={sorted(actual - set(expected))}")


def tar_payload(data, name):
    with tarfile.open(fileobj=io.BytesIO(data), mode="r:*") as archive:
        matches = [entry for entry in archive.getmembers()
                   if entry.name.removeprefix("./") == name]
        require(len(matches) == 1 and matches[0].isfile(), f"Missing/duplicate tar file: {name}")
        require(matches[0].size <= 512 * 1024 * 1024, "Tar payload exceeds release limit")
        return archive.extractfile(matches[0]).read()


def deb_members(data):
    require(data[:8] == b"!<arch>\n", "Invalid deb/ar archive")
    members = {}
    offset = 8
    while offset < len(data):
        header = data[offset:offset + 60]
        require(len(header) == 60 and header[58:] == b"`\n", "Invalid ar member header")
        name = header[:16].decode("ascii").strip().removesuffix("/")
        size = int(header[48:58])
        start = offset + 60
        require(size >= 0 and start + size <= len(data) and name not in members,
                "Invalid/duplicate ar member")
        members[name] = data[start:start + size]
        offset = start + size + size % 2
    require(offset == len(data) and members.get("debian-binary") == b"2.0\n",
            "Invalid deb format version")
    return members


def validate_deb(path, arch, full_version):
    members = deb_members(path.read_bytes())
    control = [name for name in members if name.startswith("control.tar")]
    payload = [name for name in members if name.startswith("data.tar")]
    require(len(control) == len(payload) == 1, "Missing/duplicate deb control or data")
    text = tar_payload(members[control[0]], "control").decode("utf-8")
    # nFPM maps the SemVer prerelease separator to Debian's sorting separator.
    deb_version = full_version.replace("-", "~", 1) if "-" in full_version.split("+")[0] else full_version
    for key, expected in (("Architecture", arch),
                          ("Version", deb_version + "-1"), ("Package", "vpro-wails")):
        values = re.findall(rf"^{key}: (.+)$", text, flags=re.MULTILINE)
        require(values == [expected], f"deb {key} mismatch: {values}, expected {expected}")
    cpu(tar_payload(members[payload[0]], "usr/local/bin/vpro-wails"), "elf", arch)


def check_bundle(bundle, numeric):
    check_cpu(bundle / "Contents/MacOS/vpro-wails", "macho", "universal")
    info = plistlib.loads((bundle / "Contents/Info.plist").read_bytes())
    require(info["CFBundleVersion"] == info["CFBundleShortVersionString"] == numeric,
            "macOS bundle version mismatch")


def unique_output(root, suffix):
    paths = list((root / "bin").glob(f"*{suffix}"))
    require(len(paths) == 1 and paths[0].is_file() and not paths[0].is_symlink(),
            f"Expected exactly one bin/*{suffix}, found {len(paths)}")
    return paths[0]


def package(root, platform):
    full, numeric = release_version()
    arch = "universal" if platform == "macos" else os.environ["RELEASE_ARCH"]
    group = f"{platform}-{arch}"
    require(group in ASSETS, f"Unsupported release target: {group}")
    destination = root / "dist-artifacts"
    destination.mkdir()  # Refuse stale outputs from a previous attempt.
    binary = root / "bin/vpro-wails"
    if platform == "windows":
        binary = binary.with_suffix(".exe")
        check_cpu(binary, "pe", arch)
        installer = root / f"bin/vpro-wails-{arch}-installer.exe"
        require(installer.is_file(), f"Missing exact NSIS output: {installer}")
        # NSIS has an x86 launcher even for ARM64; inspect the installed payload.
        with tempfile.TemporaryDirectory() as temporary:
            run("7z", "x", "-y", str(installer), f"-o{temporary}")
            payloads = list(Path(temporary).rglob("vpro-wails.exe"))
            require(len(payloads) == 1, "Missing/ambiguous NSIS application payload")
            check_cpu(payloads[0], "pe", arch)
            require(sha256(payloads[0]) == sha256(binary), "NSIS payload differs from built EXE")
        shutil.copyfile(binary, destination / ASSETS[group][0])
        with zipfile.ZipFile(destination / ASSETS[group][1], "w", zipfile.ZIP_DEFLATED) as archive:
            archive.write(binary, "vpro-wails.exe")
        shutil.copyfile(installer, destination / ASSETS[group][2])
    elif platform == "linux":
        check_cpu(binary, "elf", arch)
        appimage = unique_output(root, ".AppImage")
        check_cpu(appimage, "elf", arch)
        require(appimage.read_bytes()[8:11] == b"AI\x02", "Expected type-2 AppImage")
        with tempfile.TemporaryDirectory() as temporary:
            run(str(appimage.resolve()), "--appimage-extract", cwd=temporary)
            check_cpu(Path(temporary) / "squashfs-root/usr/bin/vpro-wails", "elf", arch)
        deb = unique_output(root, ".deb")
        validate_deb(deb, arch, full)
        shutil.copyfile(appimage, destination / ASSETS[group][0])
        (destination / ASSETS[group][0]).chmod(0o755)
        shutil.copyfile(deb, destination / ASSETS[group][1])
        with tarfile.open(destination / ASSETS[group][2], "w:gz") as archive:
            archive.add(binary, arcname="vpro-wails")
    else:
        bundle = root / "bin/vpro-wails.app"
        check_bundle(bundle, numeric)
        dmg = root / "bin/vpro-wails.dmg"
        require(dmg.is_file(), "Missing exact DMG output")
        with tempfile.TemporaryDirectory() as temporary:
            mount = Path(temporary) / "mount"
            mount.mkdir()
            run("hdiutil", "attach", "-readonly", "-nobrowse", "-mountpoint", str(mount), str(dmg))
            try:
                check_bundle(mount / "vpro-wails.app", numeric)
                require(sha256(mount / "vpro-wails.app/Contents/MacOS/vpro-wails") ==
                        sha256(bundle / "Contents/MacOS/vpro-wails"), "DMG payload differs")
            finally:
                run("hdiutil", "detach", str(mount))
        shutil.copyfile(dmg, destination / ASSETS[group][0])
        run("ditto", "-c", "-k", "--keepParent", str(bundle), str(destination / ASSETS[group][1]))
    exact_files(destination, ASSETS[group])
    validate_portable(destination, group, full)
    witness = {
        "schema": 1, "group": group, "version": full, "commit": os.environ["GITHUB_SHA"],
        "architectures": list(ARCHES) if arch == "universal" else [arch],
        "sha256": {name: sha256(destination / name) for name in ASSETS[group]},
    }
    (destination / f"release-witness-{group}.json").write_text(
        json.dumps(witness, sort_keys=True) + "\n", encoding="utf-8")


def validate_portable(directory, group, full):
    if group.startswith("windows-"):
        arch = group.removeprefix("windows-")
        binary = (directory / ASSETS[group][0]).read_bytes()
        cpu(binary, "pe", arch)
        with zipfile.ZipFile(directory / ASSETS[group][1]) as archive:
            require(archive.namelist() == ["vpro-wails.exe"], "Unexpected Windows ZIP entries")
            require(archive.read("vpro-wails.exe") == binary, "Windows ZIP payload differs")
    elif group.startswith("linux-"):
        arch = group.removeprefix("linux-")
        check_cpu(directory / ASSETS[group][0], "elf", arch)
        validate_deb(directory / ASSETS[group][1], arch, full)
        cpu(tar_payload((directory / ASSETS[group][2]).read_bytes(), "vpro-wails"), "elf", arch)
    else:
        with zipfile.ZipFile(directory / ASSETS[group][1]) as archive:
            names = archive.namelist()
            executable = "vpro-wails.app/Contents/MacOS/vpro-wails"
            info_path = "vpro-wails.app/Contents/Info.plist"
            require(names.count(executable) == names.count(info_path) == 1,
                    "Missing/duplicate macOS ZIP bundle entries")
            cpu(archive.read(executable), "macho", "universal")
            info = plistlib.loads(archive.read(info_path))
            numeric = version(full)[1]
            require(info["CFBundleVersion"] == info["CFBundleShortVersionString"] == numeric,
                    "macOS ZIP version mismatch")


def manifest(directory):
    full, _ = release_version()
    witnesses = {f"release-witness-{group}.json" for group in GROUPS}
    exact_files(directory, REQUIRED | witnesses)
    for group in GROUPS:
        witness = json.loads((directory / f"release-witness-{group}.json").read_text(encoding="utf-8"))
        expected = {
            "schema": 1, "group": group, "version": full, "commit": os.environ["GITHUB_SHA"],
            "architectures": list(ARCHES) if group == "macos-universal" else [group.split("-")[1]],
            "sha256": {name: sha256(directory / name) for name in ASSETS[group]},
        }
        require(witness == expected, f"Invalid provenance/CPU witness: {group}")
        validate_portable(directory, group, full)
    content = "".join(f"{sha256(directory / name)}  {name}\n" for name in sorted(REQUIRED))
    (directory / MANIFEST).write_text(content, encoding="ascii", newline="\n")
    Path("release-assets.txt").write_text(
        "".join(f"{directory / name}\n" for name in sorted(REQUIRED | {MANIFEST})),
        encoding="utf-8", newline="\n")


def parse_manifest(content):
    lines = content.splitlines()
    require(len(lines) == len(REQUIRED), "Checksum manifest has the wrong number of entries")
    entries = {}
    for line in lines:
        match = re.fullmatch(r"([0-9a-f]{64})  ([A-Za-z0-9._-]+)", line)
        require(match is not None, "Malformed checksum manifest line")
        digest, name = match.groups()
        require(name in REQUIRED and name not in entries, "Unexpected/duplicate checksum entry")
        entries[name] = digest
    require(set(entries) == REQUIRED, "Incomplete checksum manifest")
    return entries


def verify(directory, trusted_manifest):
    content = trusted_manifest.read_text(encoding="ascii")
    expected = parse_manifest(content)
    exact_files(directory, REQUIRED | {MANIFEST})
    require((directory / MANIFEST).read_bytes() == trusted_manifest.read_bytes(),
            "Downloaded checksum manifest differs from validated manifest")
    for name, digest in expected.items():
        require(sha256(directory / name) == digest, f"SHA256 mismatch: {name}")


def remote(path):
    assets = json.loads(path.read_text(encoding="utf-8"))["assets"]
    names = [asset["name"] for asset in assets]
    require(len(names) == len(REQUIRED) + 1 and set(names) == REQUIRED | {MANIFEST},
            "Remote release has missing, duplicate or unexpected assets")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("version")
    commands.add_parser("source-tag")
    commands.add_parser("metadata")
    toolchain = commands.add_parser("toolchain")
    toolchain.add_argument("--arch", choices=ARCHES, required=True)
    packaging = commands.add_parser("package")
    packaging.add_argument("platform", choices=("windows", "macos", "linux"))
    architecture = commands.add_parser("cpu")
    architecture.add_argument("path", type=Path)
    architecture.add_argument("--format", choices=("pe", "elf", "macho"), required=True)
    architecture.add_argument("--arch", choices=(*ARCHES, "universal"), required=True)
    for command in ("manifest", "remote", "verify"):
        subparser = commands.add_parser(command)
        subparser.add_argument("path", type=Path)
        if command == "verify":
            subparser.add_argument("--manifest", type=Path, required=True)
    args = parser.parse_args()
    if args.command == "version":
        full, _ = release_version()
        with Path(os.environ["GITHUB_OUTPUT"]).open("a", encoding="utf-8") as output:
            output.write(f"version={full}\ntag=v{full}\n")
    elif args.command == "toolchain":
        print(json.dumps(windows_toolchain(args.arch), sort_keys=True))
    elif args.command == "source-tag":
        check_source_tag()
    elif args.command == "metadata":
        prepare_metadata(Path.cwd())
    elif args.command == "package":
        package(Path.cwd(), args.platform)
    elif args.command == "cpu":
        check_cpu(args.path, args.format, args.arch)
    elif args.command == "manifest":
        manifest(args.path)
    elif args.command == "remote":
        remote(args.path)
    else:
        verify(args.path, args.manifest)


if __name__ == "__main__":
    main()
