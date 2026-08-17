use crate::reconcile::RunStatus;
use crate::scope::{hosts_are_equivalent, is_public_ip, ScopeError, SourceScope};
use chrono::{DateTime, NaiveDate, Utc};
use serde_json::{Map, Value};
use sha2::{Digest, Sha256};
use spider::{
    client::Client,
    configuration::RedirectPolicy,
    fetch_engine::{EngineError, EngineRequest, EngineResponse, HttpFetchEngine},
    packages::robotparser::parser::RobotFileParser,
    page::Page,
    website::{CrawlStatus, Website},
};
use std::{collections::HashSet, net::SocketAddr};
use thiserror::Error;
use url::Url;

#[derive(Debug, Clone)]
pub struct StructuredJobObservation {
    pub source_job_key: String,
    pub content_hash: String,
    pub title: String,
    pub company: String,
    pub location_text: String,
    pub latitude: Option<f64>,
    pub longitude: Option<f64>,
    pub role: String,
    pub required_skills: Vec<String>,
    pub preferred_skills: Vec<String>,
    pub seniority: String,
    pub minimum_experience_years: Option<f64>,
    pub domains: Vec<String>,
    pub employment_type: String,
    pub work_mode: String,
    pub salary_min: Option<f64>,
    pub salary_max: Option<f64>,
    pub salary_currency: Option<String>,
    pub salary_period: Option<String>,
    pub salary_source_text: Option<String>,
    pub original_url: String,
    pub source_declared_closed: bool,
}

impl StructuredJobObservation {
    pub fn validate(&self) -> Result<(), AdapterError> {
        let Ok(original_url) = Url::parse(&self.original_url) else {
            return Err(AdapterError::Rejected);
        };
        let required = [
            &self.source_job_key,
            &self.content_hash,
            &self.title,
            &self.company,
            &self.location_text,
            &self.role,
            &self.seniority,
            &self.employment_type,
            &self.original_url,
        ];
        if required.iter().any(|value| value.trim().is_empty())
            || self.content_hash.len() != 64
            || !self
                .content_hash
                .chars()
                .all(|value| value.is_ascii_hexdigit() && !value.is_ascii_uppercase())
            || !matches!(self.work_mode.as_str(), "REMOTE" | "HYBRID" | "ONSITE")
            || !matches!(original_url.scheme(), "http" | "https")
            || (self.salary_min.is_some() || self.salary_max.is_some())
                && self
                    .salary_currency
                    .as_deref()
                    .unwrap_or("")
                    .trim()
                    .is_empty()
            || self.salary_min.is_some_and(|value| value < 0.0)
            || self.salary_max.is_some_and(|value| value < 0.0)
            || self
                .salary_min
                .zip(self.salary_max)
                .is_some_and(|(minimum, maximum)| minimum > maximum)
            || self.latitude.is_some() != self.longitude.is_some()
            || self
                .latitude
                .is_some_and(|value| !value.is_finite() || !(-90.0..=90.0).contains(&value))
            || self
                .longitude
                .is_some_and(|value| !value.is_finite() || !(-180.0..=180.0).contains(&value))
        {
            return Err(AdapterError::Rejected);
        }
        Ok(())
    }
}

#[derive(Debug, Error)]
pub enum AdapterError {
    #[error("source adapter is not configured")]
    NotConfigured,
    #[error("source adapter rejected page")]
    Rejected,
}

#[derive(Debug, Clone)]
pub struct ObservationBatch {
    pub observations: Vec<StructuredJobObservation>,
    pub authoritative: bool,
}

pub trait PageAdapter: Send + Sync {
    fn observe(&self, page: &Page) -> Result<ObservationBatch, AdapterError>;
}

#[derive(Debug, Default, Clone, Copy)]
pub struct NoopAdapter;

impl PageAdapter for NoopAdapter {
    fn observe(&self, _page: &Page) -> Result<ObservationBatch, AdapterError> {
        Err(AdapterError::NotConfigured)
    }
}

#[derive(Debug, Default, Clone, Copy)]
pub struct JsonLdJobPostingAdapter;

impl PageAdapter for JsonLdJobPostingAdapter {
    fn observe(&self, page: &Page) -> Result<ObservationBatch, AdapterError> {
        parse_job_posting_html(&page.get_html(), page.get_url())
    }
}

