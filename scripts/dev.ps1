# Memuat .env ke environment proses lalu menjalankan: go run ./cmd/easypos <args>
# Contoh: .\scripts\dev.ps1 serve
$root = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $root '.env'
if (Test-Path $envFile) {
    Get-Content $envFile | ForEach-Object {
        if ($_ -match '^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)\s*$' -and $_ -notmatch '^\s*#') {
            [Environment]::SetEnvironmentVariable($Matches[1], $Matches[2], 'Process')
        }
    }
} else {
    Write-Warning ".env tidak ada; salin dari .env.example"
}
Push-Location $root
try { go run ./cmd/easypos @args } finally { Pop-Location }
