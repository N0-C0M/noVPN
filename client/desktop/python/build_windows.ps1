param(
    [string]$PythonExe = "python",
    [string]$Version = "0.1.0",
    [switch]$SkipInstaller
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..\..")).Path
$buildRoot = Join-Path $repoRoot "client\desktop\build"
$distRoot = Join-Path $buildRoot "dist"
$workRoot = Join-Path $buildRoot "pyinstaller-work"
$specRoot = Join-Path $buildRoot "pyinstaller-spec"
$installerRoot = Join-Path $buildRoot "installer"
$appName = "NoVPN Desktop"
$entryPoint = Join-Path $repoRoot "client\desktop\python\app.py"

New-Item -ItemType Directory -Force -Path $buildRoot, $installerRoot | Out-Null
if (Test-Path $distRoot) {
    Remove-Item -LiteralPath $distRoot -Recurse -Force
}
if (Test-Path $workRoot) {
    Remove-Item -LiteralPath $workRoot -Recurse -Force
}
if (Test-Path $specRoot) {
    Remove-Item -LiteralPath $specRoot -Recurse -Force
}

Write-Host "Installing build dependency: pyinstaller"
& $PythonExe -m pip install --upgrade pyinstaller
if ($LASTEXITCODE -ne 0) {
    throw "Failed to install pyinstaller."
}

$pyInstallerArgs = @(
    "-m", "PyInstaller",
    "--noconfirm",
    "--clean",
    "--windowed",
    "--name", $appName,
    "--distpath", $distRoot,
    "--workpath", $workRoot,
    "--specpath", $specRoot,
    "--paths", (Join-Path $repoRoot "client\desktop\python"),
    "--add-data", ((Join-Path $repoRoot "client\common\profiles\reality\default.profile.json") + ";client\common\profiles\reality"),
    "--add-data", ((Join-Path $repoRoot "client\android\app\src\main\secure\bootstrap.json") + ";client\android\app\src\main\secure"),
    "--add-data", ((Join-Path $repoRoot "client\desktop\runtime\bin") + ";client\desktop\runtime\bin"),
    $entryPoint
)

Write-Host "Building desktop executable with PyInstaller"
& $PythonExe @pyInstallerArgs
if ($LASTEXITCODE -ne 0) {
    throw "PyInstaller build failed."
}

$appDistDir = Join-Path $distRoot $appName
$appExecutable = Join-Path $appDistDir "$appName.exe"
if (-not (Test-Path -LiteralPath $appExecutable)) {
    throw "Build output not found: $appExecutable"
}

Write-Host "Desktop build is ready: $appDistDir"

if ($SkipInstaller) {
    Write-Host "Skipping installer generation because -SkipInstaller was set."
    exit 0
}

$iscc = Get-Command "ISCC.exe" -ErrorAction SilentlyContinue
if ($null -eq $iscc) {
    Write-Warning "Inno Setup compiler (ISCC.exe) is not in PATH. Install Inno Setup or run with -SkipInstaller."
    exit 0
}

$issPath = Join-Path $repoRoot "client\desktop\installer\novpn-desktop.iss"
Write-Host "Building setup.exe with Inno Setup"
& $iscc.Source $issPath "/DMyAppVersion=$Version" "/DSourceDir=$appDistDir" "/DOutputDir=$installerRoot"
if ($LASTEXITCODE -ne 0) {
    throw "Inno Setup build failed."
}

Write-Host "Installer build is ready in: $installerRoot"