fn parse_job_posting_html(
    html: &str,
    original_url: &str,
) -> Result<ObservationBatch, AdapterError> {
    let mut postings: Vec<Map<String, Value>> = Vec::new();
    for body in json_ld_script_bodies(html) {
        let Ok(value) = serde_json::from_str::<Value>(body) else {
            continue;
        };
        collect_job_postings(&value, &mut postings);
    }
    if postings.is_empty() {
        return Ok(ObservationBatch {
            observations: Vec::new(),
            authoritative: false,
        });
    }

    let mut seen_keys = HashSet::new();
    let mut observations = Vec::with_capacity(postings.len());
    for posting in postings {
        let observation = build_job_observation(&posting, original_url)?;
        if seen_keys.insert(observation.source_job_key.clone()) {
            observations.push(observation);
        }
    }
    Ok(ObservationBatch {
        observations,
        authoritative: true,
    })
}

fn json_ld_script_bodies(html: &str) -> Vec<&str> {
    let lower = html.to_ascii_lowercase();
    let mut bodies = Vec::new();
    let mut cursor = 0;
    while let Some(relative_start) = lower[cursor..].find("<script") {
        let start = cursor + relative_start;
        let Some(relative_tag_end) = lower[start..].find('>') else {
            break;
        };
        let tag_end = start + relative_tag_end;
        let tag = &lower[start..=tag_end];
        if tag.contains("application/ld+json") {
            let body_start = tag_end + 1;
            if let Some(relative_body_end) = lower[body_start..].find("</script") {
                let body_end = body_start + relative_body_end;
                bodies.push(html[body_start..body_end].trim());
                cursor = body_end + 8;
                continue;
            }
        }
        cursor = tag_end + 1;
    }
    bodies
}

fn collect_job_postings(value: &Value, postings: &mut Vec<Map<String, Value>>) {
    match value {
        Value::Array(values) => values
            .iter()
            .for_each(|value| collect_job_postings(value, postings)),
        Value::Object(object) => {
            if is_job_posting(object) {
                postings.push(object.clone());
            }
            if let Some(graph) = object.get("@graph") {
                collect_job_postings(graph, postings);
            }
        }
        _ => {}
    }
}

fn is_job_posting(object: &Map<String, Value>) -> bool {
    match object.get("@type") {
        Some(Value::String(value)) => value.eq_ignore_ascii_case("JobPosting"),
        Some(Value::Array(values)) => values.iter().any(|value| {
            value
                .as_str()
                .is_some_and(|value| value.eq_ignore_ascii_case("JobPosting"))
        }),
        _ => false,
    }
}

fn build_job_observation(
    object: &Map<String, Value>,
    original_url: &str,
) -> Result<StructuredJobObservation, AdapterError> {
    let title = required_text(object.get("title"))?;
    let company = object
        .get("hiringOrganization")
        .and_then(value_text)
        .ok_or(AdapterError::Rejected)?;
    let location_text = location_text(object.get("jobLocation")).ok_or(AdapterError::Rejected)?;
    let (latitude, longitude) = location_coordinates(object.get("jobLocation"));
    let role = object
        .get("role")
        .and_then(value_text)
        .unwrap_or_else(|| title.clone());
    let experience_value = object.get("experienceRequirements");
    let employment_type = object
        .get("employmentType")
        .and_then(value_text)
        .map(normalize_employment_type)
        .unwrap_or_else(|| "UNSPECIFIED".to_owned());
    let description = object
        .get("description")
        .and_then(value_text)
        .unwrap_or_default();
    let job_location_type = object
        .get("jobLocationType")
        .and_then(value_text)
        .unwrap_or_default();
    let work_mode = derive_work_mode(&format!("{job_location_type} {description}"));
    let seniority = derive_seniority(
        experience_value
            .and_then(value_text)
            .as_deref()
            .unwrap_or_default(),
    );
    let minimum_experience_years = experience_years(experience_value);
    let domains = object
        .get("industry")
        .and_then(value_list)
        .or_else(|| object.get("occupationalCategory").and_then(value_list))
        .unwrap_or_default();
    let required_skills = object
        .get("skills")
        .and_then(value_list)
        .unwrap_or_default();
    let (salary_min, salary_max, salary_currency, salary_period) =
        parse_salary(object.get("baseSalary"));
    let source_job_key = object
        .get("identifier")
        .and_then(value_text)
        .unwrap_or_else(|| original_url.to_owned());
    let content_hash = hash_json(object)?;
    let source_declared_closed = object
        .get("validThrough")
        .and_then(value_text)
        .is_some_and(|value| is_expired(&value));

    let observation = StructuredJobObservation {
        source_job_key,
        content_hash,
        title,
        company,
        location_text,
        latitude,
        longitude,
        role,
        required_skills,
        preferred_skills: Vec::new(),
        seniority,
        minimum_experience_years,
        domains,
        employment_type,
        work_mode,
        salary_min,
        salary_max,
        salary_currency,
        salary_period,
        salary_source_text: None,
        original_url: original_url.to_owned(),
        source_declared_closed,
    };
    observation.validate()?;
    Ok(observation)
}

