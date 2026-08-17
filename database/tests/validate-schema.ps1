$ErrorActionPreference = 'Stop'

$migrationDirectory = Join-Path $PSScriptRoot '..\migrations'
$upPath = Join-Path $migrationDirectory '000001_initial_schema.up.sql'
$downPath = Join-Path $migrationDirectory '000001_initial_schema.down.sql'
$promotionUpPath = Join-Path $migrationDirectory '000002_promotion_slides.up.sql'
$promotionDownPath = Join-Path $migrationDirectory '000002_promotion_slides.down.sql'
$adminUpPath = Join-Path $migrationDirectory '000003_admin_auth_cloudinary.up.sql'
$adminDownPath = Join-Path $migrationDirectory '000003_admin_auth_cloudinary.down.sql'
$jobLinkUpPath = Join-Path $migrationDirectory '000004_job_link_constraints.up.sql'
$jobLinkDownPath = Join-Path $migrationDirectory '000004_job_link_constraints.down.sql'
$locationUpPath = Join-Path $migrationDirectory '000005_canonical_locations.up.sql'
$locationDownPath = Join-Path $migrationDirectory '000005_canonical_locations.down.sql'
$locationResolutionUpPath = Join-Path $migrationDirectory '000006_location_resolution_and_scan_location.up.sql'
$locationResolutionDownPath = Join-Path $migrationDirectory '000006_location_resolution_and_scan_location.down.sql'
$locationNormalizationUpPath = Join-Path $migrationDirectory '000007_location_key_normalization.up.sql'
$locationNormalizationDownPath = Join-Path $migrationDirectory '000007_location_key_normalization.down.sql'
$settingsUpPath = Join-Path $migrationDirectory '000008_settings_and_crawl_requests.up.sql'
$settingsDownPath = Join-Path $migrationDirectory '000008_settings_and_crawl_requests.down.sql'
$runtimeUpPath = Join-Path $migrationDirectory '000009_admin_management_runtime.up.sql'
$runtimeDownPath = Join-Path $migrationDirectory '000009_admin_management_runtime.down.sql'
$fixturePath = Join-Path $PSScriptRoot '..\fixtures\development-job.sql'
$failures = [System.Collections.Generic.List[string]]::new()

function Assert-Condition {
    param(
        [bool]$Condition,
        [string]$Message
    )

    if (-not $Condition) {
        $failures.Add($Message)
    }
}

function Assert-Contains {
    param(
        [string]$Text,
        [string]$Pattern,
        [string]$Message
    )

    Assert-Condition -Condition ($Text -match $Pattern) -Message $Message
}

function Assert-NotContains {
    param(
        [string]$Text,
        [string]$Pattern,
        [string]$Message
    )

    Assert-Condition -Condition ($Text -notmatch $Pattern) -Message $Message
}

Assert-Condition -Condition (Test-Path -LiteralPath $upPath) -Message "Missing up migration: $upPath"
Assert-Condition -Condition (Test-Path -LiteralPath $downPath) -Message "Missing down migration: $downPath"
Assert-Condition -Condition (Test-Path -LiteralPath $promotionUpPath) -Message "Missing promotion up migration: $promotionUpPath"
Assert-Condition -Condition (Test-Path -LiteralPath $promotionDownPath) -Message "Missing promotion down migration: $promotionDownPath"
Assert-Condition -Condition (Test-Path -LiteralPath $adminUpPath) -Message "Missing admin/Cloudinary up migration: $adminUpPath"
Assert-Condition -Condition (Test-Path -LiteralPath $adminDownPath) -Message "Missing admin/Cloudinary down migration: $adminDownPath"
Assert-Condition -Condition (Test-Path -LiteralPath $jobLinkUpPath) -Message "Missing Job Link up migration: $jobLinkUpPath"
Assert-Condition -Condition (Test-Path -LiteralPath $jobLinkDownPath) -Message "Missing Job Link down migration: $jobLinkDownPath"
Assert-Condition -Condition (Test-Path -LiteralPath $locationUpPath) -Message "Missing location up migration: $locationUpPath"
Assert-Condition -Condition (Test-Path -LiteralPath $locationDownPath) -Message "Missing location down migration: $locationDownPath"
Assert-Condition -Condition (Test-Path -LiteralPath $locationResolutionUpPath) -Message "Missing location-resolution up migration: $locationResolutionUpPath"
Assert-Condition -Condition (Test-Path -LiteralPath $locationResolutionDownPath) -Message "Missing location-resolution down migration: $locationResolutionDownPath"
Assert-Condition -Condition (Test-Path -LiteralPath $locationNormalizationUpPath) -Message "Missing location-normalization up migration: $locationNormalizationUpPath"
Assert-Condition -Condition (Test-Path -LiteralPath $locationNormalizationDownPath) -Message "Missing location-normalization down migration: $locationNormalizationDownPath"
Assert-Condition -Condition (Test-Path -LiteralPath $settingsUpPath) -Message "Missing settings/crawl-request up migration: $settingsUpPath"
Assert-Condition -Condition (Test-Path -LiteralPath $settingsDownPath) -Message "Missing settings/crawl-request down migration: $settingsDownPath"
Assert-Condition -Condition (Test-Path -LiteralPath $runtimeUpPath) -Message "Missing crawler-runtime up migration: $runtimeUpPath"
Assert-Condition -Condition (Test-Path -LiteralPath $runtimeDownPath) -Message "Missing crawler-runtime down migration: $runtimeDownPath"
Assert-Condition -Condition (Test-Path -LiteralPath $fixturePath) -Message "Missing explicit development fixture: $fixturePath"

