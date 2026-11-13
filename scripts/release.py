"""Fail-closed release packaging, CPU witnesses and CI-only version preparation."""

import argparse
from datetime import datetime, timezone
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


REPOSITORY = "Boostao/vpro_wails"
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
RELEASE_MANIFEST = "release-manifest.json"
DEFAULT_STANDARDS = {
    "becVersion": "BECdb v13 (2026-05-28)",
    "taxonomyVersion": "BCPlantTaxonomy v19 (2026-02-09)",
    "lmh25Edition": "LMH25 (2023)",
}
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


WINDOWS_SYSTEM_DLLS = frozenset({
    "advapi32.dll", "bcrypt.dll", "bcryptprimitives.dll", "cfgmgr32.dll",
    "comctl32.dll", "comdlg32.dll", "crypt32.dll", "d2d1.dll", "d3d11.dll",
    "dbghelp.dll", "dnsapi.dll", "dwmapi.dll", "dwrite.dll", "dxgi.dll",
    "gdi32.dll", "gdiplus.dll", "imm32.dll", "iphlpapi.dll", "kernel32.dll",
    "kernelbase.dll", "msimg32.dll", "msvcrt.dll", "ncrypt.dll", "netapi32.dll",
    "ntdll.dll", "ole32.dll", "oleaut32.dll", "opengl32.dll", "powrprof.dll",
    "profapi.dll", "psapi.dll", "rpcrt4.dll", "rstrtmgr.dll", "secur32.dll",
    "setupapi.dll", "shcore.dll", "shell32.dll", "shlwapi.dll", "ucrtbase.dll",
    "user32.dll", "userenv.dll", "uxtheme.dll", "version.dll", "winhttp.dll",
    "wininet.dll", "winmm.dll", "winspool.drv", "ws2_32.dll", "wsock32.dll",
    "wtsapi32.dll",
})


def is_windows_system_dll(name):
    lower = name.lower()
    if lower in WINDOWS_SYSTEM_DLLS:
        return True
    return lower.startswith(("api-ms-win-", "ext-ms-win-")) and lower.endswith(".dll")


def pe_imports(data, offset=None):
    if offset is None:
        require(data[:2] == b"MZ" and len(data) >= 64, "Invalid PE executable")
        offset = struct.unpack_from("<I", data, 60)[0]
    require(offset + 26 <= len(data) and data[offset:offset + 4] == b"PE\0\0",
            "Missing PE header")
    require(struct.unpack_from("<H", data, offset + 24)[0] == 0x20B,
            "Expected PE32+ application")
    num_sections, = struct.unpack_from("<H", data, offset + 6)
    opt_size, = struct.unpack_from("<H", data, offset + 20)
    opt_offset = offset + 24
    sec_offset = opt_offset + opt_size
    require(sec_offset + num_sections * 40 <= len(data), "Truncated PE section table")
    if opt_size == 0 and num_sections == 0:
        return []
    require(opt_size >= 112, "Truncated PE32+ optional header")
    num_rva_and_sizes, = struct.unpack_from("<I", data, opt_offset + 108)
    require(num_rva_and_sizes <= (opt_size - 112) // 8,
            "Truncated PE32+ data directories")
    sections = []
    for index in range(num_sections):
        vsize, vrva, rsize, roff = struct.unpack_from("<IIII", data, sec_offset + index * 40 + 8)
        require(rsize == 0 or roff + rsize <= len(data), "Truncated PE section data")
        sections.append((vsize, vrva, rsize, roff))

    def rva_to_offset(rva):
        for vsize, vrva, rsize, roff in sections:
            span = max(vsize, rsize)
            if vrva <= rva < vrva + span:
                delta = rva - vrva
                require(delta < rsize, f"PE RVA {rva:#x} points outside raw section data")
                return roff + delta
        raise ValueError(f"Unmapped PE RVA: {rva:#x}")

    def read_dll_name(rva):
        pos = rva_to_offset(rva)
        end = data.find(b"\x00", pos)
        require(end != -1 and 0 < end - pos <= 256, "Malformed PE import DLL name")
        try:
            name = data[pos:end].decode("ascii")
        except UnicodeDecodeError as exc:
            raise ValueError("Non-ASCII PE import DLL name") from exc
        require(all(32 < ord(ch) < 127 for ch in name),
                "Control or whitespace character in PE import DLL name")
        return name

    dlls = []
    if num_rva_and_sizes > 1:
        imp_rva, imp_size = struct.unpack_from("<II", data, opt_offset + 112 + 8)
        if imp_rva != 0 or imp_size != 0:
            require(imp_rva != 0 and imp_size >= 20, "Malformed PE import directory")
            pos = rva_to_offset(imp_rva)
            while True:
                require(pos + 20 <= len(data), "Truncated PE import descriptor table")
                descriptor = struct.unpack_from("<IIIII", data, pos)
                if descriptor == (0, 0, 0, 0, 0):
                    break
                name_rva = descriptor[3]
                require(name_rva != 0, "PE import descriptor missing DLL name RVA")
                dlls.append(read_dll_name(name_rva))
                pos += 20
    if num_rva_and_sizes > 13:
        delay_rva, delay_size = struct.unpack_from("<II", data, opt_offset + 112 + 13 * 8)
        if delay_rva != 0 or delay_size != 0:
            require(delay_rva != 0 and delay_size >= 32, "Malformed PE delay-import directory")
            pos = rva_to_offset(delay_rva)
            while True:
                require(pos + 32 <= len(data), "Truncated PE delay-import descriptor table")
                fields = struct.unpack_from("<IIIIIIII", data, pos)
                if fields == (0,) * 8:
                    break
                attrs, name_rva = fields[0], fields[1]
                require((attrs & 1) != 0 and name_rva != 0,
                        "Invalid PE delay-import descriptor")
                dlls.append(read_dll_name(name_rva))
                pos += 32
    return dlls


