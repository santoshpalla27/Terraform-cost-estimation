# Combine all Go code from clouds/ directory into a single file
# Usage: ./combine-clouds.ps1

$outputFile = "clouds-code.txt"
$cloudsDir = Join-Path $PSScriptRoot "clouds"

# Clear output file
"" | Out-File $outputFile -Encoding UTF8

# Header
$header = @"
================================================================================
                    CLOUDS DIRECTORY - COMBINED CODE
                    Generated: $(Get-Date -Format "yyyy-MM-dd HH:mm:ss")
================================================================================

"@
$header | Out-File $outputFile -Append -Encoding UTF8

# Get all .go files in clouds/ recursively
$goFiles = Get-ChildItem -Path $cloudsDir -Filter "*.go" -Recurse | Sort-Object FullName

Write-Host "Found $($goFiles.Count) Go files in clouds/"

foreach ($file in $goFiles) {
    # Get relative path from clouds/
    $relativePath = $file.FullName.Substring($cloudsDir.Length + 1)
    
    $fileHeader = @"

================================================================================
FILE: clouds/$relativePath
================================================================================

"@
    $fileHeader | Out-File $outputFile -Append -Encoding UTF8

    # Add file contents
    Get-Content $file.FullName | Out-File $outputFile -Append -Encoding UTF8
}

# Summary
$totalLines = (Get-Content $outputFile | Measure-Object -Line).Lines

$summary = @"

================================================================================
                              SUMMARY
================================================================================
Total files: $($goFiles.Count)
Total lines: $totalLines
Clouds covered: AWS, Azure, GCP
"@
$summary | Out-File $outputFile -Append -Encoding UTF8

Write-Host ""
Write-Host "Output written to: $outputFile"
Write-Host "Total files: $($goFiles.Count)"
Write-Host "Total lines: $totalLines"