if ($failures.Count -gt 0) {
    $failures | ForEach-Object { Write-Output $_ }
    exit 1
}

$up = Get-Content -LiteralPath $upPath -Raw
$down = Get-Content -LiteralPath $downPath -Raw
$promotionUp = Get-Content -LiteralPath $promotionUpPath -Raw
$promotionDown = Get-Content -LiteralPath $promotionDownPath -Raw
$adminUp = Get-Content -LiteralPath $adminUpPath -Raw
$adminDown = Get-Content -LiteralPath $adminDownPath -Raw
$jobLinkUp = Get-Content -LiteralPath $jobLinkUpPath -Raw
$jobLinkDown = Get-Content -LiteralPath $jobLinkDownPath -Raw
$locationUp = Get-Content -LiteralPath $locationUpPath -Raw
$locationDown = Get-Content -LiteralPath $locationDownPath -Raw
$locationResolutionUp = if (Test-Path -LiteralPath $locationResolutionUpPath) { Get-Content -LiteralPath $locationResolutionUpPath -Raw } else { '' }
$locationResolutionDown = if (Test-Path -LiteralPath $locationResolutionDownPath) { Get-Content -LiteralPath $locationResolutionDownPath -Raw } else { '' }
$locationNormalizationUp = if (Test-Path -LiteralPath $locationNormalizationUpPath) { Get-Content -LiteralPath $locationNormalizationUpPath -Raw } else { '' }
$locationNormalizationDown = if (Test-Path -LiteralPath $locationNormalizationDownPath) { Get-Content -LiteralPath $locationNormalizationDownPath -Raw } else { '' }
$settingsUp = if (Test-Path -LiteralPath $settingsUpPath) { Get-Content -LiteralPath $settingsUpPath -Raw } else { '' }
$settingsDown = if (Test-Path -LiteralPath $settingsDownPath) { Get-Content -LiteralPath $settingsDownPath -Raw } else { '' }
$runtimeUp = if (Test-Path -LiteralPath $runtimeUpPath) { Get-Content -LiteralPath $runtimeUpPath -Raw } else { '' }
$runtimeDown = if (Test-Path -LiteralPath $runtimeDownPath) { Get-Content -LiteralPath $runtimeDownPath -Raw } else { '' }
$fixture = Get-Content -LiteralPath $fixturePath -Raw

Assert-Contains -Text $up -Pattern '(?im)^BEGIN;\s*$' -Message 'Up migration must start a transaction.'
Assert-Contains -Text $up -Pattern '(?im)^COMMIT;\s*$' -Message 'Up migration must commit its transaction.'
Assert-Contains -Text $up -Pattern '(?im)CREATE\s+EXTENSION\s+IF\s+NOT\s+EXISTS\s+pgcrypto' -Message 'Up migration must enable pgcrypto for UUID defaults.'