def validate_pe_imports(data, offset=None):
    dlls = pe_imports(data, offset)
    non_system = sorted({name for name in dlls if not is_windows_system_dll(name)},
                        key=str.lower)
    require(not non_system,
            f"Non-system Windows DLL dependency in PE imports: {', '.join(non_system)}")


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
        validate_pe_imports(data, offset)
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
                          ("Version", deb_version + "-1"), ("Package", "vpro")):
        values = re.findall(rf"^{key}: (.+)$", text, flags=re.MULTILINE)
        require(values == [expected], f"deb {key} mismatch: {values}, expected {expected}")
    cpu(tar_payload(members[payload[0]], "usr/bin/VPro"), "elf", arch)


def check_bundle(bundle, numeric):
    check_cpu(bundle / "Contents/MacOS/VPro", "macho", "universal")
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
    binary = root / "bin/VPro"
    if platform == "windows":
        binary = binary.with_suffix(".exe")
        check_cpu(binary, "pe", arch)
        installer = root / f"bin/VPro-{arch}-installer.exe"
        require(installer.is_file(), f"Missing exact NSIS output: {installer}")
        # NSIS has an x86 launcher even for ARM64; inspect the installed payload.
        with tempfile.TemporaryDirectory() as temporary:
            run("7z", "x", "-y", str(installer), f"-o{temporary}")
            payloads = list(Path(temporary).rglob("VPro.exe"))
            require(len(payloads) == 1, "Missing/ambiguous NSIS application payload")
            check_cpu(payloads[0], "pe", arch)
            require(sha256(payloads[0]) == sha256(binary), "NSIS payload differs from built EXE")
        shutil.copyfile(binary, destination / ASSETS[group][0])
        with zipfile.ZipFile(destination / ASSETS[group][1], "w", zipfile.ZIP_DEFLATED) as archive:
            archive.write(binary, "VPro.exe")
        shutil.copyfile(installer, destination / ASSETS[group][2])
    elif platform == "linux":
        check_cpu(binary, "elf", arch)
        appimage = unique_output(root, ".AppImage")
        check_cpu(appimage, "elf", arch)
        require(appimage.read_bytes()[8:11] == b"AI\x02", "Expected type-2 AppImage")
        with tempfile.TemporaryDirectory() as temporary:
            run(str(appimage.resolve()), "--appimage-extract", cwd=temporary)
            check_cpu(Path(temporary) / "squashfs-root/usr/bin/VPro", "elf", arch)
        deb = unique_output(root, ".deb")
        validate_deb(deb, arch, full)
        shutil.copyfile(appimage, destination / ASSETS[group][0])
        (destination / ASSETS[group][0]).chmod(0o755)
        shutil.copyfile(deb, destination / ASSETS[group][1])
        with tarfile.open(destination / ASSETS[group][2], "w:gz") as archive:
            archive.add(binary, arcname="VPro")
    else:
        bundle = root / "bin/VPro.app"
        check_bundle(bundle, numeric)
        dmg = root / "bin/VPro.dmg"
        require(dmg.is_file(), "Missing exact DMG output")
        with tempfile.TemporaryDirectory() as temporary:
            mount = Path(temporary) / "mount"
            mount.mkdir()
            run("hdiutil", "attach", "-readonly", "-nobrowse", "-mountpoint", str(mount), str(dmg))
            try:
                check_bundle(mount / "VPro.app", numeric)
                require(sha256(mount / "VPro.app/Contents/MacOS/VPro") ==
                        sha256(bundle / "Contents/MacOS/VPro"), "DMG payload differs")
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
            require(archive.namelist() == ["VPro.exe"], "Unexpected Windows ZIP entries")
            require(archive.read("VPro.exe") == binary, "Windows ZIP payload differs")
    elif group.startswith("linux-"):
        arch = group.removeprefix("linux-")
        check_cpu(directory / ASSETS[group][0], "elf", arch)
        validate_deb(directory / ASSETS[group][1], arch, full)
        cpu(tar_payload((directory / ASSETS[group][2]).read_bytes(), "VPro"), "elf", arch)
    else:
        with zipfile.ZipFile(directory / ASSETS[group][1]) as archive:
            names = archive.namelist()
            executable = "VPro.app/Contents/MacOS/VPro"
            info_path = "VPro.app/Contents/Info.plist"
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


