<#
.SYNOPSIS
    Build the single-file release binary: dist/keyboard-statistics-v<version>.exe.

.DESCRIPTION
    Steps:
      1. Generate the Windows resource object (assets/app.manifest + version info)
         into cmd/app/rsrc_windows_<arch>.syso with go-winres; go build links it
         into the exe automatically.
      2. Try CGO_ENABLED=0 first (pure Go deps mean a smaller, dependency-free
         binary). If that fails, fall back to the default cgo build.
      3. Verify: artifact exists, size is sane, version string and manifest
         markers are really inside the binary.
      4. Self-test: build a throwaway console variant and run --version
         (the GUI variant has no console, so its output is invisible).

    go-winres is executed with "go run module@version", so nothing is added to
    this project's go.mod / go.sum.

.PARAMETER Version
    Version string, default 0.1.0-dev. Used both for -X main.version and for
    the artifact name.

.PARAMETER SkipVersionSelfTest
    Skip step 4 (the console self-test). The byte-level checks still run.

.NOTES
    Why this file is pure ASCII:
      Windows PowerShell 5.1 decodes BOM-less scripts with the ANSI code page.
      Chinese comments in a UTF-8 (no BOM) file get mis-decoded, and a UTF-8
      trailing byte can even swallow the following ASCII character, which
      breaks parsing outright (verified on this machine). The repo requires
      UTF-8 without BOM, so the safe intersection is ASCII: keep every
      character in this file below U+0080.

    Why go-winres "make" instead of "simply":
      "simply" only offers its built-in cli/gui manifests; a custom
      assets/app.manifest requires "make" plus a winres.json. Resource paths
      inside winres.json are resolved relative to the json file's directory,
      which is why "app.manifest" is enough there.

    Why the numeric version is rewritten:
      go-winres --file-version only accepts plain 1.2.3 or 1.2.3.4, so
      0.1.0-test is normalized to 0.1.0.0 before being passed on.

    Network:
      go-winres is fetched through the configured GOPROXY. GitHub direct
      connects usually fail here, but goproxy.cn works:
          $env:GOPROXY = 'https://goproxy.cn,direct'
          go run github.com/tc-hib/go-winres@v0.3.3 make ...
      Replace the module version below with @latest if you want the newest.

    CGO:
      While this project used fyne (OpenGL), CGO_ENABLED=0 always failed
      because go-gl/gl is cgo-only. With the native Win32 UI it may succeed,
      so the script probes on every run instead of hardcoding a choice.