fn required_text(value: Option<&Value>) -> Result<String, AdapterError> {
    value
        .and_then(value_text)
        .filter(|value| !value.trim().is_empty())
        .ok_or(AdapterError::Rejected)
}

fn value_text(value: &Value) -> Option<String> {
    match value {
        Value::String(value) => non_empty(value),
        Value::Number(value) => Some(value.to_string()),
        Value::Array(values) => {
            let values = values.iter().filter_map(value_text).collect::<Vec<_>>();
            (!values.is_empty()).then(|| values.join(", "))
        }
        Value::Object(object) => ["name", "value", "valueReference", "@id"]
            .iter()
            .find_map(|key| object.get(*key).and_then(value_text)),
        _ => None,
    }
}

fn value_list(value: &Value) -> Option<Vec<String>> {
    let raw_values = match value {
        Value::Array(values) => values.iter().filter_map(value_text).collect::<Vec<_>>(),
        _ => value_text(value).into_iter().collect(),
    };
    let values = raw_values
        .into_iter()
        .flat_map(|value| {
            value
                .split([',', ';', '\n'])
                .map(str::trim)
                .filter(|value| !value.is_empty())
                .map(ToOwned::to_owned)
                .collect::<Vec<_>>()
        })
        .collect::<Vec<_>>();
    (!values.is_empty()).then_some(values)
}

fn location_text(value: Option<&Value>) -> Option<String> {
    let values = match value {
        Some(Value::Array(values)) => values
            .iter()
            .filter_map(location_text_value)
            .collect::<Vec<_>>(),
        Some(value) => location_text_value(value).into_iter().collect(),
        None => Vec::new(),
    };
    (!values.is_empty()).then(|| values.join(" / "))
}

fn location_text_value(value: &Value) -> Option<String> {
    let object = value.as_object()?;
    if let Some(name) = object.get("name").and_then(value_text) {
        return non_empty(&name);
    }
    let address = object.get("address").unwrap_or(value);
    let address = address.as_object()?;
    let parts = [
        "streetAddress",
        "addressLocality",
        "addressRegion",
        "addressCountry",
    ]
    .iter()
    .filter_map(|key| address.get(*key).and_then(value_text))
    .filter_map(|value| non_empty(&value))
    .collect::<Vec<_>>();
    (!parts.is_empty()).then(|| parts.join(", "))
}

fn location_coordinates(value: Option<&Value>) -> (Option<f64>, Option<f64>) {
    let values = match value {
        Some(Value::Array(values)) => values.iter().collect::<Vec<_>>(),
        Some(value) => vec![value],
        None => Vec::new(),
    };
    for value in values {
        let Some(object) = value.as_object() else {
            continue;
        };
        let geo = object
            .get("geo")
            .and_then(Value::as_object)
            .unwrap_or(object);
        let latitude = geo.get("latitude").and_then(number_value);
        let longitude = geo.get("longitude").and_then(number_value);
        if latitude.is_some() || longitude.is_some() {
            return (latitude, longitude);
        }
    }
    (None, None)
}

fn parse_salary(
    value: Option<&Value>,
) -> (Option<f64>, Option<f64>, Option<String>, Option<String>) {
    let Some(object) = value.and_then(Value::as_object) else {
        return (None, None, None, None);
    };
    let currency = object
        .get("currency")
        .and_then(value_text)
        .and_then(|value| non_empty(&value));
    let value = object.get("value");
    let (minimum, maximum, single) = match value {
        Some(Value::Object(value)) => (
            value.get("minValue").and_then(number_value),
            value.get("maxValue").and_then(number_value),
            value.get("value").and_then(number_value),
        ),
        Some(value) => (None, None, number_value(value)),
        None => (None, None, None),
    };
    let minimum = minimum.or(single);
    let period = object
        .get("value")
        .and_then(Value::as_object)
        .and_then(|value| value.get("unitText"))
        .and_then(value_text)
        .and_then(|value| non_empty(&value));
    (minimum, maximum.or(single), currency, period)
}

fn number_value(value: &Value) -> Option<f64> {
    match value {
        Value::Number(value) => value.as_f64(),
        Value::String(value) => value.replace(',', "").parse::<f64>().ok(),
        _ => None,
    }
}

fn experience_years(value: Option<&Value>) -> Option<f64> {
    let object = value.and_then(Value::as_object);
    if let Some(number) = object
        .and_then(|value| value.get("yearsOfExperience"))
        .and_then(number_value)
    {
        return Some(number);
    }
    if let Some(months) = object
        .and_then(|value| value.get("monthsOfExperience"))
        .and_then(number_value)
    {
        return Some(months / 12.0);
    }
    let text = value.and_then(value_text)?;
    let number = first_number(&text)?;
    if text.to_ascii_lowercase().contains("month") {
        Some(number / 12.0)
    } else if text.to_ascii_lowercase().contains("year") {
        Some(number)
    } else {
        None
    }
}