$requiredColumns = @{
    job_sources = @('id', 'source_key', 'display_name', 'base_url', 'source_type', 'approval_status', 'robots_url', 'terms_url', 'approved_at', 'approved_by', 'created_at', 'updated_at')
    source_crawl_runs = @('id', 'source_id', 'run_status', 'started_at', 'finished_at', 'pages_seen', 'jobs_seen', 'jobs_created', 'jobs_updated', 'jobs_missing', 'error_code')
    job_cache = @('id', 'source_id', 'source_job_key', 'content_hash', 'title', 'company', 'location_text', 'latitude', 'longitude', 'role', 'required_skills', 'preferred_skills', 'seniority', 'minimum_experience_years', 'domains', 'employment_type', 'work_mode', 'salary_min', 'salary_max', 'salary_currency', 'salary_period', 'salary_source_text', 'original_url', 'status', 'last_seen_at', 'missing_healthy_cycles', 'created_at', 'updated_at')
    structured_profiles = @('id', 'roles', 'skills', 'years_of_experience', 'seniority', 'domains', 'education', 'certifications', 'schema_version', 'parser_model', 'created_at', 'updated_at', 'expires_at')
    scans = @('id', 'status', 'profile_id', 'location_text', 'latitude', 'longitude', 'radius_km', 'error_code', 'created_at', 'updated_at', 'expires_at')
    scan_matches = @('id', 'scan_id', 'job_id', 'required_skills_points', 'role_relevance_points', 'experience_points', 'seniority_points', 'preferred_skills_domain_points', 'match_percent', 'distance_km', 'created_at')
}

foreach ($table in $requiredColumns.Keys) {
    $tablePattern = "(?is)CREATE\s+TABLE\s+public\.$table\s*\((.*?)(?=\n\);)"
    $tableMatch = [regex]::Match($up, $tablePattern)
    Assert-Condition -Condition $tableMatch.Success -Message "Missing table definition: public.$table"

    if ($tableMatch.Success) {
        $tableBody = $tableMatch.Groups[1].Value
        foreach ($column in $requiredColumns[$table]) {
            Assert-Contains -Text $tableBody -Pattern "(?im)^\s*$column\s+" -Message "Missing column $column on public.$table"
        }
    }
}

