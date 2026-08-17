# Baron Actionable Recovery

- Recovery ID: `recovery-afb9922188ca3578`
- Outcome: `interrupted`
- Recorded: 2026-08-17T19:56:21+07:00

## Root Cause

Baron proof runner for backend-go-tests stayed silent and was interrupted; direct backend, crawler, PostgreSQL contract, frontend, schema, API health, and browser smoke checks passed.

## Last Successful Step

direct verification and staged secret/build-artifact scan

## Evidence

- none recorded

## Affected Files

- none recorded

## Safe Next Action

Preserve direct receipts; retry Baron proof only after worker/state lock clears.

## Retry Conditions

- none recorded

## Linked State

- Plan: `location-aware-crawler-and-matching`
- Harness story: `unknown`
- Harness risk: `unknown`
- Proof: 20260817112413794 - trusted execution receipt receipt-5500c954f6d1 passed for frontend-production-build via bun
- Trace: standard/standard passed yes

## Recovery Rules

- Preserve this failed attempt even after a later retry succeeds.
- Reconcile repo state before retrying.
- Do not claim completion until required proof and trace pass.
