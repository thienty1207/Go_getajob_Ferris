# Baron Review Finding

- ID: `finding-2a462e488db2c71f`
- Status: `closed`
- Severity: `medium`
- Recorded: 2026-08-16T20:41:11+07:00

## Summary

Crawler DNS validation is TOCTOU-prone: Spider re-resolves source hostname after one public DNS preflight, leaving a DNS-rebinding SSRF path; destination binding and negative proof are required.

## Evidence

- crawler/src/crawl.rs performs lookup_host before Spider, then passes hostname to Spider; crawler/src/scope.rs validates URL text only; no per-request destination IP binding

## Affected Files

- none recorded

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.

## Closure Evidence

- Closed: 2026-08-16T23:49:05+07:00
- Fix evidence: Crawler now canonicalizes every accepted request host and numeric port before the pinned reqwest client, disables redirects, validates all resolved destinations, enforces path-aware robots and body bounds, and rejects unresolved persistence recovery; regression coverage includes terminal-dot fetch, canonical ports, query/fragment path escapes, and public destination checks.
- Verification: receipt-ca42540e3e2b Rust tests passed; receipt-4ee53b4351da strict Clippy passed; receipt-24e1dada9273 live PostgreSQL crawler contract passed; receipt-83d6d92c8b9d crawler runtime smoke passed; current security-auditor gate PASS.
