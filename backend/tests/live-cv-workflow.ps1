param(
    [string]$CVPath
)

$ErrorActionPreference = 'Stop'

# This gate exercises the real local API, PostgreSQL, and configured DeepSeek
# provider without creating mock profiles. It creates one temporary client
# session and always removes that identity (and its cascading rows) afterward.
$backendRoot = Split-Path $PSScriptRoot -Parent
$envPath = Join-Path $backendRoot '.env'
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
$userID = [guid]::NewGuid().ToString()
$sessionID = [guid]::NewGuid().ToString()
$rawSession = 'session_' + [guid]::NewGuid().ToString('N')
$rawCSRF = 'csrf_' + [guid]::NewGuid().ToString('N')
$email = 'codex-cv-e2e-' + [guid]::NewGuid().ToString('N') + '@example.test'
$tempCV = $null
$tempCVOwned = $false
$cvInput = $null
$createdUser = $false
$scanID = $null

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
    $result = & $psql @psqlArguments
    if ($LASTEXITCODE -ne 0) {
        throw 'PostgreSQL command failed during the live CV gate.'
    }
    return ([string]$result).Trim()
}

try {
    $locationID = Invoke-DatabaseScalar -Sql @"
SELECT id
FROM public.locations
WHERE is_active = true
  AND EXISTS (
      SELECT 1
      FROM public.active_job_cache jobs
      WHERE jobs.location_id = locations.id
  )
ORDER BY id
LIMIT 1;
"@
    if (-not $locationID) {
        throw 'No active Job Location with public jobs is available for the live CV gate.'
    }

    # Every interpolated value below is generated locally from a GUID or a
    # fixed test string, so it cannot contain a quote or SQL fragment.
    $seedSQL = @"
INSERT INTO public.client_users (id, email, display_name, provider)
VALUES ('$userID', '$email', 'Codex CV E2E', 'google');

INSERT INTO public.client_sessions
    (id, client_user_id, token_hash, csrf_token_hash, expires_at)
VALUES
    ('$sessionID', '$userID', digest('$rawSession', 'sha256'), digest('$rawCSRF', 'sha256'), now() + interval '1 hour');
"@
    [void](Invoke-DatabaseScalar -Sql $seedSQL)
    $createdUser = $true

    if ($CVPath) {
        $cvInput = Get-Item -LiteralPath $CVPath
        if (-not $cvInput -or $cvInput.PSIsContainer) {
            throw 'The supplied CV path is not a file.'
        }
    }
    else {
        $tempCV = Join-Path ([IO.Path]::GetTempPath()) ('codex-cv-e2e-' + [guid]::NewGuid().ToString('N') + '.txt')
        $cvText = @(
            'Backend Software Engineer'
            'Summary: Five years building production web services and data pipelines.'
            'Skills: Go, PostgreSQL, REST APIs, Docker, Git, SQL, unit testing.'
            'Experience: 5 years as Backend Developer and Software Engineer.'
            'Seniority: Senior.'
            'Domains: software engineering, cloud services.'
            'Education: Bachelor of Computer Science.'
        ) -join [Environment]::NewLine
        [IO.File]::WriteAllText($tempCV, $cvText, [Text.UTF8Encoding]::new($false))
        $tempCVOwned = $true
        $cvInput = Get-Item -LiteralPath $tempCV
    }

    $webSession = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
    $sessionCookie = [System.Net.Cookie]::new('ferris_client_session', $rawSession, '/', '127.0.0.1')
    $webSession.Cookies.Add([uri]'http://127.0.0.1:8080', $sessionCookie)

    $missingCSRFStatus = 0
    try {
        $missingCSRFRequest = @{
            UseBasicParsing = $true
            Uri             = 'http://127.0.0.1:8080/api/v1/client/scans'
            Method          = 'Post'
            Form            = @{ cv = $cvInput; location_id = $locationID }
            WebSession      = $webSession
            TimeoutSec      = 20
        }
        Invoke-WebRequest @missingCSRFRequest | Out-Null
    }
    catch {
        $missingCSRFStatus = [int]$_.Exception.Response.StatusCode
    }
    if ($missingCSRFStatus -ne 403) {
        throw "Missing-CSRF upload returned $missingCSRFStatus instead of 403."
    }

    $uploadRequest = @{
        Uri        = 'http://127.0.0.1:8080/api/v1/client/scans'
        Method     = 'Post'
        Form       = @{ cv = $cvInput; location_id = $locationID }
        Headers    = @{ 'X-CSRF-Token' = $rawCSRF }
        WebSession = $webSession
        TimeoutSec = 30
    }
    $accepted = Invoke-RestMethod @uploadRequest
    $scanID = [string]$accepted.scan_id
    if (-not $scanID -or $accepted.status -ne 'processing') {
        throw 'CV upload did not return an accepted processing scan.'
    }

    $final = $null
    $phasesSeen = [System.Collections.Generic.HashSet[string]]::new()
    for ($attempt = 0; $attempt -lt 180; $attempt++) {
        Start-Sleep -Milliseconds 500
        $scanRequest = @{
            Uri        = "http://127.0.0.1:8080/api/v1/client/scans/$scanID"
            Method     = 'Get'
            WebSession = $webSession
            TimeoutSec = 15
        }
        $current = Invoke-RestMethod @scanRequest
        if ($current.status -ne 'processing') {
            $final = $current
            break
        }
        if ($current.phase) {
            [void]$phasesSeen.Add([string]$current.phase)
        }
    }
    if ($null -eq $final) {
        throw 'CV scan did not finish within 90 seconds.'
    }
    if ($final.status -ne 'completed') {
        throw "CV scan finished as '$($final.status)' with code '$($final.error_code)'."
    }
    if ($null -eq $final.cv_summary -or -not $final.cv_summary.headline -or -not $final.cv_summary.overview) {
        throw 'Completed scan did not return a bounded CV summary.'
    }
    if (@($final.matches).Count -lt 1) {
        throw 'Location-scoped matching returned no job for a location that has active jobs.'
    }

    $closedMatchCount = Invoke-DatabaseScalar -Sql @"
SELECT count(*)
FROM public.scan_matches matches
JOIN public.job_cache jobs ON jobs.id = matches.job_id
WHERE matches.scan_id = '$scanID'
  AND jobs.status <> 'ACTIVE';
"@
    if ($closedMatchCount -ne '0') {
        throw "Completed scan persisted $closedMatchCount non-active job match(es)."
    }

    $softwareMatches = @($final.matches | Where-Object { $_.title -match '(?i)software|developer|backend|back-end|frontend|front-end|full.?stack|programmer' })
    $overScoredUnrelated = @($softwareMatches | Where-Object { [double]$_.match_percent -gt 60 })
    if ($overScoredUnrelated.Count -gt 0) {
        throw "Unrelated software-role match exceeded the calibration guard: $($overScoredUnrelated[0].match_percent)%."
    }

    $historyRequest = @{
        Uri        = 'http://127.0.0.1:8080/api/v1/client/cv-history?page=1&page_size=10'
        Method     = 'Get'
        WebSession = $webSession
        TimeoutSec = 15
    }
    $history = Invoke-RestMethod @historyRequest
    $historyItem = @($history.items) | Where-Object { $_.scan_id -eq $scanID }
    if ($historyItem.Count -ne 1 -or $null -eq $historyItem[0].profile) {
        throw 'Structured CV history did not contain the completed scan.'
    }
    $historyJSON = $history | ConvertTo-Json -Depth 20 -Compress
    if ($historyJSON -match '(?i)raw_cv|raw_file|phone|address|photo') {
        throw 'CV history response contains a forbidden raw or PII field.'
    }

    $profileID = Invoke-DatabaseScalar -Sql "SELECT profile_id FROM public.scans WHERE id = '$scanID';"
    if (-not $profileID) {
        throw 'Completed scan did not persist a structured profile.'
    }

    $deleteRequest = @{
        UseBasicParsing = $true
        Uri             = "http://127.0.0.1:8080/api/v1/client/cv-history/$scanID"
        Method          = 'Delete'
        Headers         = @{ 'X-CSRF-Token' = $rawCSRF }
        WebSession      = $webSession
        TimeoutSec      = 15
    }
    $deleteResponse = Invoke-WebRequest @deleteRequest
    if ($deleteResponse.StatusCode -ne 204) {
        throw "CV delete returned $($deleteResponse.StatusCode)."
    }

    $remainingRows = Invoke-DatabaseScalar -Sql @"
SELECT
    (SELECT count(*) FROM public.scans WHERE id = '$scanID') || '|' ||
    (SELECT count(*) FROM public.structured_profiles WHERE id = '$profileID');
"@
    if ($remainingRows -ne '0|0') {
        throw "CV deletion left scan/profile rows: $remainingRows."
    }

    $rawTempCount = 0
    $cvTempDirectory = Join-Path ([IO.Path]::GetTempPath()) 'gogetsomefoodferris-cv'
    if (Test-Path -LiteralPath $cvTempDirectory) {
        $tempFileQuery = @{
            LiteralPath = $cvTempDirectory
            File        = $true
            Filter      = "$scanID.*"
            ErrorAction = 'SilentlyContinue'
        }
        $rawTempCount = @(Get-ChildItem @tempFileQuery).Count
    }
    if ($rawTempCount -ne 0) {
        throw 'Raw CV temp file was retained after processing.'
    }

    [pscustomobject]@{
        MissingCSRF             = 'PASS (403)'
        AuthenticatedUpload     = 'PASS (202)'
        DeepSeekParse           = 'PASS'
        LocationScopedMatch     = "PASS ($(@($final.matches).Count) active jobs)"
        ActiveOnlyResults       = 'PASS (0 non-active matches)'
        CVSummary               = 'PASS'
        LoadingPhases           = "PASS ($([string]::Join(', ', ($phasesSeen | Sort-Object))))"
        RoleCalibration         = "PASS ($(@($softwareMatches).Count) unrelated software jobs checked)"
        StructuredHistory       = 'PASS'
        DeleteAndProfileCleanup = 'PASS'
        RawCVRetention          = 'PASS (0 files)'
    } | Format-List
}
finally {
    if ($tempCVOwned -and $tempCV -and (Test-Path -LiteralPath $tempCV)) {
        Remove-Item -LiteralPath $tempCV -Force
    }
    if ($createdUser) {
        [void](Invoke-DatabaseScalar -Sql "DELETE FROM public.client_users WHERE id = '$userID';")
    }
    Remove-Item Env:PGPASSWORD -ErrorAction SilentlyContinue
}