Assert-Contains -Text $up -Pattern "(?is)approval_status\s+text.*CHECK\s*\(.*'REVIEW'.*'ACTIVE'.*'DISABLED'" -Message 'Source approval states must be constrained to REVIEW, ACTIVE, and DISABLED.'
Assert-Contains -Text $up -Pattern "(?is)run_status\s+text.*CHECK\s*\(.*'HEALTHY'.*'SOURCE_ERROR'.*'PARSER_ERROR'.*'ANOMALY'" -Message 'Crawl run statuses must distinguish healthy and non-missing failures.'
Assert-Contains -Text $up -Pattern "(?is)status\s+text.*CHECK\s*\(.*'ACTIVE'.*'VERIFYING'.*'CLOSED'.*'EXPIRED'.*'DISABLED'" -Message 'Job lifecycle states must be constrained.'
Assert-Contains -Text $up -Pattern "(?is)status\s+text.*CHECK\s*\(.*'RECEIVED'.*'PARSING'.*'MATCHING'.*'COMPLETED'.*'FAILED'" -Message 'Scan lifecycle states must be constrained.'
Assert-Contains -Text $up -Pattern '(?is)radius_km\s+numeric\(.*CHECK\s*\(.*radius_km\s*>\s*0' -Message 'Scan radius must be positive and measured in kilometers.'
Assert-Contains -Text $up -Pattern "(?is)job_cache_salary_currency_required_check\s+CHECK\s*\(.*salary_min\s+IS\s+NULL.*salary_currency\s+IS\s+NOT\s+NULL" -Message 'Parsed salary amounts must retain the source currency.'
Assert-Contains -Text $up -Pattern "(?is)scans_profile_ready_check\s+CHECK\s*\(\s*status\s+NOT\s+IN\s*\(\s*'MATCHING'.*'COMPLETED'.*profile_id\s+IS\s+NOT\s+NULL" -Message 'A scan may wait for parsing without a profile but must have one before matching or completion.'
Assert-Contains -Text $up -Pattern '(?is)CREATE\s+(OR\s+REPLACE\s+)?FUNCTION\s+public\.is_structured_record_array' -Message 'Structured profile JSONB must use a database-side shape validator.'
Assert-Contains -Text $up -Pattern '(?is)structured_profiles_education_shape_check\s+CHECK\s*\(\s*public\.is_structured_record_array' -Message 'Education JSONB must use the structured-record validator.'
Assert-Contains -Text $up -Pattern '(?is)structured_profiles_certifications_shape_check\s+CHECK\s*\(\s*public\.is_structured_record_array' -Message 'Certification JSONB must use the structured-record validator.'
Assert-Contains -Text $up -Pattern "(?is)CREATE\s+VIEW\s+public\.active_job_cache.*FROM\s+public\.job_cache.*JOIN\s+public\.job_sources.*approval_status\s*=\s*'ACTIVE'.*jobs\.status\s*=\s*'ACTIVE'" -Message 'Public active jobs must be exposed through a source-approval-aware view.'
Assert-Contains -Text $up -Pattern '(?im)CREATE\s+INDEX\s+job_cache_missing_cycles_idx' -Message 'Job reconciliation needs an index for missing healthy cycles.'
Assert-Contains -Text $up -Pattern '(?im)CREATE\s+INDEX\s+scan_matches_job_idx' -Message 'Job foreign-key lookups need a leading job_id index.'
Assert-Contains -Text $up -Pattern '(?is)required_skills_points\s+numeric\(.*CHECK\s*\(.*required_skills_points\s+BETWEEN\s+0\s+AND\s+35' -Message 'Required-skills score must be bounded at 35 points.'
Assert-Contains -Text $up -Pattern '(?is)role_relevance_points\s+numeric\(.*CHECK\s*\(.*role_relevance_points\s+BETWEEN\s+0\s+AND\s+25' -Message 'Role-relevance score must be bounded at 25 points.'
Assert-Contains -Text $up -Pattern '(?is)experience_points\s+numeric\(.*CHECK\s*\(.*experience_points\s+BETWEEN\s+0\s+AND\s+15' -Message 'Experience score must be bounded at 15 points.'
Assert-Contains -Text $up -Pattern '(?is)seniority_points\s+numeric\(.*CHECK\s*\(.*seniority_points\s+BETWEEN\s+0\s+AND\s+15' -Message 'Seniority score must be bounded at 15 points.'
Assert-Contains -Text $up -Pattern '(?is)preferred_skills_domain_points\s+numeric\(.*CHECK\s*\(.*preferred_skills_domain_points\s+BETWEEN\s+0\s+AND\s+10' -Message 'Preferred-skills/domain score must be bounded at 10 points.'
Assert-Contains -Text $up -Pattern '(?is)match_percent\s+numeric\(.*CHECK\s*\(.*match_percent\s+BETWEEN\s+0\s+AND\s+100' -Message 'Total match percent must be bounded from 0 to 100.'
Assert-Contains -Text $up -Pattern '(?is)scan_matches_score_sum_check\s+CHECK\s*\(.*match_percent\s*=\s*required_skills_points.*preferred_skills_domain_points' -Message 'Total match percent must equal the five stored weighted components.'
Assert-Contains -Text $up -Pattern '(?is)distance_km\s+numeric\(.*scan_matches_distance_check\s+CHECK\s*\(\s*distance_km\s+IS\s+NULL\s+OR\s+distance_km\s+>=\s+0' -Message 'Stored match distance must be numeric and nonnegative in kilometers.'
Assert-Contains -Text $up -Pattern '(?is)UNIQUE\s*\(\s*source_id\s*,\s*source_job_key\s*\)' -Message 'Job identity must be unique within a source.'
Assert-Contains -Text $up -Pattern '(?is)UNIQUE\s*\(\s*scan_id\s*,\s*job_id\s*\)' -Message 'A scan must not contain duplicate job matches.'

Assert-Contains -Text $promotionUp -Pattern '(?im)^BEGIN;\s*$' -Message 'Promotion up migration must start a transaction.'
Assert-Contains -Text $promotionUp -Pattern '(?im)^COMMIT;\s*$' -Message 'Promotion up migration must commit its transaction.'
Assert-Contains -Text $promotionDown -Pattern '(?im)^BEGIN;\s*$' -Message 'Promotion down migration must start a transaction.'
Assert-Contains -Text $promotionDown -Pattern '(?im)^COMMIT;\s*$' -Message 'Promotion down migration must commit its transaction.'