fn first_number(value: &str) -> Option<f64> {
    value
        .split(|character: char| !character.is_ascii_digit() && character != '.')
        .find(|part| !part.is_empty())
        .and_then(|part| part.parse::<f64>().ok())
}

fn derive_seniority(value: &str) -> String {
    let value = value.to_ascii_lowercase();
    if value.contains("intern") || value.contains("entry") || value.contains("junior") {
        "JUNIOR".to_owned()
    } else if value.contains("senior") || value.contains("lead") || value.contains("principal") {
        "SENIOR".to_owned()
    } else if value.contains("mid") || value.contains("middle") {
        "MID".to_owned()
    } else {
        "UNSPECIFIED".to_owned()
    }
}

fn derive_work_mode(value: &str) -> String {
    let value = value.to_ascii_lowercase();
    if value.contains("telecommute") || value.contains("remote") {
        "REMOTE".to_owned()
    } else if value.contains("hybrid") {
        "HYBRID".to_owned()
    } else {
        "ONSITE".to_owned()
    }
}

fn normalize_employment_type(value: String) -> String {
    value
        .split(['-', ' ', '/'])
        .filter(|part| !part.is_empty())
        .map(|part| part.to_ascii_uppercase())
        .collect::<Vec<_>>()
        .join("_")
}

fn hash_json(object: &Map<String, Value>) -> Result<String, AdapterError> {
    let bytes = serde_json::to_vec(object).map_err(|_| AdapterError::Rejected)?;
    let mut hasher = Sha256::new();
    hasher.update(bytes);
    Ok(format!("{:x}", hasher.finalize()))
}

fn is_expired(value: &str) -> bool {
    DateTime::parse_from_rfc3339(value)
        .map(|date| date.with_timezone(&Utc) < Utc::now())
        .unwrap_or_else(|_| {
            NaiveDate::parse_from_str(value, "%Y-%m-%d")
                .map(|date| date < Utc::now().date_naive())
                .unwrap_or(false)
        })
}

fn non_empty(value: &str) -> Option<String> {
    let value = value.trim();
    (!value.is_empty()).then(|| value.to_owned())
}

#[derive(Debug, Clone)]
pub struct CrawlReport {
    pub run_status: RunStatus,
    pub pages_seen: i32,
    pub jobs_seen: i32,
    pub jobs_created: i32,
    pub jobs_updated: i32,
    pub jobs_missing: i32,
    pub error_code: Option<&'static str>,
    pub reconciliation_safe: bool,
    pub observations: Vec<StructuredJobObservation>,
}

