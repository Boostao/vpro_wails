import contextlib
import hashlib
import importlib.util
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
import unittest
from unittest.mock import patch
import zipfile


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("release", ROOT / "scripts/release.py")
release = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(release)

# Independent ordered release contract, not generated from the implementation.
EXPECTED = (
    "vpro-linux-amd64.AppImage", "vpro-linux-amd64.deb", "vpro-linux-amd64.tar.gz",
    "vpro-linux-arm64.AppImage", "vpro-linux-arm64.deb", "vpro-linux-arm64.tar.gz",
    "vpro-macos-universal.dmg", "vpro-macos-universal.zip",
    "vpro-windows-amd64-installer.exe", "vpro-windows-amd64.exe", "vpro-windows-amd64.zip",
    "vpro-windows-arm64-installer.exe", "vpro-windows-arm64.exe", "vpro-windows-arm64.zip",
)


def pe(arch, imports=("KERNEL32.dll", "api-ms-win-crt-runtime-l1-1-0.dll"), delay_imports=()):
    pe_offset = 128
    opt_offset = pe_offset + 24
    opt_size = 240
    sec_offset = opt_offset + opt_size
    rdata_rva = 0x1000
    rdata_raw = 512

    section_payload = bytearray()
    imp_rva = imp_size = 0
    if imports:
        imp_rva = rdata_rva + len(section_payload)
        imp_size = (len(imports) + 1) * 20
        desc_start = len(section_payload)
        section_payload.extend(b"\x00" * imp_size)
        for index, dll in enumerate(imports):
            name_rva = rdata_rva + len(section_payload)
            section_payload.extend(dll.encode("ascii") + b"\x00")
            struct.pack_into("<IIIII", section_payload, desc_start + index * 20,
                             0, 0, 0, name_rva, 0)

    delay_rva = delay_size = 0
    if delay_imports:
        if len(section_payload) % 4:
            section_payload.extend(b"\x00" * (4 - len(section_payload) % 4))
        delay_rva = rdata_rva + len(section_payload)
        delay_size = (len(delay_imports) + 1) * 32
        desc_start = len(section_payload)
        section_payload.extend(b"\x00" * delay_size)
        for index, dll in enumerate(delay_imports):
            name_rva = rdata_rva + len(section_payload)
            section_payload.extend(dll.encode("ascii") + b"\x00")
            struct.pack_into("<IIIIIIII", section_payload, desc_start + index * 32,
                             1, name_rva, 0, 0, 0, 0, 0, 0)

    raw_size = max(64, ((len(section_payload) + 15) // 16) * 16)
    section_payload.extend(b"\x00" * (raw_size - len(section_payload)))

    data = bytearray(rdata_raw + len(section_payload))
    data[:2] = b"MZ"
    struct.pack_into("<I", data, 60, pe_offset)
    data[pe_offset:pe_offset + 4] = b"PE\0\0"
    struct.pack_into("<HH", data, pe_offset + 4,
                     0x8664 if arch == "amd64" else 0xAA64, 1)
    struct.pack_into("<H", data, pe_offset + 20, opt_size)
    struct.pack_into("<H", data, opt_offset, 0x20B)
    struct.pack_into("<I", data, opt_offset + 108, 16)
    struct.pack_into("<II", data, opt_offset + 112 + 8, imp_rva, imp_size)
    struct.pack_into("<II", data, opt_offset + 112 + 13 * 8, delay_rva, delay_size)
    data[sec_offset:sec_offset + 6] = b".rdata"
    struct.pack_into("<IIII", data, sec_offset + 8,
                     len(section_payload), rdata_rva, raw_size, rdata_raw)
    data[rdata_raw:rdata_raw + len(section_payload)] = section_payload
    return bytes(data)


def elf(arch, appimage=False):
    data = bytearray(128)
    data[:6] = b"\x7fELF\x02\x01"
    struct.pack_into("<H", data, 18, 62 if arch == "amd64" else 183)
    if appimage:
        data[8:11] = b"AI\x02"
    return bytes(data)


def universal():
    data = bytearray(256)
    struct.pack_into(">II", data, 0, 0xCAFEBABE, 2)
    for index, (machine, offset) in enumerate(((0x01000007, 128), (0x0100000C, 192))):
        struct.pack_into(">IIIII", data, 8 + index * 20, machine, 0, offset, 64, 0)
        data[offset:offset + 4] = b"\xcf\xfa\xed\xfe"
        struct.pack_into("<I", data, offset + 4, machine)
    return bytes(data)


def tar_bytes(entries):
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode="w:gz") as archive:
        for name, data in entries:
            entry = tarfile.TarInfo(name)
            entry.size = len(data)
            archive.addfile(entry, io.BytesIO(data))
    return output.getvalue()


def deb(arch, version="2.3.4"):
    control = f"Package: vpro\nVersion: {version}-1\nArchitecture: {arch}\n".encode()
    entries = (("debian-binary", b"2.0\n"),
               ("control.tar.gz", tar_bytes([("./control", control)])),
               ("data.tar.gz", tar_bytes([("./usr/bin/VPro", elf(arch))])))
    data = bytearray(b"!<arch>\n")
    for name, content in entries:
        header = f"{name + '/':<16}{0:<12}{0:<6}{0:<6}{'100644':<8}{len(content):<10}`\n"
        data.extend(header.encode("ascii"))
        data.extend(content)
        if len(content) % 2:
            data.extend(b"\n")
    return bytes(data)


def bundle_info():
    return plistlib.dumps({"CFBundleVersion": "2.3.4", "CFBundleShortVersionString": "2.3.4"})


def asset_fixture(directory):
    for arch in ("amd64", "arm64"):
        (directory / f"vpro-windows-{arch}.exe").write_bytes(pe(arch))
        (directory / f"vpro-windows-{arch}-installer.exe").write_bytes(b"installer fixture")
        with zipfile.ZipFile(directory / f"vpro-windows-{arch}.zip", "w") as archive:
            archive.writestr("VPro.exe", pe(arch))
        (directory / f"vpro-linux-{arch}.AppImage").write_bytes(elf(arch, True))
        (directory / f"vpro-linux-{arch}.deb").write_bytes(deb(arch))
        (directory / f"vpro-linux-{arch}.tar.gz").write_bytes(tar_bytes([("VPro", elf(arch))]))
    (directory / "vpro-macos-universal.dmg").write_bytes(b"DMG fixture")
    with zipfile.ZipFile(directory / "vpro-macos-universal.zip", "w") as archive:
        archive.writestr("VPro.app/Contents/MacOS/VPro", universal())
        archive.writestr("VPro.app/Contents/Info.plist", bundle_info())
    for group in ("windows-amd64", "windows-arm64", "macos-universal", "linux-amd64", "linux-arm64"):
        names = [name for name in EXPECTED if name.startswith("vpro-" + group)]
        witness = {"schema": 1, "group": group, "version": "2.3.4", "commit": "abc123",
                   "architectures": ["amd64", "arm64"] if group == "macos-universal" else [group.split("-")[1]],
                   "sha256": {name: hashlib.sha256((directory / name).read_bytes()).hexdigest() for name in names}}
        (directory / f"release-witness-{group}.json").write_text(json.dumps(witness), encoding="utf-8")


@contextlib.contextmanager
def working_directory(path):
    previous = Path.cwd()
    os.chdir(path)
    try:
        yield
    finally:
        os.chdir(previous)


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.environment = patch.dict(os.environ, {"RELEASE_VERSION": "v2.3.4", "GITHUB_SHA": "abc123"})
        self.environment.start()
        self.addCleanup(self.environment.stop)

    def test_windows_cgo_toolchains_preserve_supported_duckdb_abi(self):
        amd64 = release.windows_toolchain("amd64")
        self.assertEqual(amd64["cc"], "gcc.exe")
        self.assertEqual(amd64["cxx"], "g++.exe")
        self.assertEqual(amd64["directory"], "mingw64")
        self.assertEqual(amd64["sha256"],
                         "88868d745b807f083a117ff69348d8bc021ad7389aa503379dbed1866efcaeb9")
        self.assertEqual(amd64["url"],
                         "https://github.com/brechtsanders/winlibs_mingw/releases/download/"
                         "14.2.0posix-12.0.0-ucrt-r3/"
                         "winlibs-x86_64-posix-seh-gcc-14.2.0-mingw-w64ucrt-12.0.0-r3.zip")
        arm64 = release.windows_toolchain("arm64")
        self.assertEqual(arm64["cc"], "aarch64-w64-mingw32-clang.exe")
        self.assertEqual(arm64["cxx"], "aarch64-w64-mingw32-clang++.exe")
        self.assertEqual(arm64["directory"], "llvm-mingw-20250709-ucrt-x86_64")
        self.assertEqual(arm64["sha256"],
                         "82babcd6aae4dc3606e8e0471d816989c384b4bf86a139f184a8b4a1b2c2758d")
        for arch in ("386", "AMD64", "", "arm64;bad"):
            with self.subTest(arch=arch), self.assertRaises(ValueError):
                release.windows_toolchain(arch)
        amd64["cc"] = "clang.exe"
        self.assertEqual(release.windows_toolchain("amd64")["cc"], "gcc.exe")

    def test_toolchain_cli_is_shared_with_powershell_setup(self):
        for arch, expected in (("amd64", "gcc.exe"), ("arm64", "aarch64-w64-mingw32-clang.exe")):
            output = io.StringIO()
            with patch("sys.argv", ["release.py", "toolchain", "--arch", arch]), \
                    contextlib.redirect_stdout(output):
                release.main()
            self.assertEqual(json.loads(output.getvalue())["cc"], expected)
        setup = (ROOT / "scripts/release-windows.ps1").read_text()
        self.assertIn("python scripts/release.py toolchain --arch $env:RELEASE_ARCH", setup)
        self.assertIn("if ($LASTEXITCODE -ne 0) { throw 'Target toolchain selection failed' }", setup)
        self.assertIn("Join-Path $bin $toolchain.cc", setup)
        self.assertIn("Join-Path $bin $toolchain.cxx", setup)
        self.assertIn("Get-FileHash $archive -Algorithm SHA256", setup)
        self.assertIn("& $cc -static $probe -o $output", setup)
        self.assertIn("static __thread int tls_probe", setup)
        self.assertIn("python scripts/release.py cpu $output --format pe --arch $env:RELEASE_ARCH", setup)

    def test_windows_static_runtime_linkage_and_pe_import_allowlist(self):
        access_reader = (ROOT / "internal/accessreader/reader.go").read_text(encoding="utf-8")
        self.assertIn("#cgo windows LDFLAGS: -static -lm", access_reader)
        win_taskfile = (ROOT / "build/windows/Taskfile.yml").read_text(encoding="utf-8")
        self.assertEqual(win_taskfile.count("-extldflags=-static"), 2)
        cross_docker = (ROOT / "build/docker/Dockerfile.cross").read_text(encoding="utf-8")
        self.assertIn('-H windowsgui -extldflags=-static', cross_docker)

        # Standard inbox Win32 + UCRT API-set DLLs pass on both amd64 and arm64.
        for arch in ("amd64", "arm64"):
            clean = pe(arch, imports=(
                "KERNEL32.dll", "USER32.dll", "ole32.dll", "msvcrt.dll", "ucrtbase.dll",
                "api-ms-win-crt-runtime-l1-1-0.dll", "ext-ms-win-ntuser-window-l1-1-0.dll",
            ))
            release.cpu(clean, "pe", arch)

        # MinGW / LLVM / VC++ redistributable / third-party runtime DLLs fail closed.
        for forbidden in (
            "libwinpthread-1.dll", "libgcc_s_seh-1.dll", "libstdc++-6.dll",
            "libc++.dll", "libunwind.dll", "vcruntime140.dll", "sqlite3.dll", "duckdb.dll",
        ):
            with self.subTest(forbidden=forbidden):
                pattern = rf"Non-system Windows DLL.*{re.escape(forbidden)}"
                leaked = pe("amd64", imports=("KERNEL32.dll", forbidden))
                with self.assertRaisesRegex(ValueError, pattern):
                    release.cpu(leaked, "pe", "amd64")
                delayed = pe("arm64", imports=("KERNEL32.dll",), delay_imports=(forbidden,))
                with self.assertRaisesRegex(ValueError, pattern):
                    release.cpu(delayed, "pe", "arm64")

        # Packaging rejects NSIS payload or standalone EXE leaking libwinpthread-1.dll.
        (self.root / "bin").mkdir()
        (self.root / "bin/VPro.exe").write_bytes(
            pe("amd64", imports=("KERNEL32.dll", "libwinpthread-1.dll"))
        )
        (self.root / "bin/VPro-amd64-installer.exe").write_bytes(b"NSIS")
        with patch.dict(os.environ, {"RELEASE_ARCH": "amd64"}):
            with self.assertRaisesRegex(ValueError, "libwinpthread-1.dll"):
                release.package(self.root, "windows")
        self.assertEqual(list((self.root / "dist-artifacts").iterdir()), [])

        # Corrupted / truncated import directory RVA fails closed.
        corrupted = bytearray(pe("amd64"))
        struct.pack_into("<II", corrupted, 128 + 24 + 112 + 8, 0x9999, 40)
        with self.assertRaisesRegex(ValueError, "Unmapped PE RVA"):
            release.cpu(bytes(corrupted), "pe", "amd64")

    def test_windows_target_goarch_reaches_dependencies_without_cross_building_cli(self):
        workflow = (ROOT / ".github/workflows/release.yml").read_text()
        windows = workflow.split("  build-windows:", 1)[1].split("  build-macos:", 1)[0]
        job_environment = windows.split("    steps:", 1)[0]
        self.assertNotIn("GOARCH:", job_environment)
        cli = windows.split("      - name: Install Wails CLI", 1)[1].split("      - name:", 1)[0]
        self.assertIn("CGO_ENABLED: '0'", cli)
        self.assertNotIn("GOARCH:", cli)
        build = windows.split("      - name: Build binary and NSIS installer with cgo", 1)[1]
        build = build.split("      - uses:", 1)[0]
        self.assertIn("        env:\n          GOARCH: ${{ matrix.arch }}\n", build)
        self.assertIn('task windows:create:nsis:installer "ARCH=$env:RELEASE_ARCH" CGO_ENABLED=1', build)

    def test_safe_versions(self):
        for raw, expected in (("v2.3.4", ("2.3.4", "2.3.4")),
                              ("0.0.0", ("0.0.0", "0.0.0")),
                              ("65535.1.2", ("65535.1.2", "65535.1.2")),
                              ("v1.2.3-rc.1+build-7", ("1.2.3-rc.1+build-7", "1.2.3"))):
            with self.subTest(raw=raw):
                self.assertEqual(release.version(raw), expected)
        for raw in ("", " v1.2.3", "v1.2.3\n", "01.2.3", "1.2.3-01", "1.2", "65536.0.0",
                    "1.2.3;echo x", '1.2.3"$(touch x)', "1.2.3/other", "1.2.3-",
                    "1.2.3+a..b", "1.2.3-\u00e9", "1.2.3\ninjected=true", "v" + "9" * 129):
            with self.subTest(raw=raw), self.assertRaises(ValueError):
                release.version(raw)

    def test_version_outputs_have_no_shell_interpolation(self):
        output = self.root / "outputs"
        with patch.dict(os.environ, {"GITHUB_OUTPUT": str(output)}), \
                patch("sys.argv", ["release.py", "version"]):
            release.main()
        self.assertEqual(output.read_text(), "version=2.3.4\ntag=v2.3.4\n")

    def test_cpu_witnesses_and_wrong_architectures(self):
        for arch in ("amd64", "arm64"):
            for kind, data in (("pe", pe(arch)), ("elf", elf(arch))):
                release.cpu(data, kind, arch)
                with self.assertRaises(ValueError):
                    release.cpu(data, kind, "arm64" if arch == "amd64" else "amd64")
                for invalid in (b"", data[:20]):
                    with self.assertRaises(ValueError):
                        release.cpu(invalid, kind, arch)
        release.cpu(universal(), "macho", "universal")
        for offset, fmt, value in ((4, ">I", 1), (28, ">I", 0x01000007),
                                   (36, ">I", 128), (132, "<I", 0x0100000C),
                                   (16, ">I", 1000)):
            broken = bytearray(universal())
            struct.pack_into(fmt, broken, offset, value)
            with self.subTest(offset=offset), self.assertRaises(ValueError):
                release.cpu(bytes(broken), "macho", "universal")
        with self.assertRaises(ValueError):
            release.cpu(universal()[128:192], "macho", "universal")

    def test_metadata_is_ci_only_and_versions_reach_all_consumers(self):
        for name in release.METADATA:
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, path)
        before = {name: (self.root / name).read_bytes() for name in release.METADATA}
        with patch.dict(os.environ, {"CI": "false", "GITHUB_ACTIONS": "false"}):
            with self.assertRaisesRegex(ValueError, "CI-only"):
                release.prepare_metadata(self.root)
        self.assertEqual(before, {name: (self.root / name).read_bytes() for name in release.METADATA})
        with patch.dict(os.environ, {"CI": "true", "GITHUB_ACTIONS": "true",
                                    "GITHUB_WORKSPACE": str(self.root),
                                    "RELEASE_VERSION": "v2.3.4-rc.1+build.7"}), \
                patch.object(release, "run") as command:
            release.prepare_metadata(self.root)
            command.assert_called_once_with("git", "diff", "--exit-code", "HEAD", "--",
                                            *release.METADATA, cwd=self.root)
        self.assertIn('version: "2.3.4"', (self.root / "build/config.yml").read_text())
        self.assertIn('version: "2.3.4-rc.1+build.7"',
                      (self.root / "build/linux/nfpm/nfpm.yaml").read_text())
        self.assertIn('INFO_PRODUCTVERSION "2.3.4"',
                      (self.root / "build/windows/nsis/wails_tools.nsh").read_text(encoding="utf-8"))
        info = json.loads((self.root / "build/windows/info.json").read_text(encoding="utf-8"))
        self.assertEqual(info["fixed"]["file_version"], "2.3.4")
        self.assertEqual(info["info"]["0000"]["ProductVersion"], "2.3.4-rc.1+build.7")
        info = plistlib.loads((self.root / "build/darwin/Info.plist").read_bytes())
        self.assertEqual(info["CFBundleVersion"], "2.3.4")
        self.assertEqual(info["CFBundleShortVersionString"], "2.3.4")
        self.assertEqual(info["CFBundleExecutable"], "VPro")

    def test_metadata_fails_before_writing_on_drift(self):
        with patch.dict(os.environ, {"CI": "true", "GITHUB_ACTIONS": "true",
                                    "GITHUB_WORKSPACE": str(self.root)}), \
                patch.object(release, "run", side_effect=subprocess.CalledProcessError(1, "git")):
            with self.assertRaises(subprocess.CalledProcessError):
                release.prepare_metadata(self.root)
        self.assertEqual(list(self.root.iterdir()), [])
        with self.assertRaises(ValueError):
            release.replace_once('version: "1"\nversion: "2"', r'^version: .*$', 'version: "3"')

    def test_deb_version_cpu_and_container_validation(self):
        path = self.root / "app.deb"
        path.write_bytes(deb("arm64"))
        release.validate_deb(path, "arm64", "2.3.4")
        for arch, full in (("amd64", "2.3.4"), ("arm64", "0.1.0")):
            with self.assertRaises(ValueError):
                release.validate_deb(path, arch, full)
        path.write_bytes(deb("arm64", "2.3.4~rc.1+build-7"))
        release.validate_deb(path, "arm64", "2.3.4-rc.1+build-7")
        for data in (b"", deb("arm64")[:-4], deb("arm64") + b"junk"):
            with self.assertRaises((ValueError, UnicodeError)):
                release.deb_members(data)

    def test_manifest_exact_contract_and_download_verification(self):
        asset_fixture(self.root)
        self.assertEqual(release.REQUIRED, set(EXPECTED))
        with working_directory(self.root):
            release.manifest(self.root)
        content = (self.root / "SHA256SUMS.txt").read_text()
        self.assertEqual(list(release.parse_manifest(content)), list(EXPECTED))
        for line, name in zip(content.splitlines(), EXPECTED):
            self.assertEqual(line, hashlib.sha256((self.root / name).read_bytes()).hexdigest() + "  " + name)
        paths = (self.root / "release-assets.txt").read_text().splitlines()
        self.assertEqual({Path(path).name for path in paths}, set(EXPECTED) | {"SHA256SUMS.txt"})
        download = self.root / "download"
        download.mkdir()
        for name in (*EXPECTED, "SHA256SUMS.txt"):
            shutil.copyfile(self.root / name, download / name)
        release.verify(download, self.root / "SHA256SUMS.txt")
        (download / EXPECTED[0]).write_bytes(b"tampered")
        with self.assertRaisesRegex(ValueError, "SHA256 mismatch"):
            release.verify(download, self.root / "SHA256SUMS.txt")

    def test_missing_extra_empty_and_stale_witnesses_block_manifest(self):
        for problem in ("missing", "extra", "empty", "commit", "version", "cpu", "hash"):
            with self.subTest(problem=problem), tempfile.TemporaryDirectory() as temporary:
                directory = Path(temporary)
                asset_fixture(directory)
                if problem == "missing":
                    (directory / EXPECTED[0]).unlink()
                elif problem == "extra":
                    (directory / "unwanted.zip").write_bytes(b"x")
                elif problem == "empty":
                    (directory / EXPECTED[0]).write_bytes(b"")
                else:
                    path = directory / "release-witness-windows-arm64.json"
                    witness = json.loads(path.read_text())
                    key = {"cpu": "architectures", "hash": "sha256"}.get(problem, problem)
                    witness[key] = "wrong"
                    path.write_text(json.dumps(witness))
                with self.assertRaises(ValueError), working_directory(directory):
                    release.manifest(directory)
                self.assertFalse((directory / "SHA256SUMS.txt").exists())
                self.assertFalse((directory / "release-assets.txt").exists())

    def test_forged_receipt_does_not_hide_wrong_portable_cpu(self):
        asset_fixture(self.root)
        path = self.root / "vpro-linux-arm64.AppImage"
        path.write_bytes(elf("amd64", True))
        receipt = self.root / "release-witness-linux-arm64.json"
        witness = json.loads(receipt.read_text())
        witness["sha256"][path.name] = hashlib.sha256(path.read_bytes()).hexdigest()
        receipt.write_text(json.dumps(witness))
        with self.assertRaisesRegex(ValueError, "ELF CPU mismatch"):
            release.manifest(self.root)

    def test_release_tag_source_absent_lightweight_and_annotated(self):
        commit = "a" * 40
        tag = "refs/tags/v2.3.4"
        self.assertEqual(release.validate_source_tag("", "v2.3.4", commit), "absent")
        self.assertEqual(release.validate_source_tag(f"{commit}\t{tag}\n", "v2.3.4", commit), "matching")
        annotated = f"{'b' * 40}\t{tag}\n{commit}\t{tag}^{{}}\n"
        self.assertEqual(release.validate_source_tag(annotated, "v2.3.4", commit), "matching")
        for output in (f"{'c' * 40}\t{tag}\n",
                       f"{commit}\t{tag}\n{'c' * 40}\t{tag}^{{}}\n"):
            with self.assertRaisesRegex(ValueError, "different source"):
                release.validate_source_tag(output, "v2.3.4", commit)

    def test_release_tag_source_malformed_or_ambiguous_refuses(self):
        commit = "a" * 40
        tag = "refs/tags/v2.3.4"
        for output in ("bad", f"invalid\t{tag}\n", f"{commit}\trefs/tags/v1.0.0\n",
                       f"{commit}\t{tag}^{{}}\n",
                       f"{commit}\t{tag}\n{commit}\t{tag}\n"):
            with self.assertRaises(ValueError):
                release.validate_source_tag(output, "v2.3.4", commit)
        with self.assertRaisesRegex(ValueError, "checkout commit"):
            release.validate_source_tag("", "v2.3.4", "unknown")

    def test_release_tag_source_lookup_failure_and_version_mismatch_refuse(self):
        with patch.dict(os.environ, {"RELEASE_TAG": "v2.3.4"}), \
                patch.object(release.subprocess, "run",
                             side_effect=subprocess.CalledProcessError(1, "git")):
            with self.assertRaises(subprocess.CalledProcessError):
                release.check_source_tag()
        with patch.dict(os.environ, {"RELEASE_TAG": "v2.3.3"}), \
                patch.object(release.subprocess, "run") as lookup:
            with self.assertRaisesRegex(ValueError, "validated version"):
                release.check_source_tag()
            lookup.assert_not_called()
        with patch.dict(os.environ, {"RELEASE_TAG": "v2.3.4", "GITHUB_SHA": "a" * 40}), \
                patch.object(release.subprocess, "run",
                             return_value=subprocess.CompletedProcess([], 0, stdout="", stderr="")) as lookup, \
                contextlib.redirect_stdout(io.StringIO()):
            release.check_source_tag()
            self.assertEqual(lookup.call_args.args[0],
                             ["git", "ls-remote", "--tags", "origin",
                              "refs/tags/v2.3.4", "refs/tags/v2.3.4^{}"])
            self.assertTrue(lookup.call_args.kwargs["check"])

    def test_strict_manifest_and_remote_assets(self):
        content = "".join("a" * 64 + "  " + name + "\n" for name in EXPECTED)
        for broken in (content + content.splitlines()[0] + "\n",
                       content.replace(EXPECTED[0], EXPECTED[1]),
                       content.replace(EXPECTED[0], "../secret"),
                       content.replace("a" * 64, "not-a-hash", 1)):
            with self.assertRaises(ValueError):
                release.parse_manifest(broken)
        path = self.root / "remote.json"
        names = [*EXPECTED, "SHA256SUMS.txt"]
        path.write_text(json.dumps({"assets": [{"name": name} for name in names]}))
        release.remote(path)
        for bad in (names[:-1], names + ["unknown"], names[:-1] + [names[0]]):
            path.write_text(json.dumps({"assets": [{"name": name} for name in bad]}))
            with self.assertRaises(ValueError):
                release.remote(path)

    def test_windows_inspects_nsis_payload_not_launcher_cpu(self):
        (self.root / "bin").mkdir()
        (self.root / "bin/VPro.exe").write_bytes(pe("arm64"))
        (self.root / "bin/VPro-arm64-installer.exe").write_bytes(b"x86 NSIS launcher")

        def extract(*args, **kwargs):
            self.assertEqual(args[:3], ("7z", "x", "-y"))
            self.assertTrue(args[3].endswith("VPro-arm64-installer.exe"))
            (Path(args[4][2:]) / "VPro.exe").write_bytes(pe("arm64"))

        with patch.dict(os.environ, {"RELEASE_ARCH": "arm64"}), patch.object(release, "run", side_effect=extract):
            release.package(self.root, "windows")
        self.assertEqual({path.name for path in (self.root / "dist-artifacts").iterdir()},
                         {"vpro-windows-arm64.exe", "vpro-windows-arm64.zip",
                          "vpro-windows-arm64-installer.exe", "release-witness-windows-arm64.json"})

    def test_packaging_command_failure_never_creates_witness(self):
        (self.root / "bin").mkdir()
        (self.root / "bin/VPro.exe").write_bytes(pe("amd64"))
        (self.root / "bin/VPro-amd64-installer.exe").write_bytes(b"NSIS")
        with patch.dict(os.environ, {"RELEASE_ARCH": "amd64"}), \
                patch.object(release, "run", side_effect=subprocess.CalledProcessError(1, "7z")):
            with self.assertRaises(subprocess.CalledProcessError):
                release.package(self.root, "windows")
        self.assertEqual(list((self.root / "dist-artifacts").iterdir()), [])

    def test_linux_native_packaging_requires_all_formats_and_extracts_payload(self):
        (self.root / "bin").mkdir()
        (self.root / "bin/VPro").write_bytes(elf("arm64"))
        (self.root / "bin/VPro-aarch64.AppImage").write_bytes(elf("arm64", True))
        (self.root / "bin/VPro.deb").write_bytes(deb("arm64"))

        def extract(*args, **kwargs):
            self.assertEqual(args[1], "--appimage-extract")
            payload = Path(kwargs["cwd"]) / "squashfs-root/usr/bin/VPro"
            payload.parent.mkdir(parents=True)
            payload.write_bytes(elf("arm64"))

        with patch.dict(os.environ, {"RELEASE_ARCH": "arm64"}), \
                patch.object(release, "run", side_effect=extract):
            release.package(self.root, "linux")
        self.assertEqual({path.name for path in (self.root / "dist-artifacts").iterdir()},
                         {"vpro-linux-arm64.AppImage", "vpro-linux-arm64.deb",
                          "vpro-linux-arm64.tar.gz", "release-witness-linux-arm64.json"})

    def test_linux_extract_error_is_fatal(self):
        (self.root / "bin").mkdir()
        (self.root / "bin/VPro").write_bytes(elf("amd64"))
        (self.root / "bin/VPro-x86_64.AppImage").write_bytes(elf("amd64", True))
        with patch.dict(os.environ, {"RELEASE_ARCH": "amd64"}), \
                patch.object(release, "run", side_effect=subprocess.CalledProcessError(1, "AppImage")):
            with self.assertRaises(subprocess.CalledProcessError):
                release.package(self.root, "linux")
        self.assertEqual(list((self.root / "dist-artifacts").iterdir()), [])

    def test_missing_or_ambiguous_linux_output_is_fatal(self):
        (self.root / "bin").mkdir()
        with self.assertRaisesRegex(ValueError, "exactly one"):
            release.unique_output(self.root, ".deb")
        for name in ("one.deb", "two.deb"):
            (self.root / "bin" / name).write_bytes(b"x")
        with self.assertRaisesRegex(ValueError, "exactly one"):
            release.unique_output(self.root, ".deb")

    def test_mac_dmg_payload_and_detach_even_on_validation_failure(self):
        bundle = self.root / "bin/VPro.app/Contents"
        (bundle / "MacOS").mkdir(parents=True)
        (bundle / "MacOS/VPro").write_bytes(universal())
        (bundle / "Info.plist").write_bytes(bundle_info())
        (self.root / "bin/VPro.dmg").write_bytes(b"dmg")

        def mount(*args, **kwargs):
            if args[1] == "attach":
                directory = Path(args[args.index("-mountpoint") + 1])
                (directory / "VPro.app/Contents/MacOS").mkdir(parents=True)
                (directory / "VPro.app/Contents/MacOS/VPro").write_bytes(pe("amd64"))

        with patch.object(release, "run", side_effect=mount) as commands:
            with self.assertRaisesRegex(ValueError, "fat Mach-O"):
                release.package(self.root, "macos")
        self.assertEqual(commands.call_args_list[-1].args[:2], ("hdiutil", "detach"))
        self.assertEqual(list((self.root / "dist-artifacts").iterdir()), [])

    def test_mac_success_validates_both_archives_and_detach_failure_is_fatal(self):
        bundle = self.root / "bin/VPro.app/Contents"
        (bundle / "MacOS").mkdir(parents=True)
        (bundle / "MacOS/VPro").write_bytes(universal())
        (bundle / "Info.plist").write_bytes(bundle_info())
        (self.root / "bin/VPro.dmg").write_bytes(b"dmg")

        def tools(*args, **kwargs):
            if args[0:2] == ("hdiutil", "attach"):
                mount = Path(args[args.index("-mountpoint") + 1])
                shutil.copytree(bundle.parent, mount / "VPro.app")
            elif args[0] == "ditto":
                with zipfile.ZipFile(args[-1], "w") as archive:
                    archive.writestr("VPro.app/Contents/MacOS/VPro", universal())
                    archive.writestr("VPro.app/Contents/Info.plist", bundle_info())

        with patch.object(release, "run", side_effect=tools):
            release.package(self.root, "macos")
        self.assertEqual({path.name for path in (self.root / "dist-artifacts").iterdir()},
                         {"vpro-macos-universal.dmg", "vpro-macos-universal.zip",
                          "release-witness-macos-universal.json"})
        for path in (self.root / "dist-artifacts").iterdir():
            path.unlink()
        (self.root / "dist-artifacts").rmdir()

        def detach_failure(*args, **kwargs):
            if args[:2] == ("hdiutil", "detach"):
                raise subprocess.CalledProcessError(1, "hdiutil")
            tools(*args, **kwargs)

        with patch.object(release, "run", side_effect=detach_failure):
            with self.assertRaises(subprocess.CalledProcessError):
                release.package(self.root, "macos")
        self.assertEqual(list((self.root / "dist-artifacts").iterdir()), [])

    def test_release_manifest_generation_channel_merging_and_validation(self):
        asset_fixture(self.root)
        with working_directory(self.root):
            release.manifest(self.root)
        stable_path = self.root / "release-manifest.json"
        doc = release.release_manifest(
            self.root,
            stable_path,
            published_at="2026-11-12T12:00:00Z",
            root=ROOT,
        )
        self.assertEqual(doc["schemaVersion"], 1)
        self.assertEqual(doc["repository"], "Boostao/vpro_wails")
        self.assertEqual(doc["defaultChannel"], "stable")
        self.assertIsNone(doc["channels"]["preview"])
        stable = doc["channels"]["stable"]
        self.assertEqual(stable["tag"], "v2.3.4")
        self.assertEqual(stable["version"], "2.3.4")
        self.assertIs(stable["prerelease"], False)
        self.assertIs(stable["unsignedNotice"], True)
        self.assertEqual(
            stable["sha256sumsUrl"],
            "https://github.com/Boostao/vpro_wails/releases/download/v2.3.4/SHA256SUMS.txt",
        )
        self.assertIn("BECdb v13", stable["standards"]["becVersion"])
        self.assertIn("BCPlantTaxonomy v19", stable["standards"]["taxonomyVersion"])
        self.assertEqual(stable["standards"]["lmh25Edition"], "LMH25 (2023)")
        self.assertEqual([a["name"] for a in stable["assets"]], sorted([*EXPECTED, "SHA256SUMS.txt"]))
        for entry in stable["assets"]:
            expected_bytes = (self.root / entry["name"]).read_bytes()
            self.assertEqual(entry["sizeBytes"], len(expected_bytes))
            self.assertEqual(entry["sha256"], hashlib.sha256(expected_bytes).hexdigest())

        # Verify remote & downloaded release verification with release-manifest.json attached.
        remote_json = self.root / "remote-with-manifest.json"
        remote_names = [*EXPECTED, "SHA256SUMS.txt", "release-manifest.json"]
        remote_json.write_text(json.dumps({"assets": [{"name": n} for n in remote_names]}))
        release.remote(remote_json, require_release_manifest=True)
        remote_without = self.root / "remote-without-manifest.json"
        remote_without.write_text(json.dumps({"assets": [{"name": n} for n in remote_names[:-1]]}))
        with self.assertRaisesRegex(ValueError, "missing release-manifest.json"):
            release.remote(remote_without, require_release_manifest=True)

        downloaded = self.root / "downloaded-with-manifest"
        downloaded.mkdir()
        for name in remote_names:
            shutil.copyfile(self.root / name, downloaded / name)
        release.verify(downloaded, self.root / "SHA256SUMS.txt", release_manifest_path=stable_path)
        (downloaded / "release-manifest.json").write_text("{}", encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "differs from validated manifest"):
            release.verify(downloaded, self.root / "SHA256SUMS.txt", release_manifest_path=stable_path)

        # Merge a preview RC release on top of the existing stable release-manifest.json via CLI.
        rc_dir = self.root / "rc-assets"
        rc_dir.mkdir()
        with patch.dict(os.environ, {"RELEASE_VERSION": "v2.4.0-rc.1", "RELEASE_PRERELEASE": "true"}):
            for arch in ("amd64", "arm64"):
                (rc_dir / f"vpro-windows-{arch}.exe").write_bytes(pe(arch))
                (rc_dir / f"vpro-windows-{arch}-installer.exe").write_bytes(b"rc installer")
                with zipfile.ZipFile(rc_dir / f"vpro-windows-{arch}.zip", "w") as archive:
                    archive.writestr("VPro.exe", pe(arch))
                (rc_dir / f"vpro-linux-{arch}.AppImage").write_bytes(elf(arch, True))
                (rc_dir / f"vpro-linux-{arch}.deb").write_bytes(deb(arch, "2.4.0~rc.1"))
                (rc_dir / f"vpro-linux-{arch}.tar.gz").write_bytes(tar_bytes([("VPro", elf(arch))]))
            (rc_dir / "vpro-macos-universal.dmg").write_bytes(b"rc DMG")
            with zipfile.ZipFile(rc_dir / "vpro-macos-universal.zip", "w") as archive:
                archive.writestr("VPro.app/Contents/MacOS/VPro", universal())
                archive.writestr("VPro.app/Contents/Info.plist",
                                 plistlib.dumps({"CFBundleVersion": "2.4.0", "CFBundleShortVersionString": "2.4.0"}))
            for group in ("windows-amd64", "windows-arm64", "macos-universal", "linux-amd64", "linux-arm64"):
                names = [name for name in EXPECTED if name.startswith("vpro-" + group)]
                witness = {"schema": 1, "group": group, "version": "2.4.0-rc.1", "commit": "abc123",
                           "architectures": ["amd64", "arm64"] if group == "macos-universal" else [group.split("-")[1]],
                           "sha256": {name: hashlib.sha256((rc_dir / name).read_bytes()).hexdigest() for name in names}}
                (rc_dir / f"release-witness-{group}.json").write_text(json.dumps(witness), encoding="utf-8")
            with working_directory(rc_dir):
                release.manifest(rc_dir)
            with self.assertRaisesRegex(ValueError, "cannot publish to stable"):
                release.release_manifest(rc_dir, self.root / "bad.json", channel="stable")
            merged_path = self.root / "merged-manifest.json"
            with patch("sys.argv", [
                "release.py", "release-manifest", str(rc_dir),
                "--output", str(merged_path),
                "--base-manifest", str(stable_path),
                "--published-at", "2026-11-13T08:30:00Z",
            ]):
                release.main()
        merged = json.loads(merged_path.read_text(encoding="utf-8"))
        release.validate_release_manifest(merged)
        self.assertEqual(merged["channels"]["stable"]["tag"], "v2.3.4")
        self.assertIs(merged["channels"]["stable"]["prerelease"], False)
        self.assertEqual(merged["channels"]["preview"]["tag"], "v2.4.0-rc.1")
        self.assertIs(merged["channels"]["preview"]["prerelease"], True)

        # Repository site/release-manifest.json must satisfy strict schema validation.
        repo_manifest = json.loads((ROOT / "site/release-manifest.json").read_text(encoding="utf-8"))
        release.validate_release_manifest(repo_manifest)

    def test_sync_release_manifest_hydrates_non_draft_channels_and_skips_drafts(self):
        asset_fixture(self.root)
        with working_directory(self.root):
            release.manifest(self.root)
        stable_manifest = self.root / "stable-release-manifest.json"
        release.release_manifest(
            self.root,
            stable_manifest,
            published_at="2026-11-13T17:00:00Z",
            root=ROOT,
        )

        rc_dir = self.root / "sync-rc"
        rc_dir.mkdir()
        with patch.dict(os.environ, {"RELEASE_VERSION": "v2.4.0-rc.1", "RELEASE_PRERELEASE": "true"}):
            for arch in ("amd64", "arm64"):
                (rc_dir / f"vpro-windows-{arch}.exe").write_bytes(pe(arch))
                (rc_dir / f"vpro-windows-{arch}-installer.exe").write_bytes(b"rc installer")
                with zipfile.ZipFile(rc_dir / f"vpro-windows-{arch}.zip", "w") as archive:
                    archive.writestr("VPro.exe", pe(arch))
                (rc_dir / f"vpro-linux-{arch}.AppImage").write_bytes(elf(arch, True))
                (rc_dir / f"vpro-linux-{arch}.deb").write_bytes(deb(arch, "2.4.0~rc.1"))
                (rc_dir / f"vpro-linux-{arch}.tar.gz").write_bytes(tar_bytes([("VPro", elf(arch))]))
            (rc_dir / "vpro-macos-universal.dmg").write_bytes(b"rc DMG")
            with zipfile.ZipFile(rc_dir / "vpro-macos-universal.zip", "w") as archive:
                archive.writestr("VPro.app/Contents/MacOS/VPro", universal())
                archive.writestr("VPro.app/Contents/Info.plist",
                                 plistlib.dumps({"CFBundleVersion": "2.4.0", "CFBundleShortVersionString": "2.4.0"}))
            for group in ("windows-amd64", "windows-arm64", "macos-universal", "linux-amd64", "linux-arm64"):
                names = [name for name in EXPECTED if name.startswith("vpro-" + group)]
                witness = {"schema": 1, "group": group, "version": "2.4.0-rc.1", "commit": "abc123",
                           "architectures": ["amd64", "arm64"] if group == "macos-universal" else [group.split("-")[1]],
                           "sha256": {name: hashlib.sha256((rc_dir / name).read_bytes()).hexdigest() for name in names}}
                (rc_dir / f"release-witness-{group}.json").write_text(json.dumps(witness), encoding="utf-8")
            with working_directory(rc_dir):
                release.manifest(rc_dir)
            rc_manifest = self.root / "rc-release-manifest.json"
            release.release_manifest(
                rc_dir,
                rc_manifest,
                published_at="2026-11-13T18:00:00Z",
                root=ROOT,
            )

        complete_assets = [{"name": n} for n in (*EXPECTED, "SHA256SUMS.txt", "release-manifest.json")]
        releases_feed = self.root / "releases-feed.json"
        releases_feed.write_text(json.dumps([
            {"tag_name": "v9.9.9-draft", "draft": True, "prerelease": False, "assets": complete_assets},
            {"tag_name": "v2.5.0-incomplete", "draft": False, "prerelease": False, "assets": [{"name": "SHA256SUMS.txt"}]},
            {"tag_name": "v2.4.0-rc.1", "draft": False, "prerelease": True, "assets": complete_assets},
            {"tag_name": "v2.3.4", "draft": False, "prerelease": False, "assets": complete_assets},
        ]), encoding="utf-8")

        manifests_by_tag = {
            "v2.3.4": stable_manifest.read_text(encoding="utf-8"),
            "v2.4.0-rc.1": rc_manifest.read_text(encoding="utf-8"),
        }
        synced_out = self.root / "synced-release-manifest.json"
        synced = release.sync_release_manifest(
            synced_out,
            base_manifest=ROOT / "site/release-manifest.json",
            releases_json=releases_feed,
            asset_loader=lambda tag, _: manifests_by_tag[tag],
        )
        self.assertEqual(synced["channels"]["stable"]["tag"], "v2.3.4")
        self.assertEqual(synced["channels"]["preview"]["tag"], "v2.4.0-rc.1")

        # Empty/draft-only feed clears channels cleanly instead of leaving dead download URLs.
        draft_only_feed = self.root / "draft-only.json"
        draft_only_feed.write_text(json.dumps([
            {"tag_name": "v0.1.0-demo", "draft": True, "prerelease": False, "assets": complete_assets},
        ]), encoding="utf-8")
        cleared_out = self.root / "cleared-release-manifest.json"
        cleared = release.sync_release_manifest(
            cleared_out,
            base_manifest=synced_out,
            releases_json=draft_only_feed,
            asset_loader=lambda tag, _: manifests_by_tag[tag],
        )
        self.assertIsNone(cleared["channels"]["stable"])
        self.assertIsNone(cleared["channels"]["preview"])

    def test_workflow_contract(self):
        workflow = (ROOT / ".github/workflows/release.yml").read_text()
        self.assertNotIn("|| true", workflow)
        self.assertNotIn("continue-on-error", workflow)
        self.assertNotIn("darwin:package:dmg", workflow)
        self.assertNotIn("macos-14", workflow)
        self.assertIn("runs-on: macos-15-intel", workflow)
        self.assertNotIn("go-version: '1.25'", workflow)
        self.assertEqual(workflow.count("go-version: '1.26'"), 3)
        self.assertIn("task darwin:package:universal", workflow)
        self.assertIn("task darwin:create:dmg", workflow)
        self.assertIn("runner: ubuntu-24.04-arm", workflow)
        self.assertIn("windows:create:nsis:installer", workflow)
        self.assertIn('CGO_ENABLED=1', workflow)
        self.assertIn("task linux:create:appimage linux:create:deb", workflow)
        self.assertNotIn("${{ inputs.version }}", workflow.split("run:", 1)[-1])
        self.assertEqual(workflow.count("contents: write"), 1)
        self.assertIn("contents: read", workflow)
        self.assertIn("python scripts/release.py sync-release-manifest", workflow)
        self.assertIn("python scripts/release.py manifest release-assets", workflow)
        self.assertIn("python scripts/release.py release-manifest release-assets --output release-assets/release-manifest.json", workflow)
        self.assertIn("python scripts/release.py remote release-remote.json --require-release-manifest", workflow)
        self.assertIn("python scripts/release.py verify release-downloaded --manifest release-assets/SHA256SUMS.txt --release-manifest release-assets/release-manifest.json", workflow)
        self.assertIn("name: github-pages", workflow)
        self.assertIn("actions/configure-pages@v5", workflow)
        pages = (ROOT / ".github/workflows/pages.yml").read_text()
        self.assertNotIn("enablement: true", pages)
        self.assertNotIn("path: 'docs'", pages)
        self.assertIn("node-version: '22'", pages)
        self.assertIn("python scripts/release.py sync-release-manifest", pages)
        self.assertIn("node scripts/build-docs-src.mjs && node site/build.mjs", pages)
        self.assertIn("path: 'site/dist'", pages)


if __name__ == "__main__":
    unittest.main()