def verify(directory, trusted_manifest, *, release_manifest_path=None):
    content = trusted_manifest.read_text(encoding="ascii")
    expected = parse_manifest(content)
    expected_set = REQUIRED | {MANIFEST}
    if release_manifest_path is not None:
        expected_set = expected_set | {RELEASE_MANIFEST}
    exact_files(directory, expected_set)
    require((directory / MANIFEST).read_bytes() == trusted_manifest.read_bytes(),
            "Downloaded checksum manifest differs from validated manifest")
    for name, digest in expected.items():
        require(sha256(directory / name) == digest, f"SHA256 mismatch: {name}")
    if release_manifest_path is not None:
        trusted_bytes = release_manifest_path.read_bytes()
        require((directory / RELEASE_MANIFEST).read_bytes() == trusted_bytes,
                "Downloaded release-manifest.json differs from validated manifest")
        validate_release_manifest(json.loads(trusted_bytes.decode("utf-8")))


def remote(path, *, require_release_manifest=False):
    assets = json.loads(path.read_text(encoding="utf-8"))["assets"]
    names = [asset["name"] for asset in assets]
    require(len(names) == len(set(names)), "Remote release has duplicate asset names")
    base_expected = REQUIRED | {MANIFEST}
    with_portal_manifest = base_expected | {RELEASE_MANIFEST}
    if require_release_manifest:
        require(set(names) == with_portal_manifest,
                "Remote release is missing release-manifest.json or has unexpected assets")
    else:
        require(set(names) in (base_expected, with_portal_manifest),
                "Remote release has missing, duplicate or unexpected assets")


