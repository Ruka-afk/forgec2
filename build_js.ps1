# ForgeC2 Asset Bundling Script
# Merge JS and CSS files to reduce HTTP requests and improve loading speed

param(
    [string]$Mode = "production",  # production or development
    [bool]$Minify = $true,         # enable minification (remove comments and whitespace)
    [switch]$SkipJS = $false,      # skip JS bundling
    [switch]$SkipCSS = $false      # skip CSS bundling
)

$JS_DIR = "internal\server\templates\static\js"
$CSS_DIR = "internal\server\templates\static\css"
$JS_OUTPUT_DIR = $JS_DIR
$CSS_OUTPUT_DIR = $CSS_DIR

# JS file grouping strategy
$JS_BUNDLES = @{
    "core.bundle.js" = @(
        "core.js",
        "shortcuts.js",
        "command-history.js",
        "layout.js",
        "notifications.js",
        "lazyload.js",
        "svg-icons.js"
    )
    "agents.bundle.js" = @(
        "agents.js",
        "agent-detail.js",
        "shell.js",
        "files.js"
    )
    "dashboard.bundle.js" = @(
        "dashboard.js",
        "topology.js"
    )
    "settings.bundle.js" = @(
        "settings.js"
    )
    "ops.bundle.js" = @(
        "credentials.js",
        "tasks.js",
        "audit.js"
    )
    "plugins.bundle.js" = @(
        "plugins.js",
        "bof.js"
    )
    "generate.bundle.js" = @(
        "generate.js"
    )
    "listeners.bundle.js" = @(
        "listeners.js"
    )
    "toolkit.bundle.js" = @(
        "toolkit.js",
        "lateral.js",
        "privesc.js",
        "scanner.js"
    )
    "comms.bundle.js" = @(
        "chat.js",
        "ai.js",
        "traffic.js"
    )
    "report.bundle.js" = @(
        "report.js",
        "timeline.js",
        "loot.js",
        "automation.js",
        "infrastructure.js"
    )
    "admin.bundle.js" = @(
        "users.js",
        "token.js",
        "templates.js",
        "pivoting.js",
        "screen.js",
        "bof_repo.js"
    )
    "search.bundle.js" = @(
        "search.js"
    )
}

# CSS file grouping strategy
$CSS_BUNDLES = @{
    "app.bundle.css" = @(
        "layout.css",
        "skeleton.css",
        "lazyload.css"
    )
}

