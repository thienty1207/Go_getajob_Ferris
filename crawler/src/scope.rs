use std::net::{IpAddr, Ipv4Addr, Ipv6Addr};

use thiserror::Error;
use url::Url;

#[derive(Debug, Error, PartialEq, Eq)]
pub enum ScopeError {
    #[error("source URL must use http or https")]
    InvalidScheme,
    #[error("source URL must contain a host")]
    MissingHost,
    #[error("source URL cannot contain credentials")]
    CredentialsNotAllowed,
    #[error("source URL points to a private or local host")]
    PrivateHost,
    #[error("source URL has an invalid port")]
    InvalidPort,
    #[error("source URL is malformed")]
    MalformedURL,
}

#[derive(Debug, Clone)]
pub struct SourceScope {
    base: Url,
    path_prefix: String,
}

impl SourceScope {
    pub fn new(raw: &str) -> Result<Self, ScopeError> {
        let mut base = parse_public_url(raw)?;
        canonicalize_network_url(&mut base)?;
        // Go canonicalizes Job Links by dropping fragments while retaining the
        // query. Apply the same rule here so direct Rust callers cannot create
        // a second boundary contract.
        base.set_fragment(None);
        let path_prefix = canonical_path(base.path());
        Ok(Self { base, path_prefix })
    }

    pub fn base_url(&self) -> &Url {
        &self.base
    }

    pub fn allows(&self, raw: &str) -> bool {
        let Ok(candidate) = parse_url(raw) else {
            return false;
        };
        self.allows_url(&candidate)
    }

    pub fn allows_url(&self, candidate: &Url) -> bool {
        if candidate.username() != "" || candidate.password().is_some() {
            return false;
        }
        if has_ambiguous_path_encoding(serialized_path(candidate)) {
            return false;
        }
        if !candidate.scheme().eq_ignore_ascii_case(self.base.scheme()) {
            return false;
        }
        if !same_host_and_port(&self.base, candidate) {
            return false;
        }
        path_is_under(&self.path_prefix, &canonical_path(candidate.path()))
    }

    pub fn canonical_request_url(&self, raw: &str) -> Option<Url> {
        let candidate = parse_url(raw).ok()?;
        if !self.allows_url(&candidate) {
            return None;
        }
        self.canonicalize_host_and_port(&candidate)
    }

    pub(crate) fn canonicalize_host_and_port(&self, candidate: &Url) -> Option<Url> {
        if !same_host_and_port(&self.base, candidate) {
            return None;
        }
        let mut canonical = candidate.clone();
        canonicalize_network_url(&mut canonical).ok()?;
        Some(canonical)
    }
}

fn parse_public_url(raw: &str) -> Result<Url, ScopeError> {
    let url = parse_url(raw)?;
    let host = url.host_str().ok_or(ScopeError::MissingHost)?;
    if !is_public_hostname(host) {
        return Err(ScopeError::PrivateHost);
    }
    Ok(url)
}

fn parse_url(raw: &str) -> Result<Url, ScopeError> {
    let raw = raw.trim();
    if has_ambiguous_path_encoding(raw_path(raw)) {
        return Err(ScopeError::MalformedURL);
    }
    let url = Url::parse(raw).map_err(|_| ScopeError::MalformedURL)?;
    if url.scheme() != "http" && url.scheme() != "https" {
        return Err(ScopeError::InvalidScheme);
    }
    if url.host_str().is_none() {
        return Err(ScopeError::MissingHost);
    }
    if url.port().is_some_and(|port| port == 0) {
        return Err(ScopeError::InvalidPort);
    }
    if url.username() != "" || url.password().is_some() {
        return Err(ScopeError::CredentialsNotAllowed);
    }
    if has_ambiguous_path_encoding(serialized_path(&url)) {
        return Err(ScopeError::MalformedURL);
    }
    Ok(url)
}

fn serialized_path(url: &Url) -> &str {
    raw_path(url.as_str())
}

fn raw_path(serialized: &str) -> &str {
    let authority_start = serialized.find("://").map(|index| index + 3).unwrap_or(0);
    let path_start = serialized[authority_start..]
        .find(['/', '?', '#'])
        .map(|index| authority_start + index)
        .unwrap_or(serialized.len());
    if serialized.as_bytes().get(path_start) != Some(&b'/') {
        return "";
    }
    let path_end = serialized[path_start..]
        .find(['?', '#'])
        .map(|index| path_start + index)
        .unwrap_or(serialized.len());
    &serialized[path_start..path_end]
}

