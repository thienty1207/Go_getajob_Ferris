# Baron Intent Brief

- ID: `intent-799bfb7fd60c519d`
- Title: client-frontend-mvp
- Risk: `medium`
- Confirmation: `confirmed`
- Updated: 2026-08-15T18:32:16+07:00

## Current Behavior

Repository chỉ có product spec và UI Demo; chưa có frontend source, API client hay runtime data flow.

## Target Behavior

Có SvelteKit client UI riêng, responsive và accessible, dùng API thật cho CV scan/match results, không có mock data; admin giữ ở surface/folder riêng và chưa triển khai trong tranche này.

## Scope

Chỉ triển khai frontend client trong frontend/; scaffold app, client routes/components, shared API/types, upload validation và browser verification.

## Non-Goals

- Không triển khai admin UI trong tranche này.
- Không triển khai backend, database, crawler hoặc DeepSeek integration.
- Không tạo mock data, fake API response, seed job records hoặc fallback sample results.

## Constraints

- Frontend stack là SvelteKit + TypeScript + Bun.
- CV upload nhận PDF/DOCX/TXT tối đa 10 MB.
- Location/radius dùng Việt Nam và km.
- Public job fields gồm match percent, title, company, location, distance km, employment type, work mode, optional source salary, tối đa 3 skill tags và original URL.
- Không lưu raw CV lâu dài; frontend chỉ gửi file đến API contract thật và không xử lý PII ngoài phạm vi cần thiết.

## Decisions

- Route/client và lib/client tách rõ khỏi route/admin và lib/admin.
- Khi backend/API chưa sẵn sàng, UI hiển thị loading/empty/error thật, không fallback mock.

## Required Proof

bun install; bun run check; bun run build; browser smoke test và screenshot client UI.

## Remaining Unknowns

- Backend endpoint paths và authentication contract cần được chốt trước khi tích hợp end-to-end.

## Agent Rules

- Read project, Vault, plan, Harness, and prior decisions before asking the user.
- Ask one missing high-value question at a time.
- Do not treat unknowns as facts.
- Medium/high-risk implementation requires this intent to be confirmed.