#>
[CmdletBinding()]
param(
    [string]$Version = '0.1.0-dev',
    [switch]$SkipVersionSelfTest
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# PowerShell 7.3+ turns native stderr into terminating errors; the build result
# is judged by exit code here, so keep it quiet where the variable exists.
if (Get-Variable -Name PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue) {
    $PSNativeCommandUseErrorActionPreference = $false
}

# The version ends up in a file name and in -ldflags; reject anything that
# could split the argument list or escape the dist directory.
if ($Version -notmatch '^[0-9A-Za-z][0-9A-Za-z._+-]*$') {
    throw "Invalid version '$Version': only letters, digits and . _ + - are allowed, and it must start with a letter or digit."
}

$repoRoot     = Split-Path -Parent $PSScriptRoot
$distDir      = Join-Path $repoRoot 'dist'
$exeName      = "keyboard-statistics-v$Version.exe"
$exePath      = Join-Path $distDir $exeName
$manifestPath = Join-Path $repoRoot 'assets\app.manifest'
$winresJSON   = Join-Path $repoRoot 'assets\winres.json'
$appPkg       = Join-Path $repoRoot 'cmd\app'
$sysoPrefix   = Join-Path $appPkg 'rsrc'
$winresModule = 'github.com/tc-hib/go-winres@v0.3.3'

function Write-Step {
    param([string]$Text)
    Write-Host ''
    Write-Host "==> $Text" -ForegroundColor Cyan
}

# go-winres only accepts 1.2.3 or 1.2.3.4; turn 0.1.0-test / v1.2.3 into 0.1.0.0 / 1.2.3.0.
function ConvertTo-WinVersion {
    param([string]$Version)
    $core = (($Version.Trim() -replace '^[vV]', '') -split '[-+]')[0]
    $parts = @()
    foreach ($piece in ($core -split '\.')) {
        $n = 0
        if (-not [int]::TryParse($piece, [ref]$n)) { $n = 0 }
        $parts += [Math]::Max(0, [Math]::Min(65535, $n))
        if ($parts.Count -eq 4) { break }
    }
    while ($parts.Count -lt 4) { $parts += 0 }
    return ($parts -join '.')
}

# Runs go build and returns its exit code plus merged output. stderr is
# redirected to a file on purpose: "2>&1" would turn native stderr into
# ErrorRecord objects under Windows PowerShell 5.1 and pollute the log.
function Invoke-GoBuild {
    param([string]$Ldflags, [string]$OutFile)
    $buildArgs = @('build', '-trimpath', '-ldflags', $Ldflags, '-o', $OutFile, './cmd/app')
    $errFile = [System.IO.Path]::GetTempFileName()
    try {
        $stdout = @(& go @buildArgs 2> $errFile)
        $code = $LASTEXITCODE
        $stderr = Get-Content -LiteralPath $errFile -Raw -ErrorAction SilentlyContinue
        if ($null -eq $stderr) { $stderr = '' }
        $merged = (@($stdout) + @([string]$stderr)) -join [Environment]::NewLine
        return [pscustomobject]@{
            Code   = $code
            Output = $merged.Trim()
        }
    }
    finally {
        Remove-Item -LiteralPath $errFile -Force -ErrorAction SilentlyContinue
    }
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw 'go was not found in PATH. Add the Go bin directory to PATH first.'
}
if (-not (Test-Path -LiteralPath $manifestPath)) {
    throw "Missing application manifest: $manifestPath"
}
if (-not (Test-Path -LiteralPath $winresJSON)) {
    throw "Missing winres config: $winresJSON"
}

$goVersion = (& go version).Trim()
if (Test-Path Env:GOARCH) { $arch = $env:GOARCH } else { $arch = (& go env GOARCH).Trim() }
if (-not ($arch -in @('amd64', '386', 'arm64'))) {
    throw "go-winres does not support architecture '$arch' (supported: amd64, 386, arm64)."
}

$winVersion = ConvertTo-WinVersion $Version
$sysoFiles = @()

Write-Host "Repository : $repoRoot"
Write-Host "Go         : $goVersion"
Write-Host "Arch       : $arch"
Write-Host "Version    : $Version (numeric resource version $winVersion)"

Push-Location -LiteralPath $repoRoot
try {
    New-Item -ItemType Directory -Force -Path $distDir | Out-Null
    # Repeatable builds: drop the previous artifact first so a failed build can
    # never leave an older exe behind as the "new" release.
    if (Test-Path -LiteralPath $exePath) {
        Remove-Item -LiteralPath $exePath -Force
        Write-Host "Removed previous artifact: $exePath"
    }

    Write-Step '1/4 Generate resource object (manifest + version info)'
    $winresArgs = @(
        'run', $winresModule, 'make',
        '--in', $winresJSON,
        '--out', $sysoPrefix,
        '--arch', $arch,
        '--file-version', $winVersion,
        '--product-version', $winVersion
    )
    Write-Host ("go " + ($winresArgs -join ' '))
    $winresErr = [System.IO.Path]::GetTempFileName()
    try {
        $winresOut = @(& go @winresArgs 2> $winresErr)
        $winresCode = $LASTEXITCODE
        $winresErrText = Get-Content -LiteralPath $winresErr -Raw -ErrorAction SilentlyContinue
    }
    finally {
        Remove-Item -LiteralPath $winresErr -Force -ErrorAction SilentlyContinue
    }
    if ($winresOut) { Write-Host ((@($winresOut) + @([string]$winresErrText)) -join [Environment]::NewLine).Trim() }
    elseif ($winresErrText) { Write-Host ([string]$winresErrText).Trim() }
    if ($winresCode -ne 0) {
        throw "go-winres failed (exit code $winresCode). Check that GOPROXY can fetch $winresModule."
    }
    $sysoFiles = @(Get-ChildItem -Path $appPkg -Filter 'rsrc_windows_*.syso' -ErrorAction SilentlyContinue)
    if ($sysoFiles.Count -eq 0) {
        throw "go-winres produced no rsrc_windows_*.syso under $appPkg, resources would be missing."
    }
    foreach ($f in $sysoFiles) { Write-Host "  + $($f.Name) ($($f.Length) bytes)" }

    Write-Step '2/4 go build'
    $ldflags = "-s -w -H windowsgui -X main.version=$Version"
    Write-Host "ldflags: $ldflags"

    if (Test-Path Env:CGO_ENABLED) { $savedCgo = $env:CGO_ENABLED } else { $savedCgo = $null }
    $build = $null
    $env:CGO_ENABLED = '0'
    try {
        Write-Host 'Probing CGO_ENABLED=0 (pure Go, smaller binary) ...'
        $build = Invoke-GoBuild -Ldflags $ldflags -OutFile $exePath
    }
    finally {
        # Always restore, never leak the probe setting to the caller's shell.
        if ($null -eq $savedCgo) {
            Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
        }
        else {
            $env:CGO_ENABLED = $savedCgo
        }
    }

    if ($build.Code -eq 0) {
        $cgoMode = 'CGO_ENABLED=0 (no cgo)'
    }
    else {
        Write-Host 'CGO_ENABLED=0 build failed, falling back to the default cgo build. Reason (trimmed):' -ForegroundColor Yellow
        $reason = (($build.Output -split "\r?\n") | Where-Object { $_ -match '\S' } | Select-Object -First 8) -join [Environment]::NewLine
        Write-Host $reason
        if (Test-Path -LiteralPath $exePath) { Remove-Item -LiteralPath $exePath -Force }
        Write-Host 'Rebuilding with the default cgo setting ...'
        $build = Invoke-GoBuild -Ldflags $ldflags -OutFile $exePath
        if ($build.Code -ne 0) {
            throw "go build failed (exit code $($build.Code)):`n$($build.Output)"
        }
        $cgoMode = 'default cgo (CGO_ENABLED=0 unavailable)'
    }
    if ($build.Output) { Write-Host $build.Output }

    Write-Step '3/4 Verify artifact'
    if (-not (Test-Path -LiteralPath $exePath)) {
        throw "go build reported success but produced no artifact: $exePath"
    }
    $size = (Get-Item -LiteralPath $exePath).Length
    if ($size -lt 1MB) {
        throw "Artifact is only $size bytes, clearly incomplete."
    }

    # -H windowsgui has no console, so --version is invisible; search the PE
    # bytes instead. -X main.version writes a string literal into the read-only
    # data section, and -s -w does not remove it. The manifest is embedded as
    # UTF-8 XML in .rsrc, so its markers are findable the same way.
    $bytes = [System.IO.File]::ReadAllBytes($exePath)
    $binaryText = [System.Text.Encoding]::UTF8.GetString($bytes)

    if (-not $binaryText.Contains($Version)) {
        throw "Version '$Version' not found inside the artifact: -X main.version did not take effect."
    }
    Write-Host "  Version '$Version' is embedded" -ForegroundColor Green

    foreach ($marker in @('requestedExecutionLevel', 'level="asInvoker"', 'PerMonitorV2', 'true/pm')) {
        if (-not $binaryText.Contains($marker)) {
            throw "Manifest marker '$marker' not found inside the artifact: the resource was not linked in."
        }
    }
    Write-Host '  Manifest embedded: asInvoker + PerMonitorV2 (plus dpiAware=true/pm)' -ForegroundColor Green
    $binaryText = $null
    $bytes = $null

    if (-not $SkipVersionSelfTest) {
        Write-Step '4/4 Version self-test (temporary console build)'
        $probeExe = Join-Path ([System.IO.Path]::GetTempPath()) ("ks-version-probe-{0}.exe" -f [guid]::NewGuid().ToString('N'))
        try {
            # Same flags minus -H windowsgui, so this is literally the same code.
            $probe = Invoke-GoBuild -Ldflags "-s -w -X main.version=$Version" -OutFile $probeExe
            if ($probe.Code -ne 0) { throw "Console self-test build failed:`n$($probe.Output)" }

            $psi = New-Object System.Diagnostics.ProcessStartInfo
            $psi.FileName = $probeExe
            $psi.Arguments = '--version'
            $psi.UseShellExecute = $false
            $psi.RedirectStandardOutput = $true
            $psi.RedirectStandardError = $true
            $psi.CreateNoWindow = $true

            $proc = [System.Diagnostics.Process]::Start($psi)
            # Wait first, then read: the output is a single line, so the pipe
            # cannot fill up. The timeout keeps a broken --version flag (which
            # would start the real GUI) from hanging the build forever.
            $exited = $proc.WaitForExit(10000)
            if (-not $exited) {
                try { $proc.Kill() } catch { }
                throw '--version did not exit within 10s; the --version flag may be broken.'
            }
            $selfOut = $proc.StandardOutput.ReadToEnd().Trim()
            $selfErr = $proc.StandardError.ReadToEnd().Trim()
            if ($proc.ExitCode -ne 0) {
                throw "--version exited with code $($proc.ExitCode). stderr: $selfErr"
            }
            if (-not $selfOut.Contains($Version)) {
                throw "Self-test output does not contain '$Version'. Actual output: '$selfOut'"
            }
            Write-Host "  Self-test output: $selfOut" -ForegroundColor Green
        }
        finally {
            if (Test-Path -LiteralPath $probeExe) { Remove-Item -LiteralPath $probeExe -Force -ErrorAction SilentlyContinue }
        }
    }

    Write-Host ''
    Write-Host 'Build finished' -ForegroundColor Green
    Write-Host "  Artifact   : $exePath"
    Write-Host ("  Size       : {0:N0} bytes ({1:N2} MB)" -f $size, ($size / 1MB))
    Write-Host "  cgo        : $cgoMode"
    Write-Host "  Version    : $Version (-X main.version)"
    Write-Host "  ResVersion : $winVersion"
    Write-Host '  Manifest   : asInvoker + PerMonitorV2 + Win10/11 compatibility'
}
finally {
    # The .syso is an input to go build; leaving it in the repo would invite
    # accidental commits and is not portable, so it is always removed.
    foreach ($f in $sysoFiles) {
        if (Test-Path -LiteralPath $f.FullName) {
            Remove-Item -LiteralPath $f.FullName -Force -ErrorAction SilentlyContinue
            Write-Host "Removed temporary resource object: $($f.Name)"
        }
    }
    Get-ChildItem -Path $appPkg -Filter 'rsrc_windows_*.syso' -ErrorAction SilentlyContinue |
        ForEach-Object { Remove-Item -LiteralPath $_.FullName -Force -ErrorAction SilentlyContinue }
    Pop-Location
}