fn same_host_and_port(left: &Url, right: &Url) -> bool {
    let left_host = left.host_str().unwrap_or_default();
    let right_host = right.host_str().unwrap_or_default();
    hosts_are_equivalent(left_host, right_host)
        && left.port_or_known_default() == right.port_or_known_default()
}

pub(crate) fn hosts_are_equivalent(left: &str, right: &str) -> bool {
    let left = left.trim_end_matches('.');
    let right = right.trim_end_matches('.');
    match (left.parse::<IpAddr>(), right.parse::<IpAddr>()) {
        (Ok(left), Ok(right)) => normalize_ip(left) == normalize_ip(right),
        (Ok(_), Err(_)) | (Err(_), Ok(_)) => false,
        (Err(_), Err(_)) => left.eq_ignore_ascii_case(right),
    }
}

fn normalize_ip(ip: IpAddr) -> IpAddr {
    match ip {
        IpAddr::V6(address) => address
            .to_ipv4_mapped()
            .map(IpAddr::V4)
            .unwrap_or(IpAddr::V6(address)),
        other => other,
    }
}

fn canonicalize_network_url(url: &mut Url) -> Result<(), ScopeError> {
    let host = url.host_str().ok_or(ScopeError::MissingHost)?;
    let canonical_host = canonical_host(host);
    url.set_host(Some(&canonical_host))
        .map_err(|_| ScopeError::MalformedURL)?;
    if let Some(port) = url.port() {
        if (url.scheme() == "http" && port == 80) || (url.scheme() == "https" && port == 443) {
            url.set_port(None).map_err(|_| ScopeError::MalformedURL)?;
        } else {
            url.set_port(Some(port))
                .map_err(|_| ScopeError::MalformedURL)?;
        }
    }
    Ok(())
}

fn canonical_host(host: &str) -> String {
    let normalized = host.trim_end_matches('.').to_ascii_lowercase();
    match normalized.parse::<IpAddr>() {
        Ok(ip) => normalize_ip(ip).to_string(),
        Err(_) => normalized,
    }
}

fn path_is_under(prefix: &str, candidate: &str) -> bool {
    prefix == "/" || candidate == prefix || candidate.starts_with(prefix)
}

fn canonical_path(raw: &str) -> String {
    let mut segments = Vec::new();
    for segment in raw.split('/') {
        match segment {
            "" | "." => {}
            ".." => {
                segments.pop();
            }
            value => segments.push(value),
        }
    }
    if segments.is_empty() {
        return "/".to_string();
    }
    format!("/{}/", segments.join("/"))
}

fn has_ambiguous_path_encoding(path: &str) -> bool {
    let path = path.to_ascii_lowercase();
    ["%2e", "%2f", "%5c"]
        .iter()
        .any(|escape| path.contains(escape))
}

fn is_public_hostname(host: &str) -> bool {
    let normalized = host.trim_end_matches('.').to_ascii_lowercase();
    if normalized.is_empty()
        || normalized == "localhost"
        || normalized.ends_with(".localhost")
        || normalized.ends_with(".local")
        || normalized.ends_with(".internal")
    {
        return false;
    }
    let Ok(ip) = normalized.parse::<IpAddr>() else {
        return true;
    };
    is_public_ip(ip)
}

pub(crate) fn is_public_ip(ip: IpAddr) -> bool {
    match ip {
        IpAddr::V4(address) => is_globally_routable_ipv4(address),
        IpAddr::V6(address) => {
            if let Some(mapped) = address.to_ipv4_mapped() {
                return is_public_ip(IpAddr::V4(mapped));
            }
            is_globally_routable_ipv6(address)
        }
    }
}