$promotionTableMatch = [regex]::Match($promotionUp, '(?is)CREATE\s+TABLE\s+public\.promotion_slides\s*\((.*?)(?=\n\);)')
Assert-Condition -Condition $promotionTableMatch.Success -Message 'Missing table definition: public.promotion_slides'
if ($promotionTableMatch.Success) {
    $promotionTableBody = $promotionTableMatch.Groups[1].Value
    foreach ($column in @('id', 'slot', 'image_bytes', 'mime_type', 'content_hash', 'alt_text', 'eyebrow', 'title', 'body', 'target_url', 'is_active', 'created_at', 'updated_at')) {
        Assert-Contains -Text $promotionTableBody -Pattern "(?im)^\s*$column\s+" -Message "Missing column $column on public.promotion_slides"
    }
}
Assert-Contains -Text $promotionUp -Pattern '(?is)slot\s+smallint.*UNIQUE.*CHECK\s*\(\s*slot\s+BETWEEN\s+1\s+AND\s+3' -Message 'Promotion slots must be unique and constrained to 1..3.'
Assert-Contains -Text $promotionUp -Pattern "(?is)mime_type\s+text.*CHECK\s*\(.*'image/png'.*'image/jpeg'.*'image/webp'" -Message 'Promotion MIME types must be restricted to PNG, JPEG, or WebP.'
Assert-Contains -Text $promotionUp -Pattern "(?is)content_hash\s+text.*CHECK\s*\(.*[0-9a-f].*64" -Message 'Promotion content hashes must be bounded lowercase SHA-256 hex.'
Assert-Contains -Text $promotionUp -Pattern '(?is)alt_text\s+text.*CHECK\s*\(.*char_length.*>\s*0.*char_length.*<=\s*180' -Message 'Promotion alt text must be non-empty and bounded.'
Assert-Contains -Text $promotionUp -Pattern "(?is)target_url\s+text.*CHECK\s*\(.*target_url\s+IS\s+NULL.*https?" -Message 'Promotion target URLs must be optional HTTP(S) URLs.'
Assert-Contains -Text $promotionUp -Pattern '(?im)CREATE\s+INDEX\s+promotion_slides_active_slot_idx' -Message 'Promotion reads need an active-slot index.'
Assert-NotContains -Text $promotionUp -Pattern '(?im)^\s*(INSERT|COPY)\s+' -Message 'Promotion migration must not seed or load image data.'
Assert-Contains -Text $promotionDown -Pattern '(?im)^DROP\s+TABLE\s+IF\s+EXISTS\s+public\.promotion_slides;' -Message 'Promotion down migration must remove only the promotion table.'

