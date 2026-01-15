<#
.SYNOPSIS
    Combines all source code files into a single combined-code.txt file.

.DESCRIPTION
    This script:
    1. Generates a directory tree structure
    2. Combines all relevant source files into a single file
    3. Excludes binary files, dependencies, and build artifacts
    4. Includes file separators with full paths for easy navigation

.PARAMETER OutputFile
    The output file path. Default: combined-code.txt

.PARAMETER ProjectPath
    The project root path. Default: current directory

.EXAMPLE
    .\combine-code.ps1
    .\combine-code.ps1 -OutputFile "all-code.txt" -ProjectPath "D:\my-project"
#>

param(
    [string]$OutputFile = "combined-code.txt",
    [string]$ProjectPath = "."
)

# Resolve to absolute path
$ProjectPath = Resolve-Path $ProjectPath

Write-Host "==================================" -ForegroundColor Cyan
Write-Host "  Code Combiner Script" -ForegroundColor Cyan
Write-Host "==================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Project Path: $ProjectPath" -ForegroundColor Yellow
Write-Host "Output File:  $OutputFile" -ForegroundColor Yellow
Write-Host ""

# File extensions to include
$IncludeExtensions = @(
    "*.go",
    "*.mod",
    "*.sum",
    "*.tf",
    "*.tfvars",
    "*.json",
    "*.yaml",
    "*.yml",
    "*.toml",
    "*.md",
    "*.txt",
    "*.sh",
    "*.ps1",
    "*.sql",
    "*.hcl",
    "Dockerfile*",
    "docker-compose*",
    "Makefile",
    ".gitignore",
    ".dockerignore"
)

# Directories to exclude
$ExcludeDirectories = @(
    ".git",
    ".idea",
    ".vscode",
    "node_modules",
    "vendor",
    ".terraform",
    "dist",
    "build",
    "bin",
    "__pycache__",
    ".cache",
    "cache",
    "tmp",
    "temp",
    "logs",
    "coverage"
)

# Files to exclude
$ExcludeFiles = @(
    "combined-code.txt",
    "*.exe",
    "*.dll",
    "*.so",
    "*.dylib",
    "*.zip",
    "*.tar",
    "*.gz",
    "*.db",
    "*.sqlite",
    "*.log",
    "*.tfstate",
    "*.tfstate.backup",
    "go.sum",
    "infracost-code.txt"
)

# Create output file and clear if exists
$OutputFullPath = Join-Path $ProjectPath $OutputFile
if (Test-Path $OutputFullPath) {
    Remove-Item $OutputFullPath -Force
}

# =================================
# Section 1: Directory Structure
# =================================
Write-Host "Generating directory structure..." -ForegroundColor Green

$timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
$content = @"
################################################################################
#                                                                              #
#                    TERRAFORM COST ESTIMATION SYSTEM                          #
#                         Combined Source Code                                 #
#                                                                              #
#                    Generated: $timestamp                          #
#                                                                              #
################################################################################

================================================================================
                           DIRECTORY STRUCTURE
================================================================================

"@

# Generate directory tree using ASCII characters
function Get-DirectoryTree {
    param(
        [string]$Path,
        [string]$Prefix = "",
        [int]$MaxDepth = 10,
        [int]$CurrentDepth = 0
    )

    if ($CurrentDepth -ge $MaxDepth) { return "" }

    $output = ""
    $items = Get-ChildItem -Path $Path -Force | Where-Object {
        $item = $_
        $isExcluded = $false
        
        # Check if directory should be excluded
        if ($item.PSIsContainer) {
            foreach ($exclude in $ExcludeDirectories) {
                if ($item.Name -eq $exclude) {
                    $isExcluded = $true
                    break
                }
            }
        }
        
        -not $isExcluded
    } | Sort-Object { -not $_.PSIsContainer }, Name

    $count = $items.Count
    $index = 0

    foreach ($item in $items) {
        $index++
        $isLast = ($index -eq $count)
        
        # Use ASCII characters instead of Unicode
        if ($isLast) {
            $connector = "+-- "
            $extension = "    "
        } else {
            $connector = "|-- "
            $extension = "|   "
        }

        if ($item.PSIsContainer) {
            $output += "$Prefix$connector$($item.Name)/`n"
            $output += Get-DirectoryTree -Path $item.FullName -Prefix "$Prefix$extension" -MaxDepth $MaxDepth -CurrentDepth ($CurrentDepth + 1)
        } else {
            $output += "$Prefix$connector$($item.Name)`n"
        }
    }

    return $output
}