#[derive(Debug, Error)]
pub enum CrawlError {
    #[error(transparent)]
    Scope(#[from] ScopeError),
    #[error("source host could not be resolved")]
    DNSResolution,
    #[error("crawler HTTP client could not be configured")]
    HTTPClientBuild,
    #[error("robots policy could not be fetched: {0}")]
    RobotsUnavailable(String),
    #[error("robots policy response is too large")]
    RobotsTooLarge,
}

#[derive(Debug, Error)]
enum BodyReadError {
    #[error("HTTP response body could not be read")]
    Read,
    #[error("HTTP response body is too large")]
    TooLarge,
}

pub async fn crawl_source(
    base_url: &str,
    configured_robots_url: Option<&str>,
    adapter: &dyn PageAdapter,
) -> Result<CrawlReport, CrawlError> {
    let scope = SourceScope::new(base_url)?;
    let resolved_addresses = validate_resolved_host(&scope).await?;
    // Pin every DNS result that passed the public-network check into Spider's
    // reqwest client. This closes the validation/connect TOCTOU window: a
    // later DNS answer cannot redirect the crawler to a private destination.
    let callback_scope = scope.clone();
    let mut website = Website::new(scope.base_url().as_str());
    website
        // Spider's built-in robots support fetches the origin-root URL itself
        // and does not expose the parsed policy to our path boundary. The
        // policy is fetched once with the pinned client and enforced by the
        // custom pre-request engine below instead.
        .with_respect_robots_txt(false)
        .with_depth(3)
        .with_limit(100)
        .with_redirect_limit(0)
        .with_redirect_policy(RedirectPolicy::None)
        .with_on_should_crawl_callback_closure(Some(move |page: &Page| {
            callback_scope.allows(page.get_url()) && callback_scope.allows(page.get_url_final())
        }));
    let host = scope.base_url().host_str().ok_or(ScopeError::MissingHost)?;
    let client = Client::builder()
        .tls_backend_rustls()
        .redirect(spider::client::redirect::Policy::none())
        .http1_only()
        .user_agent(ROBOTS_USER_AGENT)
        .resolve_to_addrs(host, &resolved_addresses)
        .build()
        .map_err(|_| CrawlError::HTTPClientBuild)?;
    let robots = fetch_robots_policy(&client, &scope, configured_robots_url).await?;
    if !robots.can_fetch(ROBOTS_USER_AGENT, scope.base_url().as_str()) {
        return Ok(source_error_report("robots_disallowed"));
    }

    website.set_http_client(client.clone());
    website.with_fetch_engine(ScopedHttpFetchEngine {
        scope: scope.clone(),
        client,
        robots,
    });

    let mut receiver = website.subscribe(0);
    website.crawl().await;

    let mut pages = Vec::new();
    let mut page_stream_lagged = false;
    loop {
        match receiver.try_recv() {
            Ok(page) => pages.push(page),
            Err(tokio::sync::broadcast::error::TryRecvError::Lagged(_)) => {
                page_stream_lagged = true;
                break;
            }
            Err(_) => break,
        }
    }
    let fetch_failed = crawl_status_is_failure(*website.get_status())
        || page_stream_lagged
        || pages
            .iter()
            .filter(|page| scope.allows(page.get_url()) && scope.allows(page.get_url_final()))
            .any(|page| {
                page.error_status.is_some()
                    || !page.status_code.is_success()
                    || page.content_truncated
            });
    let mut report = CrawlReport {
        run_status: if fetch_failed {
            RunStatus::SourceError
        } else {
            RunStatus::Healthy
        },
        pages_seen: 0,
        jobs_seen: 0,
        jobs_created: 0,
        jobs_updated: 0,
        jobs_missing: 0,
        error_code: None,
        reconciliation_safe: true,
        observations: Vec::new(),
    };
    report.error_code = if fetch_failed {
        Some("source_fetch_incomplete")
    } else {
        None
    };
    for page in &pages {
        if !scope.allows(page.get_url()) || !scope.allows(page.get_url_final()) {
            continue;
        }
        report.pages_seen += 1;
        if !fetch_failed {
            match adapter.observe(page) {
                Ok(batch) => {
                    report.reconciliation_safe &= batch.authoritative;
                    report.jobs_seen += i32::try_from(batch.observations.len()).unwrap_or(i32::MAX);
                    report.observations.extend(batch.observations);
                }
                Err(AdapterError::NotConfigured) => {
                    report.run_status = RunStatus::ParserError;
                    report.error_code = Some("adapter_not_configured");
                }
                Err(AdapterError::Rejected) => {
                    report.run_status = RunStatus::ParserError;
                    report.error_code = Some("adapter_rejected_page");
                }
            }
        }
    }
    finalize_crawl_report(&mut report);
    Ok(report)
}

fn source_error_report(error_code: &'static str) -> CrawlReport {
    CrawlReport {
        run_status: RunStatus::SourceError,
        pages_seen: 0,
        jobs_seen: 0,
        jobs_created: 0,
        jobs_updated: 0,
        jobs_missing: 0,
        error_code: Some(error_code),
        reconciliation_safe: false,
        observations: Vec::new(),
    }
}

fn finalize_crawl_report(report: &mut CrawlReport) {
    if report.run_status != RunStatus::Healthy {
        return;
    }
    if report.pages_seen == 0 {
        report.run_status = RunStatus::SourceError;
        report.error_code = Some("no_pages_seen");
    } else if !report.reconciliation_safe {
        report.run_status = RunStatus::Anomaly;
        report.error_code = Some("extraction_not_authoritative");
    }
}

struct ScopedHttpFetchEngine {
    scope: SourceScope,
    client: Client,
    robots: RobotFileParser,
}

const ROBOTS_USER_AGENT: &str = "FerrisCrawler";

#[async_trait::async_trait]
impl HttpFetchEngine for ScopedHttpFetchEngine {
    async fn fetch(&self, request: EngineRequest<'_>) -> Result<EngineResponse, EngineError> {
        let request_url = self
            .scope
            .canonical_request_url(request.url)
            .ok_or(EngineError::Request)?;
        if !self
            .robots
            .can_fetch(ROBOTS_USER_AGENT, request_url.as_str())
        {
            return Err(EngineError::Status(403));
        }

        let mut response = self
            .client
            .get(request_url.as_str())
            .send()
            .await
            .map_err(|error| EngineError::Other(error.to_string()))?;
        let status_code = response.status();
        let headers = response.headers().clone();
        let declared_content_length = response.content_length();
        let final_url = response.url().clone();
        if !self.scope.allows_url(&final_url) {
            return Err(EngineError::Request);
        }
        if declared_content_length.is_some_and(|length| length > MAX_PAGE_BODY_BYTES as u64) {
            return Err(EngineError::Other(
                "HTTP response body is too large".to_string(),
            ));
        }
        let body = read_bounded_body(&mut response, MAX_PAGE_BODY_BYTES)
            .await
            .map_err(|error| EngineError::Other(error.to_string()))?;

        Ok(EngineResponse {
            status_code,
            final_url: Some(final_url.to_string()),
            headers,
            body: body.to_vec(),
            declared_content_length,
            served: true,
            ..Default::default()
        })
    }