Assert-Contains -Text $adminUp -Pattern '(?im)^BEGIN;\s*$' -Message 'Admin/Cloudinary up migration must start a transaction.'
Assert-Contains -Text $adminUp -Pattern '(?im)^COMMIT;\s*$' -Message 'Admin/Cloudinary up migration must commit its transaction.'
Assert-Contains -Text $adminDown -Pattern '(?im)^BEGIN;\s*$' -Message 'Admin/Cloudinary down migration must start a transaction.'
Assert-Contains -Text $adminDown -Pattern '(?im)^COMMIT;\s*$' -Message 'Admin/Cloudinary down migration must commit its transaction.'
foreach ($table in @('admin_users', 'admin_sessions', 'admin_audit_events')) {
    Assert-Contains -Text $adminUp -Pattern "(?im)CREATE\s+TABLE\s+public\.$table" -Message "Missing admin table: public.$table"
    Assert-Contains -Text $adminDown -Pattern "(?im)DROP\s+TABLE\s+IF\s+EXISTS\s+public\.$table" -Message "Admin down migration must drop public.$table."
}
foreach ($column in @('storage_provider', 'cloudinary_public_id', 'cloudinary_secure_url', 'cloudinary_asset_id')) {
    Assert-Contains -Text $adminUp -Pattern "(?im)ADD\s+COLUMN\s+$column" -Message "Missing Cloudinary promotion column: $column"
    Assert-Contains -Text $adminDown -Pattern "(?im)DROP\s+COLUMN\s+$column" -Message "Cloudinary down migration must remove column: $column"
}
Assert-Contains -Text $adminUp -Pattern '(?is)storage_provider\s+IN\s*\(.*DATABASE.*CLOUDINARY' -Message 'Promotion storage provider must be constrained.'
Assert-Contains -Text $adminUp -Pattern '(?is)cloudinary_secure_url.*https://' -Message 'Cloudinary delivery URLs must be HTTPS.'
Assert-NotContains -Text $adminUp -Pattern '(?im)^\s*(INSERT|COPY)\s+' -Message 'Admin/Cloudinary migration must not seed data.'
Assert-Contains -Text $jobLinkUp -Pattern '(?im)^BEGIN;\s*$' -Message 'Job Link up migration must start a transaction.'
Assert-Contains -Text $jobLinkUp -Pattern '(?im)^COMMIT;\s*$' -Message 'Job Link up migration must commit its transaction.'
Assert-Contains -Text $jobLinkDown -Pattern '(?im)^BEGIN;\s*$' -Message 'Job Link down migration must start a transaction.'
Assert-Contains -Text $jobLinkDown -Pattern '(?im)^COMMIT;\s*$' -Message 'Job Link down migration must commit its transaction.'
Assert-Contains -Text $jobLinkUp -Pattern '(?is)CREATE\s+UNIQUE\s+INDEX\s+job_sources_active_base_url_uidx.*approval_status\s*<>\s*''DISABLED''' -Message 'Enabled Job Links must be unique by normalized URL.'
Assert-Contains -Text $jobLinkDown -Pattern '(?im)^DROP\s+INDEX\s+IF\s+EXISTS\s+public\.job_sources_active_base_url_uidx;' -Message 'Job Link down migration must remove its unique index.'
Assert-Contains -Text $locationUp -Pattern '(?im)^BEGIN;\s*$' -Message 'Location up migration must start a transaction.'
Assert-Contains -Text $locationUp -Pattern '(?im)^COMMIT;\s*$' -Message 'Location up migration must commit its transaction.'
Assert-Contains -Text $locationUp -Pattern '(?is)CREATE\s+TABLE\s+public\.locations' -Message 'Location migration must create public.locations.'
Assert-Contains -Text $locationUp -Pattern '(?im)^\s*ADD\s+COLUMN\s+location_id\s+uuid' -Message 'Location migration must add job_cache.location_id.'
Assert-Contains -Text $locationUp -Pattern '(?is)ON\s+DELETE\s+SET\s+NULL' -Message 'Removing a canonical location must preserve the job row.'
Assert-Contains -Text $locationUp -Pattern '(?is)COALESCE\s*\(\s*locations\.display_name.*jobs\.location_text' -Message 'Public jobs must prefer the canonical location while retaining source text.'
Assert-Contains -Text $locationDown -Pattern '(?im)^DROP\s+TABLE\s+IF\s+EXISTS\s+public\.locations;' -Message 'Location down migration must remove public.locations.'
Assert-Contains -Text $locationResolutionUp -Pattern '(?im)^BEGIN;\s*$' -Message 'Location-resolution up migration must start a transaction.'
Assert-Contains -Text $locationResolutionUp -Pattern '(?im)^COMMIT;\s*$' -Message 'Location-resolution up migration must commit its transaction.'
Assert-Contains -Text $locationResolutionUp -Pattern '(?im)ADD\s+COLUMN\s+canonical_key\s+text' -Message 'Canonical locations need a stable lookup key.'
Assert-Contains -Text $locationResolutionUp -Pattern '(?im)ALTER\s+TABLE\s+public\.locations\s+ALTER\s+COLUMN\s+canonical_key\s+SET\s+NOT NULL' -Message 'Canonical location keys must be required after backfill.'
Assert-Contains -Text $locationResolutionUp -Pattern '(?im)latitude\s+numeric' -Message 'Canonical locations need latitude for kilometer filtering.'
Assert-Contains -Text $locationResolutionUp -Pattern '(?im)longitude\s+numeric' -Message 'Canonical locations need longitude for kilometer filtering.'
Assert-Contains -Text $locationResolutionUp -Pattern '(?im)CREATE\s+TABLE\s+public\.location_aliases' -Message 'Location aliases must be persisted.'
Assert-Contains -Text $locationResolutionUp -Pattern '(?im)normalized_text\s+text\s+NOT NULL' -Message 'Location aliases need a normalized lookup key.'
Assert-Contains -Text $locationResolutionUp -Pattern '(?im)ALTER\s+TABLE\s+public\.scans\s+ADD\s+COLUMN\s+location_id\s+uuid' -Message 'Scans need canonical location identity.'
Assert-Contains -Text $locationResolutionUp -Pattern '(?im)scans_location_fk' -Message 'Scans must reference canonical locations.'
Assert-Contains -Text $locationResolutionUp -Pattern '(?im)CREATE\s+INDEX\s+scans_location_idx' -Message 'Scan location filtering needs an index.'
Assert-Contains -Text $locationResolutionDown -Pattern '(?im)^BEGIN;\s*$' -Message 'Location-resolution down migration must start a transaction.'
Assert-Contains -Text $locationResolutionDown -Pattern '(?im)^COMMIT;\s*$' -Message 'Location-resolution down migration must commit its transaction.'
Assert-Contains -Text $locationResolutionDown -Pattern '(?im)DROP\s+TABLE\s+IF\s+EXISTS\s+public\.location_aliases;' -Message 'Location-resolution down migration must remove aliases.'
Assert-Contains -Text $locationResolutionDown -Pattern '(?im)DROP\s+INDEX\s+IF\s+EXISTS\s+public\.scans_location_idx;' -Message 'Location-resolution down migration must remove the scan location index.'
Assert-Contains -Text $locationNormalizationUp -Pattern '(?im)^BEGIN;\s*$' -Message 'Location-normalization up migration must start a transaction.'
Assert-Contains -Text $locationNormalizationUp -Pattern '(?im)^COMMIT;\s*$' -Message 'Location-normalization up migration must commit its transaction.'
Assert-Contains -Text $locationNormalizationUp -Pattern '(?im)CREATE\s+EXTENSION\s+IF\s+NOT\s+EXISTS\s+unaccent' -Message 'Location normalization must enable the unaccent extension.'
Assert-Contains -Text $locationNormalizationUp -Pattern '(?im)UPDATE\s+public\.locations' -Message 'Location normalization must backfill existing canonical keys.'
Assert-Contains -Text $locationNormalizationDown -Pattern '(?im)^BEGIN;\s*$' -Message 'Location-normalization down migration must start a transaction.'
Assert-Contains -Text $locationNormalizationDown -Pattern '(?im)^COMMIT;\s*$' -Message 'Location-normalization down migration must commit its transaction.'