$projectName = Split-Path $ProjectPath -Leaf
$content += "$projectName/`n"
$content += Get-DirectoryTree -Path $ProjectPath

$content += @"

================================================================================
                              SOURCE FILES
================================================================================

"@

# =================================
# Section 2: Combine Source Files
# =================================
Write-Host "Collecting source files..." -ForegroundColor Green

# Collect all files matching our patterns
$allFiles = @()

foreach ($pattern in $IncludeExtensions) {
    $files = Get-ChildItem -Path $ProjectPath -Filter $pattern -Recurse -File -ErrorAction SilentlyContinue | Where-Object {
        $file = $_
        $include = $true
        
        # Check if in excluded directory
        foreach ($excludeDir in $ExcludeDirectories) {
            if ($file.FullName -like "*\$excludeDir\*" -or $file.FullName -like "*/$excludeDir/*") {
                $include = $false
                break
            }
        }
        
        # Check if file should be excluded
        foreach ($excludeFile in $ExcludeFiles) {
            if ($file.Name -like $excludeFile) {
                $include = $false
                break
            }
        }
        
        $include
    }
    
    $allFiles += $files
}

# Remove duplicates and sort
$allFiles = $allFiles | Sort-Object FullName -Unique

Write-Host "Found $($allFiles.Count) files to combine" -ForegroundColor Yellow

# Group files by directory for better organization
$filesByDir = $allFiles | Group-Object { Split-Path $_.FullName -Parent }

$processedCount = 0
foreach ($group in $filesByDir | Sort-Object Name) {
    foreach ($file in $group.Group | Sort-Object Name) {
        $relativePath = $file.FullName.Substring($ProjectPath.Path.Length + 1)
        $processedCount++
        
        Write-Host "  [$processedCount/$($allFiles.Count)] $relativePath" -ForegroundColor Gray
        
        # Determine file type for syntax highlighting hint
        $fileType = switch -Regex ($file.Extension) {
            "\.go$" { "go" }
            "\.tf$|\.hcl$" { "hcl" }
            "\.json$" { "json" }
            "\.ya?ml$" { "yaml" }
            "\.md$" { "markdown" }
            "\.ps1$" { "powershell" }
            "\.sh$" { "bash" }
            "\.sql$" { "sql" }
            default { "text" }
        }
        
        $fileSize = $file.Length
        $separator = @"

################################################################################
# FILE: $relativePath
# TYPE: $fileType
# SIZE: $fileSize bytes
################################################################################

"@
        $content += $separator
        
        try {
            $fileContent = Get-Content -Path $file.FullName -Raw -ErrorAction Stop
            if ($fileContent) {
                $content += $fileContent
                # Ensure file ends with newline
                if (-not $fileContent.EndsWith("`n")) {
                    $content += "`n"
                }
            }
        } catch {
            $content += "# ERROR: Could not read file - $($_.Exception.Message)`n"
        }
    }
}

# =================================
# Section 3: Summary
# =================================
$summaryTimestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
$content += @"

################################################################################
#                              END OF FILE                                     #
################################################################################

================================================================================
                                SUMMARY
================================================================================

Total Files Combined: $($allFiles.Count)
Generated: $summaryTimestamp
Project: $projectName

Files by Type:
"@

# Count files by extension
$extensionCounts = $allFiles | Group-Object Extension | Sort-Object Count -Descending
foreach ($ext in $extensionCounts) {
    $extName = if ($ext.Name) { $ext.Name } else { "(no extension)" }
    $content += "  $extName : $($ext.Count) files`n"
}

$content += @"

================================================================================
"@

# Write to output file
Write-Host ""
Write-Host "Writing output file..." -ForegroundColor Green
$content | Out-File -FilePath $OutputFullPath -Encoding UTF8

# Get output file size
$outputSize = (Get-Item $OutputFullPath).Length
$outputSizeMB = [math]::Round($outputSize / 1MB, 2)
$outputSizeKB = [math]::Round($outputSize / 1KB, 2)

Write-Host ""
Write-Host "==================================" -ForegroundColor Cyan
Write-Host "  Complete!" -ForegroundColor Green
Write-Host "==================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Output: $OutputFullPath" -ForegroundColor Yellow
Write-Host "Size:   $outputSizeKB KB ($outputSizeMB MB)" -ForegroundColor Yellow
Write-Host "Files:  $($allFiles.Count) combined" -ForegroundColor Yellow
Write-Host ""