# Simple JS minification function
function MinifyJS($content) {
    $result = ""
    $inSingleQuote = $false
    $inDoubleQuote = $false
    $inTemplateLiteral = $false
    $inTemplateExpr = $false
    $templateExprDepth = 0
    $inLineComment = $false
    $inBlockComment = $false
    $escapeNext = $false
    
    for ($i = 0; $i -lt $content.Length; $i++) {
        $char = $content[$i]
        $nextChar = if ($i + 1 -lt $content.Length) { $content[$i + 1] } else { $null }
        
        if ($escapeNext) {
            $result += $char
            $escapeNext = $false
            continue
        }
        
        if ($inBlockComment) {
            if ($char -eq '*' -and $nextChar -eq '/') {
                $inBlockComment = $false
                $i++
            }
            continue
        }
        
        if ($inLineComment) {
            if ($char -eq "`n" -or $char -eq "`r") {
                $inLineComment = $false
                $result += $char
            }
            continue
        }
        
        if ($inSingleQuote) {
            if ($char -eq "'") {
                $inSingleQuote = $false
            } elseif ($char -eq '\') {
                $escapeNext = $true
            }
            $result += $char
            continue
        }
        
        if ($inDoubleQuote) {
            if ($char -eq '"') {
                $inDoubleQuote = $false
            } elseif ($char -eq '\') {
                $escapeNext = $true
            }
            $result += $char
            continue
        }

        if ($inTemplateExpr) {
            if ($char -eq '{') { $templateExprDepth++ }
            elseif ($char -eq '}') {
                $templateExprDepth--
                if ($templateExprDepth -le 0) {
                    $inTemplateExpr = $false
                    $templateExprDepth = 0
                }
            }
            elseif ($char -eq '/' -and $nextChar -eq '/') {
                $inLineComment = $true
                $i++
                continue
            }
            elseif ($char -eq '/' -and $nextChar -eq '*') {
                $inBlockComment = $true
                $i++
                continue
            }
            elseif ($char -eq "'") { $inSingleQuote = $true }
            elseif ($char -eq '"') { $inDoubleQuote = $true }
            elseif ($char -eq '`') { $inTemplateLiteral = $true }
            $result += $char
            continue
        }

        if ($inTemplateLiteral) {
            if ($char -eq '`') {
                $inTemplateLiteral = $false
                $result += $char
                continue
            }
            if ($char -eq '$' -and $nextChar -eq '{') {
                $inTemplateExpr = $true
                $templateExprDepth = 1
                $result += $char
                continue
            }
            if ($char -eq '\') {
                $escapeNext = $true
            }
            $result += $char
            continue
        }
        
        if ($char -eq '/' -and $nextChar -eq '/') {
            $inLineComment = $true
            $i++
            continue
        }
        
        if ($char -eq '/' -and $nextChar -eq '*') {
            $inBlockComment = $true
            $i++
            continue
        }
        
        if ($char -eq "'") {
            $inSingleQuote = $true
        } elseif ($char -eq '"') {
            $inDoubleQuote = $true
        } elseif ($char -eq '`') {
            $inTemplateLiteral = $true
        }
        
        $result += $char
    }
    
    $result = [regex]::Replace($result, '\s+', ' ')
    $result = [regex]::Replace($result, '^\s+|\s+$', '', 'Multiline')
    $lines = $result -split "`n" | Where-Object { $_.Trim().Length -gt 0 }
    $result = $lines -join "`n"
    return $result
}

# Simple CSS minification function
function MinifyCSS($content) {
    $content = [regex]::Replace($content, '/\*[\s\S]*?\*/', '')
    $content = [regex]::Replace($content, '\s+', ' ')
    $content = [regex]::Replace($content, '\s*([{};:,>~+])\s*', '$1')
    $content = [regex]::Replace($content, ';}', '}')
    $content = [regex]::Replace($content, '^\s+|\s+$', '', 'Multiline')
    $lines = $content -split "`n" | Where-Object { $_.Trim().Length -gt 0 }
    $content = $lines -join ""
    return $content
}

# Merge files
function CreateBundle($bundleName, $files, $sourceDir, $outputDir, $type) {
    Write-Host "Creating $bundleName ..." -ForegroundColor Cyan
    
    $bundleContent = ""
    if ($type -eq "js") {
        $bundleContent = "// ForgeC2 Bundle: $bundleName`n"
        $bundleContent += "// Generated: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')`n"
        $bundleContent += "// Mode: $Mode`n`n"
    } elseif ($type -eq "css") {
        $bundleContent = "/* ForgeC2 Bundle: $bundleName */`n"
        $bundleContent += "/* Generated: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss') */`n"
        $bundleContent += "/* Mode: $Mode */`n`n"
    }
    
    $originalSize = 0
    
    foreach ($file in $files) {
        $filePath = Join-Path $sourceDir $file
        
        if (Test-Path $filePath) {
            Write-Host "  + $file" -ForegroundColor Green
            
            $fileContent = Get-Content $filePath -Raw -Encoding UTF8
            $originalSize += $fileContent.Length
            
            if ($type -eq "js") {
                $bundleContent += "// -------------------------------`n"
                $bundleContent += "// Source: $file`n"
                $bundleContent += "// -------------------------------`n`n"
            } elseif ($type -eq "css") {
                $bundleContent += "/* --- Source: $file --- */`n"
            }
            
            if ($Mode -eq "production" -and $Minify) {
                if ($type -eq "js") {
                    $fileContent = MinifyJS $fileContent
                } elseif ($type -eq "css") {
                    $fileContent = MinifyCSS $fileContent
                }
            }
            
            $bundleContent += $fileContent
            $bundleContent += "`n`n"
        } else {
            Write-Host "  ! File not found: $file" -ForegroundColor Yellow
        }
    }
    
    $outputPath = Join-Path $outputDir $bundleName
    $utf8NoBom = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText($outputPath, $bundleContent, $utf8NoBom)
    
    $size = (Get-Item $outputPath).Length
    $sizeKB = [math]::Round($size / 1KB, 2)
    $originalKB = [math]::Round($originalSize / 1KB, 2)
    
    if ($originalSize -gt 0) {
        $ratio = [math]::Round(($originalSize - $size) / $originalSize * 100, 1)
        Write-Host "  Generated: $bundleName ($originalKB KB -> $sizeKB KB, $ratio% reduction)" -ForegroundColor Green
    } else {
        Write-Host "  Generated: $bundleName ($sizeKB KB)" -ForegroundColor Green
    }
    
    return @{
        Original = $originalSize
        Bundle = $size
    }
}

