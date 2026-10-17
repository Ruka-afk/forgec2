# fetch-wintun.ps1 - stage the Wintun runtimes that Windows agents embed.
#
# Wintun (https://www.wintun.net) is the signed layer-3 TUN driver for Windows.
# The Windows implant's tun_start carries this runtime embedded per
# architecture and drops wintun.dll next to itself at run time (see
# internal/payload/agent/tun_wintun*.go). The signed DLLs are third-party
# binaries under the permissive prebuilt-binary license shipped inside the
# archive, so they are fetched on demand and never committed (see .gitignore).
#
#   powershell -NoProfile -File scripts\fetch-wintun.ps1
#
# Only amd64 and arm64 are staged: those are the Windows architectures the
# implant generator supports (supportedArchByOS). Without staged runtimes the
# generator keeps the honest "no bundled Wintun" stub and no build breaks.
#
# Keep this script and scripts/fetch-wintun.sh in sync (same version + SHA256).
$ErrorActionPreference = "Stop"

$WintunVersion = "0.14.1"
$WintunZipSha256 = "07c256185d6ee3652e09fa55c0b673e2624b565e02c4b9091c79ca7d2f24ef51"
$Url = "https://www.wintun.net/builds/wintun-$WintunVersion.zip"

$root = Split-Path -Parent $PSScriptRoot
$assetsDir = Join-Path $root "internal\payload\agent\assets\wintun"
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("forgec2-wintun-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null

try {
    $zip = Join-Path $tmp "wintun-$WintunVersion.zip"
    Write-Host "Fetching wintun $WintunVersion ..."
    curl.exe -sSfL -o $zip $Url
    if ($LASTEXITCODE -ne 0) { throw "download failed: $Url" }

    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $zip).Hash.ToLowerInvariant()
    if ($actual -ne $WintunZipSha256) {
        throw "wintun archive sha256 mismatch: expected $WintunZipSha256, got $actual"
    }

    $extract = Join-Path $tmp "extract"
    Expand-Archive -LiteralPath $zip -DestinationPath $extract -Force

    foreach ($arch in @("amd64", "arm64")) {
        $src = Join-Path $extract "wintun\bin\$arch\wintun.dll"
        if (-not (Test-Path -LiteralPath $src)) { throw "archive is missing bin/$arch/wintun.dll" }
        $dstDir = Join-Path $assetsDir $arch
        New-Item -ItemType Directory -Force -Path $dstDir | Out-Null
        Copy-Item -LiteralPath $src -Destination (Join-Path $dstDir "wintun.dll") -Force
        Write-Host "staged $dstDir\wintun.dll"
    }

    $license = Join-Path $extract "wintun\LICENSE.txt"
    if (Test-Path -LiteralPath $license) {
        Copy-Item -LiteralPath $license -Destination (Join-Path $assetsDir "LICENSE.txt") -Force
    }
    Write-Host "Wintun $WintunVersion staged under internal/payload/agent/assets/wintun/"
} finally {
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $tmp
}