    fn should_fetch(&self, _url: &str) -> bool {
        // Returning false would make Spider fall back to its own client and
        // bypass the path guard. Every HTTP body request must enter fetch().
        true
    }
}

const MAX_ROBOTS_BODY_BYTES: usize = 1024 * 1024;
const MAX_PAGE_BODY_BYTES: usize = 10 * 1024 * 1024;

async fn read_bounded_body(
    response: &mut spider::client::Response,
    maximum_bytes: usize,
) -> Result<Vec<u8>, BodyReadError> {
    let mut body = Vec::new();
    while let Some(chunk) = response.chunk().await.map_err(|_| BodyReadError::Read)? {
        if body.len() > maximum_bytes || chunk.len() > maximum_bytes - body.len() {
            return Err(BodyReadError::TooLarge);
        }
        body.extend_from_slice(&chunk);
    }
    Ok(body)
}

fn robots_check_url(
    scope: &SourceScope,
    configured_robots_url: Option<&str>,
) -> Result<Url, CrawlError> {
    let candidate = match configured_robots_url {
        Some(raw) => Url::parse(raw)
            .map_err(|_| CrawlError::RobotsUnavailable("configured URL is invalid".to_owned()))?,
        None => {
            let mut derived = scope.base_url().clone();
            derived.set_path("/robots.txt");
            derived.set_query(None);
            derived.set_fragment(None);
            derived
        }
    };
    if candidate.username() != ""
        || candidate.password().is_some()
        || !same_origin(scope.base_url(), &candidate)
        || candidate.path().is_empty()
    {
        return Err(CrawlError::RobotsUnavailable(
            "configured URL is outside the source origin".to_owned(),
        ));
    }
    scope.canonicalize_host_and_port(&candidate).ok_or_else(|| {
        CrawlError::RobotsUnavailable("configured URL cannot be canonicalized".to_owned())
    })
}

async fn fetch_robots_policy(
    client: &Client,
    scope: &SourceScope,
    configured_robots_url: Option<&str>,
) -> Result<RobotFileParser, CrawlError> {
    let robots_url = robots_check_url(scope, configured_robots_url)?;
    let mut response = client
        .get(robots_url.as_str())
        .send()
        .await
        .map_err(|error| CrawlError::RobotsUnavailable(format!("request failed: {error:?}")))?;
    if !same_origin(&robots_url, response.url()) {
        return Err(CrawlError::RobotsUnavailable(
            "response changed origin".to_owned(),
        ));
    }

    let status = response.status();
    let mut parser = *RobotFileParser::new();
    parser.modified();
    if status == spider::client::StatusCode::UNAUTHORIZED
        || status == spider::client::StatusCode::FORBIDDEN
    {
        parser.disallow_all = true;
        return Ok(parser);
    }
    if status.is_client_error() {
        parser.allow_all = true;
        return Ok(parser);
    }
    if !status.is_success() {
        return Err(CrawlError::RobotsUnavailable(format!(
            "HTTP status {}",
            status.as_u16()
        )));
    }
    if response
        .content_length()
        .is_some_and(|length| length > MAX_ROBOTS_BODY_BYTES as u64)
    {
        return Err(CrawlError::RobotsTooLarge);
    }
    let body = read_bounded_body(&mut response, MAX_ROBOTS_BODY_BYTES)
        .await
        .map_err(|error| match error {
            BodyReadError::Read => {
                CrawlError::RobotsUnavailable("response body could not be read".to_owned())
            }
            BodyReadError::TooLarge => CrawlError::RobotsTooLarge,
        })?;
    parser.parse_str(&String::from_utf8_lossy(&body));
    Ok(parser)
}

fn same_origin(left: &Url, right: &Url) -> bool {
    left.scheme().eq_ignore_ascii_case(right.scheme())
        && hosts_are_equivalent(
            left.host_str().unwrap_or_default(),
            right.host_str().unwrap_or_default(),
        )
        && left.port_or_known_default() == right.port_or_known_default()
}

async fn validate_resolved_host(scope: &SourceScope) -> Result<Vec<SocketAddr>, CrawlError> {
    let host = scope.base_url().host_str().ok_or(ScopeError::MissingHost)?;
    let port = scope
        .base_url()
        .port_or_known_default()
        .ok_or(ScopeError::InvalidScheme)?;
    let addresses = tokio::net::lookup_host((host, port))
        .await
        .map_err(|_| CrawlError::DNSResolution)?
        .collect::<Vec<_>>();
    if addresses.is_empty() {
        return Err(CrawlError::DNSResolution);
    }
    validate_public_addresses(&addresses)?;
    Ok(addresses)
}

fn validate_public_addresses(addresses: &[SocketAddr]) -> Result<(), CrawlError> {
    if addresses.is_empty() || addresses.iter().any(|address| !is_public_ip(address.ip())) {
        return Err(CrawlError::Scope(ScopeError::PrivateHost));
    }
    Ok(())
}

fn crawl_status_is_failure(status: CrawlStatus) -> bool {
    !matches!(status, CrawlStatus::Idle)
}

#[cfg(test)]
mod tests {
    use super::{
        crawl_status_is_failure, finalize_crawl_report, parse_job_posting_html,
        validate_public_addresses, CrawlError, ScopedHttpFetchEngine,
    };
    use crate::scope::ScopeError;
    use crate::{crawl::CrawlReport, reconcile::RunStatus, scope::SourceScope};
    use spider::{
        client::{Client, StatusCode},
        configuration::Configuration,
        fetch_engine::{EngineError, EngineMethod, EngineRequest, HttpFetchEngine},
        packages::robotparser::parser::RobotFileParser,
        website::CrawlStatus,
    };
    use std::net::{IpAddr, Ipv4Addr, Ipv6Addr, SocketAddr};
    use tokio::io::{AsyncReadExt, AsyncWriteExt};
    use tokio::net::TcpListener;

