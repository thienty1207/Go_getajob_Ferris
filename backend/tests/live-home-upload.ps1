$ErrorActionPreference = 'Stop'

# Real boundary gate for the admin Home upload. It uses the running local API,
# the configured Cloudinary account, and PostgreSQL. All temporary identities,
# metadata, cleanup jobs, and provider assets are removed before success.
$backendRoot = Split-Path $PSScriptRoot -Parent
$projectRoot = Split-Path $backendRoot -Parent
$envPath = Join-Path $backendRoot '.env'
$imagePath = Join-Path $projectRoot 'img\logo\Logo.png'
$config = @{}
Get-Content -LiteralPath $envPath | ForEach-Object {
    $line = $_.Trim()
    if ($line -and -not $line.StartsWith('#')) {
        $parts = $line.Split('=', 2)
        if ($parts.Count -eq 2) {
            $config[$parts[0].Trim()] = $parts[1].Trim()
        }
    }
}

if (-not (Test-Path -LiteralPath $imagePath)) {
    throw "Live Home fixture is missing: $imagePath"
}
$psql = Get-ChildItem -LiteralPath 'C:\Program Files\PostgreSQL' -Directory |
    Sort-Object Name -Descending |
    ForEach-Object { Join-Path $_.FullName 'bin\psql.exe' } |
    Where-Object { Test-Path -LiteralPath $_ } |
    Select-Object -First 1
if (-not $psql) {
    throw 'psql.exe was not found under C:\Program Files\PostgreSQL.'
}

foreach ($key in @('DATABASE_HOST', 'DATABASE_PORT', 'DATABASE_USER', 'DATABASE_PASSWORD', 'DATABASE_NAME')) {
    if (-not $config[$key]) {
        throw "Missing $key in backend/.env."
    }
}

$env:PGPASSWORD = $config['DATABASE_PASSWORD']
$email = 'codex-home-e2e-' + [guid]::NewGuid().ToString('N') + '@example.test'
$password = 'Aa1!' + [guid]::NewGuid().ToString('N')
$createdAdmin = $false
$uploaded = $false
$slot = $null
$publicID = $null
$result = $null

function Invoke-DatabaseScalar {
    param([Parameter(Mandatory)][string]$Sql)

    $psqlArguments = @(
        '-h', $config['DATABASE_HOST'],
        '-p', $config['DATABASE_PORT'],
        '-U', $config['DATABASE_USER'],
        '-d', $config['DATABASE_NAME'],
        '-v', 'ON_ERROR_STOP=1',
        '-tAc', $Sql
    )
    $value = & $psql @psqlArguments
    if ($LASTEXITCODE -ne 0) {
        throw 'PostgreSQL command failed during the live Home gate.'
    }
    return ([string]$value).Trim()
}

