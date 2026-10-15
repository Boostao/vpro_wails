$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if ($env:GITHUB_ACTIONS -ne 'true' -or $env:CI -ne 'true') {
    throw 'Toolchain installation is restricted to GitHub Actions'
}
if ($env:RELEASE_ARCH -notin @('amd64', 'arm64')) {
    throw 'RELEASE_ARCH must be amd64 or arm64'
}

# amd64 needs GCC's C++ ABI for DuckDB; arm64 uses LLVM with SQLite.
$toolchainJson = python scripts/release.py toolchain --arch $env:RELEASE_ARCH
if ($LASTEXITCODE -ne 0) { throw 'Target toolchain selection failed' }
$toolchain = $toolchainJson | ConvertFrom-Json
$name = $toolchain.name
$expected = $toolchain.sha256
$archive = Join-Path $env:RUNNER_TEMP "$name.zip"
$destination = Join-Path $env:RUNNER_TEMP "release-$name"
Invoke-WebRequest -UseBasicParsing $toolchain.url -OutFile $archive
if ((Get-FileHash $archive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $expected) {
    throw 'Windows cgo toolchain SHA256 mismatch'
}
Expand-Archive -LiteralPath $archive -DestinationPath $destination
$bin = Join-Path $destination "$($toolchain.directory)\bin"
$cc = Join-Path $bin $toolchain.cc
$cxx = Join-Path $bin $toolchain.cxx
if (!(Test-Path -LiteralPath $cc -PathType Leaf) -or !(Test-Path -LiteralPath $cxx -PathType Leaf)) {
    throw 'Pinned toolchain is missing the requested compilers'
}
$env:PATH = "$bin;$env:PATH"
$probe = Join-Path $env:RUNNER_TEMP 'release-cgo-probe.c'
$output = Join-Path $env:RUNNER_TEMP 'release-cgo-probe.exe'
'int main(void) { return 0; }' | Set-Content -LiteralPath $probe -Encoding ascii
& $cc $probe -o $output
if ($LASTEXITCODE -ne 0) { throw 'Target C compiler/linker probe failed' }
python scripts/release.py cpu $output --format pe --arch $env:RELEASE_ARCH
if ($LASTEXITCODE -ne 0) { throw 'C compiler produced the wrong architecture' }

$bin >> $env:GITHUB_PATH
"CC=$cc" >> $env:GITHUB_ENV
"CXX=$cxx" >> $env:GITHUB_ENV
"CGO_ENABLED=1" >> $env:GITHUB_ENV
Remove-Item -LiteralPath $archive, $probe, $output