foreach ($settingsMigration in @(@{ Text = $settingsUp; Name = 'Settings up' }, @{ Text = $settingsDown; Name = 'Settings down' })) {
    Assert-Contains -Text $settingsMigration.Text -Pattern '(?im)^BEGIN;\s*$' -Message "$($settingsMigration.Name) migration must start a transaction."
    Assert-Contains -Text $settingsMigration.Text -Pattern '(?im)^COMMIT;\s*$' -Message "$($settingsMigration.Name) migration must commit its transaction."
}

foreach ($table in @('app_settings', 'crawl_requests')) {
    Assert-Contains -Text $settingsUp -Pattern "(?im)CREATE\s+TABLE\s+public\.$table" -Message "Missing settings table: public.$table"
    Assert-Contains -Text $settingsDown -Pattern "(?im)DROP\s+TABLE\s+IF\s+EXISTS\s+public\.$table" -Message "Settings down migration must drop public.$table."
}
foreach ($column in @('setting_key', 'setting_group', 'setting_value', 'updated_at', 'updated_by')) {
    Assert-Contains -Text $settingsUp -Pattern "(?im)^\s*$column\s+" -Message "Missing app_settings column: $column"
}
foreach ($column in @('id', 'source_id', 'status', 'requested_by', 'requested_at', 'started_at', 'finished_at', 'source_run_id', 'error_code')) {
    Assert-Contains -Text $settingsUp -Pattern "(?im)^\s*$column\s+" -Message "Missing crawl_requests column: $column"
}
Assert-Contains -Text $settingsUp -Pattern "(?is)status\s+text.*CHECK\s*\(.*'PENDING'.*'RUNNING'.*'COMPLETED'.*'FAILED'" -Message 'Crawl request statuses must be bounded.'
Assert-Contains -Text $settingsUp -Pattern '(?im)CREATE\s+UNIQUE\s+INDEX\s+crawl_requests_active_source_uidx' -Message 'Only one pending or running crawl request may exist per source.'
Assert-Contains -Text $settingsUp -Pattern '(?is)FOREIGN\s+KEY\s*\(\s*source_id\s*\).*job_sources.*ON\s+DELETE\s+CASCADE' -Message 'Crawl requests must be removed with a hard-deleted Job Link.'
Assert-NotContains -Text $settingsUp -Pattern '(?im)^\s*(INSERT|COPY)\s+' -Message 'Settings migration must not seed runtime rows.'
Assert-NotContains -Text $settingsDown -Pattern '(?im)^\s*(INSERT|COPY)\s+' -Message 'Settings down migration must not load runtime data.'