try {
    $slot = Invoke-DatabaseScalar -Sql @"
SELECT candidate.slot
FROM generate_series(1, 3) AS candidate(slot)
WHERE NOT EXISTS (
    SELECT 1 FROM public.home_sections sections WHERE sections.slot = candidate.slot
)
ORDER BY candidate.slot DESC
LIMIT 1;
"@
    if (-not $slot) {
        throw 'No empty Home section slot is available; the live gate refuses to overwrite user content.'
    }

    $credentialInput = $password + [Environment]::NewLine + $password + [Environment]::NewLine
    Push-Location $backendRoot
    try {
        $provisionOutput = $credentialInput | go run .\cmd\admin\ create-user --email $email 2>&1
        if ($LASTEXITCODE -ne 0) {
            throw 'Temporary admin provisioning failed.'
        }
    }
    finally {
        Pop-Location
    }
    $createdAdmin = $true

    $webSession = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
    $loginRequest = @{
        Uri        = 'http://127.0.0.1:8080/api/v1/admin/auth/login'
        Method     = 'Post'
        ContentType = 'application/json'
        Body       = @{ email = $email; password = $password } | ConvertTo-Json
        WebSession = $webSession
        TimeoutSec = 15
    }
    $login = Invoke-RestMethod @loginRequest
    if (-not $login.csrf_token) {
        throw 'Admin login did not return a CSRF token.'
    }

    $uploadRequest = @{
        Uri        = "http://127.0.0.1:8080/api/v1/admin/home-sections/$slot"
        Method     = 'Put'
        Form       = @{
            is_active = 'true'
            title     = 'Controlled Home upload gate'
            body      = 'Temporary integration content removed after verification.'
            image     = Get-Item -LiteralPath $imagePath
        }
        Headers    = @{ 'X-CSRF-Token' = [string]$login.csrf_token }
        WebSession = $webSession
        TimeoutSec = 30
    }
    $section = Invoke-RestMethod @uploadRequest
    $uploaded = $true
    if ([string]$section.slot -ne [string]$slot -or -not $section.image_url -or -not $section.image_content_hash) {
        throw 'Home upload response is missing persisted image metadata.'
    }
    if (([uri]$section.image_url).Scheme -ne 'https') {
        throw 'Home upload returned a non-HTTPS provider URL.'
    }

    $publicRequest = @{
        Uri        = 'http://127.0.0.1:8080/api/v1/client/home-sections'
        Method     = 'Get'
        TimeoutSec = 15
    }
    $public = Invoke-RestMethod @publicRequest
    $publicSection = @($public.sections) | Where-Object { [string]$_.slot -eq [string]$slot }
    if ($publicSection.Count -ne 1 -or $publicSection[0].image_url -ne $section.image_url) {
        throw 'Public Home did not expose the persisted provider image.'
    }

    $deliveryRequest = @{
        UseBasicParsing = $true
        Uri             = $section.image_url
        Method          = 'Head'
        TimeoutSec      = 20
    }
    $deliveryResponse = Invoke-WebRequest @deliveryRequest
    if ($deliveryResponse.StatusCode -lt 200 -or $deliveryResponse.StatusCode -ge 400) {
        throw 'Cloudinary delivery URL is not reachable.'
    }

    $result = [pscustomobject]@{
        AdminLogin          = 'PASS'
        MultipartUpload     = 'PASS'
        DatabaseMetadata    = 'PASS'
        PublicHomeRead      = 'PASS'
        CloudinaryDelivery  = "PASS ($($deliveryResponse.StatusCode))"
        DurableAssetCleanup = 'PENDING'
    }
}
finally {
    if ($uploaded) {
        $publicID = Invoke-DatabaseScalar -Sql "SELECT cloudinary_public_id FROM public.home_sections WHERE slot = $slot AND updated_by = '$email';"
        [void](Invoke-DatabaseScalar -Sql @"
WITH removed AS (
    DELETE FROM public.home_sections
    WHERE slot = $slot AND updated_by = '$email'
    RETURNING cloudinary_public_id
)
INSERT INTO public.home_asset_cleanup_queue (cloudinary_public_id)
SELECT cloudinary_public_id
FROM removed
WHERE cloudinary_public_id IS NOT NULL
ON CONFLICT (cloudinary_public_id) DO NOTHING;
"@)
    }
    if ($createdAdmin) {
        [void](Invoke-DatabaseScalar -Sql "DELETE FROM public.admin_users WHERE email = '$email';")
    }

    if ($publicID) {
        $cleanupRemaining = '1'
        for ($attempt = 0; $attempt -lt 90; $attempt++) {
            $cleanupRemaining = Invoke-DatabaseScalar -Sql "SELECT count(*) FROM public.home_asset_cleanup_queue WHERE cloudinary_public_id = '$publicID';"
            if ($cleanupRemaining -eq '0') {
                break
            }
            Start-Sleep -Milliseconds 500
        }
        if ($cleanupRemaining -ne '0') {
            throw 'Durable Home asset cleanup did not finish within 45 seconds.'
        }
        if ($result) {
            $result.DurableAssetCleanup = 'PASS'
        }
    }
    Remove-Item Env:PGPASSWORD -ErrorAction SilentlyContinue
}

$result | Format-List
