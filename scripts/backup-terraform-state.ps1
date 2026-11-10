param(
    [string]$BackupDirectory = (Join-Path $env:LOCALAPPDATA 'RAS-Backend\terraform-state-backups')
)

$ErrorActionPreference = 'Stop'
$terraformDirectory = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\infra\terraform\dev'))

if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
    throw 'This backup uses Windows DPAPI. Run it in Windows PowerShell or PowerShell on Windows.'
}

$stateJson = (& terraform "-chdir=$terraformDirectory" state pull 2>$null | Out-String)
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($stateJson)) {
    throw 'Could not read Terraform state. Initialize the secured S3 backend first; no backup was written.'
}

try {
    $null = $stateJson | ConvertFrom-Json -ErrorAction Stop
} catch {
    throw 'Terraform did not return valid state JSON; no backup was written.'
}

New-Item -ItemType Directory -Path $BackupDirectory -Force | Out-Null
$plainBytes = [System.Text.Encoding]::UTF8.GetBytes($stateJson)
Add-Type -AssemblyName System.Security.Cryptography.ProtectedData
$encryptedState = [System.Security.Cryptography.ProtectedData]::Protect(
    $plainBytes,
    $null,
    [System.Security.Cryptography.DataProtectionScope]::CurrentUser
)
$stamp = [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssZ')
$backupPath = Join-Path $BackupDirectory "ras-backend-dev-$stamp.state.dpapi"
[System.IO.File]::WriteAllBytes($backupPath, $encryptedState)
Write-Output "Encrypted laptop backup created: $backupPath"
Write-Output 'It can be decrypted only by this Windows user on this computer. Keep S3 as the active Terraform backend.'
