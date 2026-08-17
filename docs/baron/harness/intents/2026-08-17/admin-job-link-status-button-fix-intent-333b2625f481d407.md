# Baron Intent Brief

- ID: `intent-333b2625f481d407`
- Title: admin-job-link-status-button-fix
- Risk: `medium`
- Confirmation: `needs_confirmation`
- Updated: 2026-08-17T12:18:51+07:00

## Current Behavior

Admin bấm Dừng, xác nhận, sau đó UI báo không thể kết nối admin service và Job Link vẫn ACTIVE.

## Target Behavior

Admin bấm Dừng, request PATCH status đi tới Go API với session và CSRF hợp lệ, database chuyển approval_status thành DISABLED, UI reload hiển thị đúng trạng thái; Khởi động lại dùng cùng contract.

## Scope

frontend admin Job Link page, shared API URL resolution, Go status route only

## Non-Goals

- không thay đổi hard-delete, crawler adapter, schema, hoặc tạo mock data

## Constraints

- giữ cookie session và CSRF; phát triển local chạy qua Vite proxy; production vẫn dùng API base cấu hình rõ ràng

## Decisions

- ưu tiên cùng-origin /api trong dev để không tách localhost và 127.0.0.1

## Required Proof

frontend regression test, backend route/service tests, frontend check/build, live authenticated status smoke nếu môi trường session cho phép

## Remaining Unknowns

- cần xác nhận request hiện tại fail ở CORS/host cookie, route stale, hoặc backend process chưa restart bằng runtime evidence

## Agent Rules

- Read project, Vault, plan, Harness, and prior decisions before asking the user.
- Ask one missing high-value question at a time.
- Do not treat unknowns as facts.
- Medium/high-risk implementation requires this intent to be confirmed.