def standards_metadata(root=None):
    base = (root or Path(__file__).resolve().parents[1]) / "resources/standards-2026-provenance.json"
    standards = dict(DEFAULT_STANDARDS)
    if base.is_file():
        provenance = json.loads(base.read_text(encoding="utf-8"))
        for source in provenance.get("sources", ()):
            label = source.get("version", "")
            if label.startswith("BECdb_v13_2026"):
                date_suffix = label.split("(", 1)[1].rstrip(")") if "(" in label else "2026"
                standards["becVersion"] = f"BECdb v13 ({date_suffix})"
            elif label.startswith("BCPlantTaxa_v19"):
                date_suffix = label.split("(", 1)[1].rstrip(")") if "(" in label else "2026"
                standards["taxonomyVersion"] = f"BCPlantTaxonomy v19 ({date_suffix})"
    return standards


def validate_download_url(url, name, tag, repository=REPOSITORY):
    require(isinstance(url, str) and "?" not in url and "#" not in url,
            f"Invalid download URL for {name}")
    expected = f"https://github.com/{repository}/releases/download/{tag}/{name}"
    require(url == expected, f"Unexpected download URL for {name}: {url} != {expected}")
    return url


def validate_channel_entry(entry, channel_name, repository=REPOSITORY):
    require(isinstance(entry, dict), f"Channel {channel_name} must be an object or null")
    full_ver = entry.get("version")
    require(isinstance(full_ver, str), f"Channel {channel_name} is missing version")
    normalized, _ = version(full_ver)
    require(normalized == full_ver, f"Channel {channel_name} version must not carry leading v")
    tag = entry.get("tag")
    require(tag == f"v{full_ver}", f"Channel {channel_name} tag must match v{full_ver}")
    published_at = entry.get("publishedAt")
    require(isinstance(published_at, str) and re.fullmatch(
        r"\d{4}-\d{2}-\d{2}(?:T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)?", published_at) is not None,
        f"Channel {channel_name} has invalid ISO-8601 publishedAt")
    prerelease = entry.get("prerelease")
    expected_prerelease = channel_name == "preview"
    require(type(prerelease) is bool and prerelease is expected_prerelease,
            f"Channel {channel_name} prerelease must be {expected_prerelease}")
    require(type(entry.get("unsignedNotice")) is bool,
            f"Channel {channel_name} unsignedNotice must be boolean")
    validate_download_url(entry.get("sha256sumsUrl"), MANIFEST, tag, repository)
    standards = entry.get("standards")
    require(isinstance(standards, dict), f"Channel {channel_name} standards must be an object")
    for key in ("becVersion", "taxonomyVersion", "lmh25Edition"):
        require(isinstance(standards.get(key), str) and len(standards[key].strip()) > 0,
                f"Channel {channel_name} standards.{key} must be a non-empty string")
    assets = entry.get("assets")
    require(isinstance(assets, list) and len(assets) > 0,
            f"Channel {channel_name} assets must be a non-empty list")
    allowed_names = REQUIRED | {MANIFEST}
    seen = set()
    for asset in assets:
        require(isinstance(asset, dict), f"Channel {channel_name} contains malformed asset")
        name = asset.get("name")
        require(isinstance(name, str) and name in allowed_names and name not in seen,
                f"Channel {channel_name} contains unexpected or duplicate asset: {name!r}")
        seen.add(name)
        validate_download_url(asset.get("url"), name, tag, repository)
        digest = asset.get("sha256")
        require(isinstance(digest, str) and re.fullmatch(r"[0-9a-f]{64}", digest) is not None,
                f"Channel {channel_name} asset {name} has invalid sha256")
        size_bytes = asset.get("sizeBytes")
        require(type(size_bytes) is int and size_bytes > 0,
                f"Channel {channel_name} asset {name} has invalid sizeBytes")
    require(REQUIRED.issubset(seen),
            f"Channel {channel_name} is missing required release assets")