fn is_globally_routable_ipv4(address: Ipv4Addr) -> bool {
    if address.is_private()
        || address.is_loopback()
        || address.is_link_local()
        || address.is_unspecified()
    {
        return false;
    }

    // These are IANA special-purpose ranges. A source hostname is allowed to
    // proceed only when every DNS result is globally routable unicast space.
    const SPECIAL_RANGES: &[(Ipv4Addr, u32)] = &[
        (Ipv4Addr::new(0, 0, 0, 0), 8),
        (Ipv4Addr::new(100, 64, 0, 0), 10),
        (Ipv4Addr::new(192, 0, 0, 0), 24),
        (Ipv4Addr::new(192, 0, 2, 0), 24),
        (Ipv4Addr::new(192, 31, 196, 0), 24),
        (Ipv4Addr::new(192, 52, 193, 0), 24),
        (Ipv4Addr::new(192, 88, 99, 0), 24),
        (Ipv4Addr::new(192, 175, 48, 0), 24),
        (Ipv4Addr::new(198, 18, 0, 0), 15),
        (Ipv4Addr::new(198, 51, 100, 0), 24),
        (Ipv4Addr::new(203, 0, 113, 0), 24),
        (Ipv4Addr::new(224, 0, 0, 0), 4),
        (Ipv4Addr::new(240, 0, 0, 0), 4),
    ];
    !SPECIAL_RANGES
        .iter()
        .any(|(network, prefix)| ipv4_in_cidr(address, *network, *prefix))
}

fn is_globally_routable_ipv6(address: Ipv6Addr) -> bool {
    if address.is_loopback()
        || address.is_unique_local()
        || address.is_unicast_link_local()
        || address.is_unspecified()
        || !ipv6_in_cidr(address, Ipv6Addr::new(0x2000, 0, 0, 0, 0, 0, 0, 0), 3)
    {
        return false;
    }

    // IPv6 protocol-transition, documentation, benchmark, and multicast
    // ranges are not globally routable application destinations.
    const SPECIAL_RANGES: &[(Ipv6Addr, u32)] = &[
        (Ipv6Addr::new(0x0064, 0xff9b, 0, 0, 0, 0, 0, 0), 96),
        (Ipv6Addr::new(0x0064, 0xff9b, 0x0001, 0, 0, 0, 0, 0), 48),
        (Ipv6Addr::new(0x2002, 0, 0, 0, 0, 0, 0, 0), 16),
        (Ipv6Addr::new(0x2001, 0, 0, 0, 0, 0, 0, 0), 23),
        (Ipv6Addr::new(0x2001, 0, 0, 0, 0, 0, 0, 0), 32),
        (Ipv6Addr::new(0x2001, 0x0002, 0, 0, 0, 0, 0, 0), 48),
        (Ipv6Addr::new(0x2001, 0x0003, 0, 0, 0, 0, 0, 0), 32),
        (Ipv6Addr::new(0x2001, 0x0010, 0, 0, 0, 0, 0, 0), 28),
        (Ipv6Addr::new(0x2001, 0x0020, 0, 0, 0, 0, 0, 0), 28),
        (Ipv6Addr::new(0x2001, 0x0db8, 0, 0, 0, 0, 0, 0), 32),
        (Ipv6Addr::new(0x3000, 0, 0, 0, 0, 0, 0, 0), 4),
        (Ipv6Addr::new(0x3fff, 0, 0, 0, 0, 0, 0, 0), 20),
        (Ipv6Addr::new(0xff00, 0, 0, 0, 0, 0, 0, 0), 8),
    ];
    !SPECIAL_RANGES
        .iter()
        .any(|(network, prefix)| ipv6_in_cidr(address, *network, *prefix))
}

fn ipv4_in_cidr(address: Ipv4Addr, network: Ipv4Addr, prefix: u32) -> bool {
    let mask = if prefix == 0 {
        0
    } else {
        u32::MAX << (32 - prefix)
    };
    u32::from(address) & mask == u32::from(network) & mask
}

fn ipv6_in_cidr(address: Ipv6Addr, network: Ipv6Addr, prefix: u32) -> bool {
    let mask = if prefix == 0 {
        0
    } else {
        u128::MAX << (128 - prefix)
    };
    u128::from(address) & mask == u128::from(network) & mask
}

#[cfg(test)]
mod tests {
    use super::{canonical_path, is_public_ip, path_is_under, ScopeError, SourceScope};
    use std::net::{IpAddr, Ipv4Addr, Ipv6Addr};

    #[test]
    fn canonical_path_always_has_a_segment_boundary() {
        assert_eq!(canonical_path("/careers"), "/careers/");
        assert_eq!(canonical_path("/careers/../jobs"), "/jobs/");
        assert!(path_is_under("/careers/", "/careers/engineering/"));
        assert!(!path_is_under("/careers/", "/careers-team/"));
    }

    #[test]
    fn ipv4_mapped_ipv6_addresses_use_ipv4_private_network_rules() {
        let mapped_loopback = Ipv6Addr::new(0, 0, 0, 0, 0, 0xffff, 0x7f00, 0x0001);
        assert!(!is_public_ip(IpAddr::V6(mapped_loopback)));
    }