foreach ($runtimeMigration in @(@{ Text = $runtimeUp; Name = 'Crawler-runtime up' }, @{ Text = $runtimeDown; Name = 'Crawler-runtime down' })) {
    Assert-Contains -Text $runtimeMigration.Text -Pattern '(?im)^BEGIN;\s*$' -Message "$($runtimeMigration.Name) migration must start a transaction."
    Assert-Contains -Text $runtimeMigration.Text -Pattern '(?im)^COMMIT;\s*$' -Message "$($runtimeMigration.Name) migration must commit its transaction."
}
$runtimeTableMatch = [regex]::Match($runtimeUp, '(?is)CREATE\s+TABLE\s+public\.crawler_runtime\s*\((.*?)(?=\n\);)')
Assert-Condition -Condition $runtimeTableMatch.Success -Message 'Missing table definition: public.crawler_runtime'
if ($runtimeTableMatch.Success) {
    $runtimeTableBody = $runtimeTableMatch.Groups[1].Value
    foreach ($column in @('runtime_key', 'status', 'last_heartbeat_at', 'last_cycle_started_at', 'last_cycle_finished_at', 'next_cycle_at', 'current_source_key', 'last_error_code', 'updated_at')) {
        Assert-Contains -Text $runtimeTableBody -Pattern "(?im)^\s*$column\s+" -Message "Missing crawler_runtime column: $column"
    }
}
Assert-Contains -Text $runtimeUp -Pattern "(?is)runtime_key\s+text\s+PRIMARY\s+KEY.*CHECK\s*\(\s*runtime_key\s*=\s*'default'" -Message 'Crawler runtime must be a singleton default row.'
Assert-Contains -Text $runtimeUp -Pattern "(?is)status\s+text.*CHECK\s*\(.*'OFFLINE'.*'IDLE'.*'RUNNING'.*'ERROR'" -Message 'Crawler runtime statuses must be constrained.'
Assert-Contains -Text $runtimeUp -Pattern "(?is)INSERT\s+INTO\s+public\.crawler_runtime.*'default'.*'OFFLINE'" -Message 'Crawler runtime migration must seed its singleton row.'
Assert-Contains -Text $runtimeDown -Pattern '(?im)^DROP\s+TABLE\s+IF\s+EXISTS\s+public\.crawler_runtime;' -Message 'Crawler-runtime down migration must remove only the runtime table.'
Assert-Contains -Text $fixture -Pattern '(?im)development-only' -Message 'Development fixture must be explicitly marked as development-only.'
Assert-Contains -Text $fixture -Pattern "(?im)development-fixture" -Message 'Development fixture must use an unmistakable source key.'
Assert-Contains -Text $fixture -Pattern "(?im)'DISABLED'" -Message 'Development fixture job must remain disabled from public results.'

foreach ($forbiddenPattern in @('(?i)raw[_ ]?cv', '(?i)raw[_ ]?jd', '(?i)raw[_ ]?html', '(?i)full[_ ]description', '(?i)resume[_ ]content', '(?im)^\s*(INSERT|COPY)\s+')) {
    Assert-NotContains -Text $up -Pattern $forbiddenPattern -Message "Up migration contains forbidden raw or data-loading token: $forbiddenPattern"
    Assert-NotContains -Text $down -Pattern $forbiddenPattern -Message "Down migration contains forbidden raw or data-loading token: $forbiddenPattern"
}

Assert-NotContains -Text $down -Pattern '(?i)DROP\s+EXTENSION.*pgcrypto' -Message 'Down migration must leave the shared pgcrypto extension installed.'
Assert-Contains -Text $down -Pattern '(?im)^DROP\s+VIEW\s+IF\s+EXISTS\s+public\.active_job_cache;' -Message 'Down migration must remove the public active-job view before its tables.'
Assert-Contains -Text $down -Pattern '(?im)^DROP\s+FUNCTION\s+IF\s+EXISTS\s+public\.is_structured_record_array' -Message 'Down migration must remove the structured JSON validator after dependent tables.'
Assert-Contains -Text $down -Pattern '(?im)^BEGIN;\s*$' -Message 'Down migration must start a transaction.'
Assert-Contains -Text $down -Pattern '(?im)^COMMIT;\s*$' -Message 'Down migration must commit its transaction.'

$dropOrder = @('scan_matches', 'scans', 'structured_profiles', 'job_cache', 'source_crawl_runs', 'job_sources')
$previousIndex = -1
foreach ($table in $dropOrder) {
    $index = $down.IndexOf("DROP TABLE IF EXISTS public.$table", [System.StringComparison]::OrdinalIgnoreCase)
    Assert-Condition -Condition ($index -ge 0) -Message "Down migration must drop public.$table."
    if ($index -ge 0) {
        Assert-Condition -Condition ($index -gt $previousIndex) -Message "Down migration drops public.$table before a dependent table."
        $previousIndex = $index
    }
}

if ($failures.Count -gt 0) {
    Write-Output ("Schema contract failed with {0} issue(s):" -f $failures.Count)
    $failures | ForEach-Object { Write-Output " - $_" }
    exit 1
}

Write-Output 'Schema contract passed.'
Write-Output ("Tables checked: {0}, public.promotion_slides" -f ($requiredColumns.Keys -join ', '))
Write-Output 'Privacy, score bounds, lifecycle states, promotion bounds, uniqueness, and rollback order checked.'