def validate_release_manifest(doc):
    require(isinstance(doc, dict), "Release manifest must be a JSON object")
    require(set(doc) == {"schemaVersion", "repository", "defaultChannel", "channels"},
            "Unexpected top-level keys in release-manifest.json")
    require(type(doc.get("schemaVersion")) is int and doc["schemaVersion"] == 1,
            "Unsupported release-manifest.json schemaVersion")
    require(doc.get("repository") == REPOSITORY,
            f"Unexpected release-manifest.json repository: {doc.get('repository')!r}")
    require(doc.get("defaultChannel") in ("stable", "preview"),
            "defaultChannel must be 'stable' or 'preview'")
    channels = doc.get("channels")
    require(isinstance(channels, dict) and "stable" in channels and
            set(channels).issubset({"stable", "preview"}),
            "channels must contain 'stable' and optional 'preview' only")
    for channel_name in ("stable", "preview"):
        entry = channels.get(channel_name)
        if entry is not None:
            validate_channel_entry(entry, channel_name, doc["repository"])
    return doc


def release_manifest(directory, output, *, channel=None, published_at=None,
                     base_manifest=None, default_channel=None, root=None):
    full, _ = release_version()
    tag = f"v{full}"
    manifest_file = directory / MANIFEST
    require(manifest_file.is_file() and not manifest_file.is_symlink() and
            manifest_file.stat().st_size > 0, f"Missing validated {MANIFEST} in {directory}")
    expected_hashes = parse_manifest(manifest_file.read_text(encoding="ascii"))
    for name in REQUIRED:
        asset_path = directory / name
        require(asset_path.is_file() and not asset_path.is_symlink() and
                asset_path.stat().st_size > 0, f"Missing or empty release asset: {name}")
        require(sha256(asset_path) == expected_hashes[name],
                f"Asset {name} does not match {MANIFEST}")
    env_prerelease = os.environ.get("RELEASE_PRERELEASE", "").strip().lower()
    inferred_prerelease = env_prerelease == "true" or "-" in full.split("+", 1)[0]
    if channel is None:
        target_channel = "preview" if inferred_prerelease else "stable"
    else:
        require(channel in ("stable", "preview"), f"Unsupported release channel: {channel}")
        if channel == "stable":
            require(not inferred_prerelease,
                    "Prerelease version or RELEASE_PRERELEASE=true cannot publish to stable channel")
        target_channel = channel
    timestamp = published_at or os.environ.get("RELEASE_PUBLISHED_AT") or (
        datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    )
    doc = {
        "schemaVersion": 1,
        "repository": REPOSITORY,
        "defaultChannel": "stable",
        "channels": {"stable": None, "preview": None},
    }
    if base_manifest is not None and base_manifest.is_file():
        existing = validate_release_manifest(json.loads(base_manifest.read_text(encoding="utf-8")))
        doc["defaultChannel"] = existing["defaultChannel"]
        doc["channels"]["stable"] = existing["channels"].get("stable")
        doc["channels"]["preview"] = existing["channels"].get("preview")
    if default_channel is not None:
        require(default_channel in ("stable", "preview"), "Invalid default_channel")
        doc["defaultChannel"] = default_channel
    asset_names = sorted(REQUIRED | {MANIFEST})
    doc["channels"][target_channel] = {
        "tag": tag,
        "version": full,
        "publishedAt": timestamp,
        "prerelease": target_channel == "preview",
        "unsignedNotice": True,
        "sha256sumsUrl": f"https://github.com/{REPOSITORY}/releases/download/{tag}/{MANIFEST}",
        "standards": standards_metadata(root),
        "assets": [
            {
                "name": name,
                "url": f"https://github.com/{REPOSITORY}/releases/download/{tag}/{name}",
                "sha256": sha256(directory / name),
                "sizeBytes": (directory / name).stat().st_size,
            }
            for name in asset_names
        ],
    }
    validate_release_manifest(doc)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(doc, indent=2) + "\n", encoding="utf-8", newline="\n")
    return doc