    #[test]
    fn pinned_destination_set_must_contain_only_public_addresses() {
        assert!(validate_public_addresses(&[SocketAddr::new(
            IpAddr::V4(Ipv4Addr::new(93, 184, 216, 34)),
            443,
        )])
        .is_ok());
        assert!(matches!(
            validate_public_addresses(&[SocketAddr::new(IpAddr::V6(Ipv6Addr::LOCALHOST), 443,)]),
            Err(CrawlError::Scope(ScopeError::PrivateHost))
        ));
    }

    #[test]
    fn incomplete_spider_status_is_classified_as_source_error() {
        assert!(!crawl_status_is_failure(CrawlStatus::Idle));
        assert!(crawl_status_is_failure(CrawlStatus::ConnectError));
        assert!(crawl_status_is_failure(CrawlStatus::ServerError));
        assert!(crawl_status_is_failure(CrawlStatus::RateLimited));
    }

    #[test]
    fn non_authoritative_extraction_cannot_reconcile_as_healthy() {
        let mut report = CrawlReport {
            run_status: RunStatus::Healthy,
            pages_seen: 1,
            jobs_seen: 0,
            jobs_created: 0,
            jobs_updated: 0,
            jobs_missing: 0,
            error_code: None,
            reconciliation_safe: false,
            observations: Vec::new(),
        };
        finalize_crawl_report(&mut report);
        assert_eq!(report.run_status, RunStatus::Anomaly);
        assert_eq!(report.error_code, Some("extraction_not_authoritative"));
    }

    #[test]
    fn json_ld_job_posting_becomes_structured_observation_without_description_storage() {
        let html = r#"
            <script type="application/ld+json">
            {
              "@context":"https://schema.org",
              "@type":"JobPosting",
              "identifier":{"value":"fpt-26118"},
              "title":"Front-end Developer",
              "description":"This raw job description must never enter the observation.",
              "hiringOrganization":{"name":"FPT Telecom"},
              "jobLocation":{"address":{"addressLocality":"Ho Chi Minh City","addressCountry":"VN"},"geo":{"latitude":10.7769,"longitude":106.7009}},
              "employmentType":"FULL_TIME",
              "skills":"React, TypeScript",
              "experienceRequirements":"3 years experience",
              "baseSalary":{"currency":"VND","value":{"minValue":20000000,"maxValue":30000000,"unitText":"MONTH"}}
            }
            </script>
        "#;
        let batch =
            parse_job_posting_html(html, "https://jobs.example.com/careers/front-end-26118")
                .unwrap();
        assert!(batch.authoritative);
        assert_eq!(batch.observations.len(), 1);
        let observation = &batch.observations[0];
        assert_eq!(observation.source_job_key, "fpt-26118");
        assert_eq!(observation.company, "FPT Telecom");
        assert_eq!(observation.location_text, "Ho Chi Minh City, VN");
        assert_eq!(observation.required_skills, vec!["React", "TypeScript"]);
        assert_eq!(observation.salary_min, Some(20_000_000.0));
        assert_eq!(observation.salary_max, Some(30_000_000.0));
        assert_eq!(observation.salary_currency.as_deref(), Some("VND"));
        assert_eq!(observation.latitude, Some(10.7769));
        assert_eq!(observation.longitude, Some(106.7009));
        assert!(!format!("{observation:?}").contains("raw job description"));
    }

