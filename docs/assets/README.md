# README assets

Images referenced by [README.md](../../README.md) and [README.zh.md](../../README.zh.md).

| File | How it is produced |
|---|---|
| `screens/login.png` | `frontend/e2e/layout.spec.ts-snapshots/login-dark-laptop-linux.png`, copied verbatim (1280×800) |
| `screens/dashboard.png` | `dashboard-dark-laptop-linux.png` with the top 56 px (app header, which carries a test-only `Reconnecting…` badge) cropped: 1280×560 |
| `social-preview.png` | 1280×640 card composed from `screens/dashboard.png`; upload it manually under **Settings → Social preview** (GitHub has no API for this field) |

The Playwright baselines are captured against an empty database, so the dashboard
shows zeroed counters. They are placeholders until a seeded demo environment
produces marketing screenshots — when that lands, replace the files in
`screens/` and keep the same names so the READMEs do not change.

## Re-cropping the dashboard

```powershell
Add-Type -AssemblyName System.Drawing
$src = "frontend/e2e/layout.spec.ts-snapshots/dashboard-dark-laptop-linux.png"
$img = [System.Drawing.Image]::FromFile((Resolve-Path $src))
$h = 560
$bmp = New-Object System.Drawing.Bitmap 1280, $h
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.DrawImage($img, (New-Object System.Drawing.Rectangle 0, 0, 1280, $h),
                  (New-Object System.Drawing.Rectangle 0, 56, 1280, $h),
                  [System.Drawing.GraphicsUnit]::Pixel)
$bmp.Save((Join-Path (Resolve-Path docs/assets/screens) "dashboard.png"),
          [System.Drawing.Imaging.ImageFormat]::Png)
$g.Dispose(); $bmp.Dispose(); $img.Dispose()
```

Keep the width at 1280 px: GitHub scales README images to the column width, so a
wider file only adds download weight.
