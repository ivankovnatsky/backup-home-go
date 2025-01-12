param (
    [Parameter(Position=0)]
    [string]$Command = "build",
    [string]$RcloneRemote = "drive_Crypt"
)

$BinaryName = "backup-home"
$MainPath = "./cmd/backup-home"
$DistDir = "dist"
$RclonePath = "Machines/$env:COMPUTERNAME/Users/$env:USERNAME"

function Ensure-Directory {
    if (-not (Test-Path $DistDir)) {
        New-Item -ItemType Directory -Path $DistDir | Out-Null
    }
}

function Clean {
    Write-Host "Cleaning..."
    if (Test-Path $DistDir) {
        Remove-Item -Path $DistDir -Recurse -Force
    }
    go clean
}

function Build {
    Write-Host "Building..."
    Ensure-Directory
    go build -o "$DistDir/$BinaryName.exe" $MainPath
}

function Test {
    Write-Host "Running tests..."
    go test -v ./...
}

function Lint {
    Write-Host "Running linter..."
    golangci-lint run
}

function Run {
    Build
    Write-Host "Running..."
    & ".\$DistDir\$BinaryName.exe" `
        --source $env:USERPROFILE `
        --destination "${RcloneRemote}:${RclonePath}"
}

function RunDebug {
    Build
    Write-Host "Running with debug output..."
    & ".\$DistDir\$BinaryName.exe" `
        --source $env:USERPROFILE `
        --destination "${RcloneRemote}:${RclonePath}" `
        --verbose
}

function DryRun {
    Build
    Write-Host "Dry run..."
    & ".\$DistDir\$BinaryName.exe" `
        --source $env:USERPROFILE `
        --destination "${RcloneRemote}:${RclonePath}" `
        --preview
}

# Execute requested command
switch ($Command.ToLower()) {
    "clean" { Clean }
    "build" { Build }
    "test" { Test }
    "lint" { Lint }
    "run" { Run }
    "run-debug" { RunDebug }
    "dry-run" { DryRun }
    default { 
        Write-Host "Unknown command: $Command"
        Write-Host "Available commands: clean, build, test, lint, run, run-debug, dry-run"
    }
}