    #[test]
    fn page_without_job_posting_is_non_authoritative_and_cannot_reconcile() {
        let batch = parse_job_posting_html(
            "<html><body>listing shell</body></html>",
            "https://jobs.example.com/careers/",
        )
        .unwrap();
        assert!(!batch.authoritative);
        assert!(batch.observations.is_empty());
    }

    #[tokio::test]
    async fn scoped_engine_rejects_out_of_prefix_before_http_request() {
        let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0)).await.unwrap();
        let port = listener.local_addr().unwrap().port();
        let server = tokio::spawn(async move {
            let (mut stream, _) = listener.accept().await.unwrap();
            let mut request = [0_u8; 1024];
            let _ = stream.read(&mut request).await.unwrap();
            stream
                .write_all(
                    b"HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nContent-Length: 2\r\nConnection: close\r\n\r\nok",
                )
                .await
                .unwrap();
        });
        let base_url = format!("http://example.com:{port}/careers/");
        let scope = SourceScope::new(&base_url).unwrap();
        let client = Client::builder()
            .redirect(spider::client::redirect::Policy::none())
            .resolve_to_addrs(
                "example.com",
                &[SocketAddr::new(IpAddr::V4(Ipv4Addr::LOCALHOST), port)],
            )
            .build()
            .unwrap();
        let mut robots = *RobotFileParser::new();
        robots.allow_all = true;
        let engine = ScopedHttpFetchEngine {
            scope,
            client,
            robots,
        };
        let configuration = Configuration::default();
        let outside = format!("http://example.com:{port}/outside");
        let outside_result = engine
            .fetch(EngineRequest {
                url: &outside,
                method: EngineMethod::Get,
                configuration: &configuration,
                only_html: true,
                attempt: 0,
                conditional_headers: None,
            })
            .await;
        assert!(matches!(outside_result, Err(EngineError::Request)));

        let inside = format!("http://EXAMPLE.COM.:{port}/careers/jobs");
        let inside_result = engine
            .fetch(EngineRequest {
                url: &inside,
                method: EngineMethod::Get,
                configuration: &configuration,
                only_html: true,
                attempt: 0,
                conditional_headers: None,
            })
            .await
            .unwrap();
        assert_eq!(inside_result.status_code, StatusCode::OK);
        assert_eq!(inside_result.body, b"ok");
        server.await.unwrap();
    }

    #[tokio::test]
    async fn robots_policy_blocks_registered_path_before_page_crawl() {
        let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0)).await.unwrap();
        let port = listener.local_addr().unwrap().port();
        let server = tokio::spawn(async move {
            let (mut stream, _) = listener.accept().await.unwrap();
            let mut request = [0_u8; 1024];
            let size = stream.read(&mut request).await.unwrap();
            stream
                .write_all(
                    b"HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 34\r\nConnection: close\r\n\r\nUser-agent: *\nDisallow: /careers/\n",
                )
                .await
                .unwrap();
            String::from_utf8_lossy(&request[..size]).into_owned()
        });
        let base_url = format!("http://example.com:{port}/careers/");
        let robots_url = format!("http://example.com:{port}/robots.txt");
        let scope = SourceScope::new(&base_url).unwrap();
        let client = Client::builder()
            .redirect(spider::client::redirect::Policy::none())
            .resolve_to_addrs(
                "example.com",
                &[SocketAddr::new(IpAddr::V4(Ipv4Addr::LOCALHOST), port)],
            )
            .build()
            .unwrap();
        let robots = super::fetch_robots_policy(&client, &scope, Some(&robots_url))
            .await
            .unwrap();
        assert!(!robots.can_fetch(super::ROBOTS_USER_AGENT, &base_url));

        let engine = super::ScopedHttpFetchEngine {
            scope,
            client,
            robots,
        };
        let configuration = Configuration::default();
        let denied = engine
            .fetch(EngineRequest {
                url: &base_url,
                method: EngineMethod::Get,
                configuration: &configuration,
                only_html: true,
                attempt: 0,
                conditional_headers: None,
            })
            .await;
        assert!(matches!(
            denied,
            Err(EngineError::Status(status)) if status == StatusCode::FORBIDDEN
        ));
        assert!(server.await.unwrap().starts_with("GET /robots.txt "));
    }
}
