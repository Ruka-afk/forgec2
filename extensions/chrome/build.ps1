param(
  [string]$OutputPath = "extensions\chrome\forgec2-chrome-c2.zip"
)

$extensionDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$targetDir = Join-Path -Path (Split-Path -Parent (Split-Path -Parent $extensionDir)) -ChildPath "frontend\public"
$zipTarget = Join-Path -Path $targetDir -ChildPath "forgec2-chrome-c2.zip"

Write-Host "[*] Building Chrome extension package..." -ForegroundColor Cyan

# Create icon if missing
$iconPath = Join-Path -Path $extensionDir -ChildPath "icon.png"
if (-not (Test-Path $iconPath)) {
  Write-Host "[*] Creating placeholder icon..." -ForegroundColor Yellow
  # Create a minimal 128x128 PNG (blue square with white shield)
  $bytes = [Convert]::FromBase64String("iVBORw0KGgoAAAANSUhEUgAAAIAAAACACAYAAADDPmHLAAAABHNCSVQICAgIfAhkiAAAAAlwSFlzAAALEwAACxMBAJqcGAAAAShJREFUeJzt3LENwjAQBdD/FYyAWIAZmIEBEC3SMQIjUMICjMAIjMAMjMBerChiKVJE51g+/akutkVJlnU+2U4SAAAAAAAAAAAAAAAA/8oY4yGlHGM8pJRjjIeUcoyxq5RyjLGrlHKMkVJKKccYKaWUcoyUUkopx0gppZRyjJRSlFKOkaKUcqSUUko5UkoppRwppZRSjpRSjikppZQjpZRSSimllFJKKaWUUkoppZRSSimllFJKKaWUUkoppZRSSimllFJKKaWUUkoppZRSSimllFJKKaWUUkoppZRSSimllFJKKaWUUkoppZRSSimllFJKKaWUUkoppZRSSimllFJKKaWUopQjpZRilHKMlFKMUo6UUopSjpRSilLKkVJKUUo5UkopSilHSilFKQcAAAAAAAAAAAAAAAAAAAAAAAAA4H98ARew0IqtAGDaAAAAAElFTkSuQmCC")
  [System.IO.File]::WriteAllBytes($iconPath, $icon)
}

# Build extension zip
if (Test-Path $zipTarget) {
  Remove-Item $zipTarget -Force
}

Compress-Archive -Path "$extensionDir\*" -DestinationPath $zipTarget -Force

if (Test-Path $zipTarget) {
  $size = (Get-Item $zipTarget).Length
  Write-Host "[+] Extension package created: $zipTarget ($([math]::Round($size/1KB)) KB)" -ForegroundColor Green
} else {
  Write-Host "[-] Failed to create extension package" -ForegroundColor Red
}