def sync_release_manifest(output, *, base_manifest=None, releases_json=None,
                          asset_loader=None, repository=REPOSITORY):
    default_channel = "stable"
    if base_manifest is not None and base_manifest.is_file():
        existing = validate_release_manifest(json.loads(base_manifest.read_text(encoding="utf-8")))
        default_channel = existing["defaultChannel"]
    if releases_json is not None:
        releases = json.loads(releases_json.read_text(encoding="utf-8"))
    else:
        result = subprocess.run(
            ["gh", "api", f"repos/{repository}/releases?per_page=50"],
            check=True, capture_output=True, text=True,
        )
        releases = json.loads(result.stdout)
    require(isinstance(releases, list), "Expected GitHub releases JSON array")

    required_remote = REQUIRED | {MANIFEST, RELEASE_MANIFEST}
    matched = {}
    for rel in releases:
        if not isinstance(rel, dict) or rel.get("draft") is not False:
            continue
        is_prerelease = rel.get("prerelease")
        if type(is_prerelease) is not bool:
            continue
        channel_name = "preview" if is_prerelease else "stable"
        if channel_name in matched:
            continue
        tag = rel.get("tag_name")
        assets = rel.get("assets")
        if not isinstance(tag, str) or not tag or not isinstance(assets, list):
            continue
        asset_names = {a.get("name") for a in assets if isinstance(a, dict)}
        if not required_remote.issubset(asset_names):
            continue
        matched[channel_name] = tag
        if "stable" in matched and "preview" in matched:
            break

    doc = {
        "schemaVersion": 1,
        "repository": repository,
        "defaultChannel": default_channel,
        "channels": {"stable": None, "preview": None},
    }
    for channel_name, tag in matched.items():
        if asset_loader is not None:
            raw = asset_loader(tag, RELEASE_MANIFEST)
        else:
            with tempfile.TemporaryDirectory() as tmp:
                run("gh", "release", "download", tag, "--repo", repository,
                    "--pattern", RELEASE_MANIFEST, "--dir", tmp)
                raw = (Path(tmp) / RELEASE_MANIFEST).read_text(encoding="utf-8")
        remote_doc = validate_release_manifest(json.loads(raw))
        entry = remote_doc["channels"].get(channel_name)
        require(entry is not None and entry.get("tag") == tag,
                f"Downloaded {RELEASE_MANIFEST} for {tag} is missing matching {channel_name} channel")
        doc["channels"][channel_name] = entry

    validate_release_manifest(doc)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(doc, indent=2) + "\n", encoding="utf-8", newline="\n")
    return doc


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
    portal_manifest = commands.add_parser("release-manifest")
    portal_manifest.add_argument("path", type=Path)
    portal_manifest.add_argument("--output", type=Path, required=True)
    portal_manifest.add_argument("--channel", choices=("stable", "preview"))
    portal_manifest.add_argument("--published-at")
    portal_manifest.add_argument("--base-manifest", type=Path)
    portal_manifest.add_argument("--default-channel", choices=("stable", "preview"))
    sync_manifest = commands.add_parser("sync-release-manifest")
    sync_manifest.add_argument("--output", type=Path, required=True)
    sync_manifest.add_argument("--base-manifest", type=Path)
    sync_manifest.add_argument("--releases-json", type=Path)
    for command in ("manifest", "remote", "verify"):
        subparser = commands.add_parser(command)
        subparser.add_argument("path", type=Path)
        if command == "remote":
            subparser.add_argument("--require-release-manifest", action="store_true")
        if command == "verify":
            subparser.add_argument("--manifest", type=Path, required=True)
            subparser.add_argument("--release-manifest", type=Path)
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
    elif args.command == "release-manifest":
        release_manifest(
            args.path,
            args.output,
            channel=args.channel,
            published_at=args.published_at,
            base_manifest=args.base_manifest,
            default_channel=args.default_channel,
        )
    elif args.command == "sync-release-manifest":
        sync_release_manifest(
            args.output,
            base_manifest=args.base_manifest,
            releases_json=args.releases_json,
        )
    elif args.command == "remote":
        remote(args.path, require_release_manifest=args.require_release_manifest)
    else:
        verify(args.path, args.manifest, release_manifest_path=args.release_manifest)


if __name__ == "__main__":
    main()