    #[test]
    fn ipv4_special_use_ranges_are_not_public_destinations() {
        let addresses = [
            Ipv4Addr::new(100, 64, 0, 1),
            Ipv4Addr::new(192, 0, 2, 1),
            Ipv4Addr::new(192, 31, 196, 1),
            Ipv4Addr::new(198, 18, 0, 1),
            Ipv4Addr::new(198, 51, 100, 1),
            Ipv4Addr::new(203, 0, 113, 1),
            Ipv4Addr::new(224, 0, 0, 1),
            Ipv4Addr::new(240, 0, 0, 1),
        ];
        assert!(addresses
            .into_iter()
            .all(|address| !is_public_ip(IpAddr::V4(address))));
    }

    #[test]
    fn ipv6_special_use_ranges_are_not_public_destinations() {
        let addresses = [
            Ipv6Addr::new(0x0064, 0xff9b, 0, 0, 0, 0, 0, 1),
            Ipv6Addr::new(0x2002, 0, 0, 0, 0, 0, 0, 1),
            Ipv6Addr::new(0x2001, 0x0100, 0, 0, 0, 0, 0, 1),
            Ipv6Addr::new(0x2001, 0x0002, 0, 0, 0, 0, 0, 1),
            Ipv6Addr::new(0x2001, 0x0db8, 0, 0, 0, 0, 0, 1),
            Ipv6Addr::new(0x3000, 0, 0, 0, 0, 0, 0, 1),
            Ipv6Addr::new(0xff00, 0, 0, 0, 0, 0, 0, 1),
            Ipv6Addr::new(0, 0, 0, 0, 0, 0xffff, 0x6440, 1),
        ];
        assert!(addresses
            .into_iter()
            .all(|address| !is_public_ip(IpAddr::V6(address))));
        assert!(is_public_ip(IpAddr::V6(Ipv6Addr::new(
            0x2606, 0x4700, 0x4700, 0, 0, 0, 0, 1,
        ))));
    }

    #[test]
    fn encoded_path_traversal_is_rejected_at_the_source_boundary() {
        let scope = SourceScope::new("https://jobs.example.com/careers").unwrap();
        assert!(!scope.allows("https://jobs.example.com/careers/%2e%2e/admin"));
        assert!(matches!(
            SourceScope::new("https://jobs.example.com/careers/%2e%2e/admin"),
            Err(ScopeError::MalformedURL)
        ));
    }

    #[test]
    fn zero_port_is_rejected_at_the_source_boundary() {
        assert!(matches!(
            SourceScope::new("https://jobs.example.com:0/careers"),
            Err(ScopeError::InvalidPort)
        ));
    }

    #[test]
    fn host_comparison_matches_canonical_job_link_metadata_rules() {
        let scope = SourceScope::new("https://jobs.example.com.:000443/careers?region=vn#ignored")
            .expect("canonical source URL");
        assert_eq!(scope.base_url().host_str(), Some("jobs.example.com"));
        assert_eq!(scope.base_url().port(), None);
        assert_eq!(scope.base_url().fragment(), None);
        assert!(scope.allows("https://JOBS.EXAMPLE.COM/careers/jobs?filter=%2e%2e#ignored"));

        let ipv6_scope = SourceScope::new("https://[2606:4700:4700::1111]/careers")
            .expect("canonical IPv6 source URL");
        assert!(ipv6_scope.allows("https://[2606:4700:4700:0:0:0:0:1111]/careers/jobs"));

        let custom_port_scope = SourceScope::new("https://jobs.example.com:0008443/careers")
            .expect("canonical custom port URL");
        assert_eq!(custom_port_scope.base_url().port(), Some(8443));
    }

    #[test]
    fn path_escape_guard_ignores_query_and_fragment_text() {
        let scope = SourceScope::new("https://jobs.example.com").unwrap();
        assert!(scope.allows("https://jobs.example.com?next=/careers/%2e%2e"));
        assert!(scope.allows("https://jobs.example.com#next=/careers/%2e%2e"));
    }

    #[test]
    fn request_host_is_canonicalized_to_the_pinned_spelling() {
        let scope = SourceScope::new("https://jobs.example.com./careers").unwrap();
        let request = scope
            .canonical_request_url("https://JOBS.EXAMPLE.COM./careers/jobs")
            .expect("request is within the source boundary");
        assert_eq!(request.host_str(), Some("jobs.example.com"));
    }
}