# Main process
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  ForgeC2 Asset Bundling Tool" -ForegroundColor Cyan
Write-Host "  Mode: $Mode" -ForegroundColor Cyan
Write-Host "  Minify: $Minify" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

$totalOriginalSize = 0
$totalBundleSize = 0
$totalFiles = 0
$totalBundles = 0

# JS Bundling
if (-not $SkipJS) {
    Write-Host "`n--- JS Bundling ---" -ForegroundColor Yellow
    
    if (-not (Test-Path $JS_DIR)) {
        Write-Host "ERROR: JS directory not found: $JS_DIR" -ForegroundColor Red
        exit 1
    }
    
    foreach ($bundleName in $JS_BUNDLES.Keys) {
        $files = $JS_BUNDLES[$bundleName]
        $totalFiles += $files.Count
        $totalBundles += 1
        
        $result = CreateBundle $bundleName $files $JS_DIR $JS_OUTPUT_DIR "js"
        $totalOriginalSize += $result.Original
        $totalBundleSize += $result.Bundle
    }
}

# CSS Bundling
if (-not $SkipCSS) {
    Write-Host "`n--- CSS Bundling ---" -ForegroundColor Yellow
    
    if (-not (Test-Path $CSS_DIR)) {
        Write-Host "ERROR: CSS directory not found: $CSS_DIR" -ForegroundColor Red
        exit 1
    }
    
    foreach ($bundleName in $CSS_BUNDLES.Keys) {
        $files = $CSS_BUNDLES[$bundleName]
        $totalFiles += $files.Count
        $totalBundles += 1
        
        $result = CreateBundle $bundleName $files $CSS_DIR $CSS_OUTPUT_DIR "css"
        $totalOriginalSize += $result.Original
        $totalBundleSize += $result.Bundle
    }
}

# Output statistics
Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "  Bundling Complete" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

$originalKB = [math]::Round($totalOriginalSize / 1KB, 2)
$bundleKB = [math]::Round($totalBundleSize / 1KB, 2)

if ($totalOriginalSize -gt 0) {
    $compressionRatio = [math]::Round(($totalOriginalSize - $totalBundleSize) / $totalOriginalSize * 100, 1)
    $savedKB = [math]::Round(($totalOriginalSize - $totalBundleSize) / 1KB, 2)
    
    Write-Host "Original files total size: $originalKB KB" -ForegroundColor Yellow
    Write-Host "Bundles total size:        $bundleKB KB" -ForegroundColor Yellow
    Write-Host "Space saved:               $savedKB KB ($compressionRatio%)" -ForegroundColor Green
} else {
    Write-Host "Bundles total size: $bundleKB KB" -ForegroundColor Yellow
}

Write-Host "Files processed:           $totalFiles files" -ForegroundColor Green
Write-Host "Bundles created:           $totalBundles bundles" -ForegroundColor Green
Write-Host "HTTP requests reduced:     $totalFiles -> $totalBundles ($($totalFiles - $totalBundles) fewer requests)" -ForegroundColor Green

Write-Host "`nTip: Run 'go build -v ./cmd/server' to verify build" -ForegroundColor Cyan
