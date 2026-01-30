<#
.SYNOPSIS
    Combines all code files from ai-stack into a single file and documents directory structure.
    
.DESCRIPTION
    This script:
    1. Generates a directory tree structure
    2. Combines all code files with headers showing their paths
    3. Outputs everything to combined-code.txt
    
.EXAMPLE
    .\combine-code.ps1
#>

param(
    [string]$SourceDir = ".",
    [string]$OutputFile = "combined-code.txt"
)

$SourceDir = Resolve-Path $SourceDir

$CodeExtensions = @(
    ".py", ".yml", ".yaml", ".sql", ".conf", ".txt", ".md", 
    ".json", ".sh", ".env", ".gitkeep"
)

$ExcludePatterns = @(
    "combined-code.txt",
    "__pycache__",
    ".git",
    "*.pyc",
    ".env",
    "node_modules",
    "venv",
    ".venv"
)

$OutputPath = Join-Path $SourceDir $OutputFile
$Separator = "=" * 80
$FileSeparator = "-" * 80

Write-Host "Combining code from: $SourceDir" -ForegroundColor Cyan
Write-Host "Output file: $OutputPath" -ForegroundColor Cyan

$Content = @()

$Content += $Separator
$Content += "PERSONAL AI OPERATING SYSTEM - COMBINED SOURCE CODE"
$Content += "Generated: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"
$Content += "Source Directory: $SourceDir"
$Content += $Separator
$Content += ""

$Content += $Separator
$Content += "DIRECTORY STRUCTURE"
$Content += $Separator
$Content += ""

function Get-DirectoryTree {
    param(
        [string]$Path,
        [string]$Indent = ""
    )
    
    $items = Get-ChildItem -Path $Path -Force | Where-Object {
        $item = $_
        $excluded = $false
        foreach ($pattern in $ExcludePatterns) {
            if ($item.Name -like $pattern) {
                $excluded = $true
                break
            }
        }
        -not $excluded
    } | Sort-Object { -not $_.PSIsContainer }, Name
    
    $output = @()
    
    for ($i = 0; $i -lt $items.Count; $i++) {
        $item = $items[$i]
        $isLastItem = ($i -eq $items.Count - 1)
        
        if ($isLastItem) {
            $prefix = "+-- "
            $childIndent = "    "
        } else {
            $prefix = "|-- "
            $childIndent = "|   "
        }
        
        if ($item.PSIsContainer) {
            $output += "$Indent$prefix$($item.Name)/"
            $output += Get-DirectoryTree -Path $item.FullName -Indent "$Indent$childIndent"
        } else {
            $size = ""
            if ($item.Length -gt 1024) {
                $sizeKB = [math]::Round($item.Length / 1024, 1)
                $size = " ($sizeKB KB)"
            } elseif ($item.Length -gt 0) {
                $size = " ($($item.Length) B)"
            }
            $output += "$Indent$prefix$($item.Name)$size"
        }
    }
    
    return $output
}

$Content += "ai-stack/"
$treeOutput = Get-DirectoryTree -Path $SourceDir
$Content += $treeOutput
$Content += ""

$Content += $Separator
$Content += "FILE CONTENTS"
$Content += $Separator
$Content += ""

$Files = Get-ChildItem -Path $SourceDir -Recurse -File | Where-Object {
    $file = $_
    $ext = $file.Extension.ToLower()
    $name = $file.Name.ToLower()
    
    $includeByName = $name -eq "dockerfile"
    $includeByExt = $CodeExtensions -contains $ext
    
    $excluded = $false
    $relativePath = $file.FullName.Substring($SourceDir.Path.Length + 1)
    
    foreach ($pattern in $ExcludePatterns) {
        if ($file.Name -like $pattern -or $relativePath -like "*$pattern*") {
            $excluded = $true
            break
        }
    }
    
    ($includeByName -or $includeByExt) -and (-not $excluded)
} | Sort-Object FullName

$FileCount = $Files.Count
$CurrentFile = 0

foreach ($File in $Files) {
    $CurrentFile++
    $RelativePath = $File.FullName.Substring($SourceDir.Path.Length + 1)
    
    Write-Host "  [$CurrentFile/$FileCount] $RelativePath" -ForegroundColor Gray
    
    $Content += $FileSeparator
    $Content += "FILE: $RelativePath"
    $Content += "SIZE: $($File.Length) bytes"
    $modTime = $File.LastWriteTime.ToString("yyyy-MM-dd HH:mm:ss")
    $Content += "MODIFIED: $modTime"
    $Content += $FileSeparator
    $Content += ""
    
    try {
        $FileContent = Get-Content -Path $File.FullName -Raw -ErrorAction Stop
        if ($FileContent) {
            $Content += $FileContent
        } else {
            $Content += "[Empty file]"
        }
    } catch {
        $Content += "[Error reading file: $($_.Exception.Message)]"
    }
    
    $Content += ""
    $Content += ""
}

$Content += $Separator
$Content += "SUMMARY"
$Content += $Separator
$Content += ""
$Content += "Total files processed: $FileCount"
$Content += ""

$Content += "Files by directory:"
$Files | Group-Object { Split-Path $_.FullName.Substring($SourceDir.Path.Length + 1) -Parent } | 
    Sort-Object Name | ForEach-Object {
        $dir = if ($_.Name) { $_.Name } else { "(root)" }
        $Content += "  ${dir}: $($_.Count) files"
    }

$Content += ""

$Content += "Files by type:"
$Files | Group-Object Extension | Sort-Object Count -Descending | ForEach-Object {
    $ext = if ($_.Name) { $_.Name } else { "(no extension)" }
    $Content += "  ${ext}: $($_.Count) files"
}

$Content += ""
$Content += $Separator
$Content += "END OF COMBINED CODE"
$Content += $Separator

$Content -join "`n" | Out-File -FilePath $OutputPath -Encoding UTF8

$OutputSize = (Get-Item $OutputPath).Length
$OutputSizeKB = [math]::Round($OutputSize / 1024, 2)

Write-Host ""
Write-Host "Done! Combined $FileCount files into $OutputFile" -ForegroundColor Green
Write-Host "Output size: $OutputSizeKB KB ($OutputSize bytes)" -ForegroundColor Green
