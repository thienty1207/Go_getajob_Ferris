# Tài liệu tiếp quản dự án

File này dành cho AI hoặc developer mới tiếp quản dự án **Go get a job ferris**. Đọc file này trước khi đọc sâu vào source. Không cần dump toàn bộ repository hoặc lịch sử chat.

## 1. Thông tin nhanh

- Project root: `D:\Work\Project\Website\GoGetSomeFoodFerris`
- GitHub: <https://github.com/thienty1207/Go_getajob_Ferris>
- Branch chính: `main`
- Brand: **Sugoi-oniichan**
- Website: **Go get a job ferris**
- Thị trường đầu tiên: Việt Nam
- Khoảng cách: km
- Tiền tệ: giữ nguyên theo source, không tự convert
- Database local: PostgreSQL

Các thư mục chính:

```text
frontend/   SvelteKit + TypeScript + Bun
backend/    Go + Gin + pgx
crawler/    Rust + Tokio + tokio-postgres
database/   PostgreSQL migrations, fixtures, schema checks
idea/       Product context, business rules, UI demo
img/        Brand assets
docs/       Architecture, plans, specs, verification records
```

## 2. Đọc trước khi làm việc

```text
AGENTS.md
idea/GO_GET_A_JOB_FERRIS.md
backend/README.md
crawler/README.md
database/README.md
docs/baron/architecture/PROJECT_PROFILE.md
docs/baron/architecture/CURRENT_ARCHITECTURE.md
docs/superpowers/plans/2026-08-17-admin-management-and-crawler-runtime.md
```

Sau đó kiểm tra trạng thái thật:

```powershell
git status --short --branch
git log -3 --oneline
```

## 3. Product rules bắt buộc

1. Tuyệt đối không tạo mock data để che API hoặc database chưa có.
2. Khi API chưa sẵn sàng, frontend phải dùng empty/loading/error/disabled state.
3. Crawler nằm ở `crawler/`, là source-controlled, allowlist-only, delta/reconcile crawler.
4. Crawler chỉ crawl Job Link đã tồn tại và đang `ACTIVE` trong database.
5. Không viết adapter riêng cho từng website như `fpt.rs`, `viettel.rs`.
6. Crawler phải giữ same-origin và URL/path scope của Job Link.
7. HTTP error, 500, source outage hoặc parser error không được tính là missing job.
8. Chỉ healthy crawl mới được reconcile missing jobs.
9. Missing lifecycle: healthy missing lần một là `VERIFYING`; hai lần liên tiếp là `CLOSED`; source tuyên bố đóng thì `CLOSED` ngay.
10. CV chỉ lưu structured profile; không lưu raw CV lâu dài.
11. Job chỉ lưu metadata, structured fields và `content_hash`; không lưu full raw JD lâu dài.
12. Không gửi PII không cần thiết sang DeepSeek.
13. CV Match % deterministic: required skills 35%, role relevance 25%, experience 15%, seniority 15%, preferred skills/domain 10%.
14. `job_cache.location_id` là canonical location authority.
15. Public filter, distance và matching phải dùng canonical `location_id` sau khi admin chỉnh location.
16. Admin authentication và client authentication là hai hệ thống tách biệt.
17. Không commit `.env`, password, API key, Cloudinary secret hoặc file runtime cá nhân.

## 4. Hiện trạng đã triển khai

### Client

Route chính: `/`.

Đã có CV upload, promotion carousel tối đa 3 ảnh, thumbnail 16:9, brand logo Sugoi-oniichan, website name Go get a job ferris, chọn location/bán kính, khu vực match result và API thật. Không dùng mock rows.

### Admin navigation

```text
Overview
Promotions

Manage Job
  Job Link
  Job Location
  Job Cache

Manage User
  Users
  CV Profiles

Settings
```

Sidebar đóng/mở được ở desktop và mobile. User/email và logout nằm cuối sidebar.

### Job Link

Route: `/admin/sources`. Tên hiển thị: **Job Link**.

Đã có thêm, sửa, xoá vĩnh viễn, dừng/khởi động, `Crawl ngay`, queue crawl request, loading/error/success state và phân trang tối đa 10 link/trang.

### Job Location

Route: `/admin/locations`. Trang này chỉ thêm, sửa, bật/tắt canonical location, xem số job và phân trang tối đa 10 location/trang. Việc gán job nằm trong Job Cache, không đưa panel gán job trở lại đây.

### Job Cache

Route: `/admin/jobs`.

Đã có search server-side theo title/company/role/location/source/original URL, lọc location, lọc job chưa gán location, inline dropdown gán canonical location, tối đa 10 job/trang và chỉ một lifecycle badge.

API quan trọng:

```text
GET   /api/v1/admin/jobs?q=&location_id=&unresolved=true&page=1&page_size=10
GET   /api/v1/admin/locations/options
PATCH /api/v1/admin/jobs/:id/location
```

`job_cache.location_id` là write boundary duy nhất cho việc sửa location của job.

### Settings và crawler runtime

Route: `/admin/settings`. Admin nhập crawler interval bằng giờ/phút. Khoảng hiện tại là tối thiểu 15 phút và tối đa 7 ngày.

Migration runtime:

```text
database/migrations/000009_admin_management_runtime.up.sql
database/migrations/000009_admin_management_runtime.down.sql
```

Bảng `public.crawler_runtime` lưu `OFFLINE`, `IDLE`, `RUNNING`, `ERROR`, heartbeat, cycle timestamps, source hiện tại, `next_cycle_at` và lỗi gần nhất. Rust crawler heartbeat mỗi 5 giây khi chạy và reload interval từ PostgreSQL, không cần restart sau khi đổi Settings.

## 5. Crawler limitation hiện tại

Crawler generic mặc định đọc server-rendered `JobPosting` JSON-LD. Trang listing render bằng JavaScript hoặc lấy dữ liệu từ endpoint riêng có thể trả `ANOMALY / extraction_not_authoritative`; FPT trước đó gặp limitation này.

Không giải quyết bằng `fpt.rs`. Nếu làm tiếp, phải thiết kế generic discovery/extraction contract, vẫn giữ allowlist, same-origin và path scope. Không chạy live crawl khi chỉ đang onboarding vì crawl thật sẽ thay đổi crawl runs/job cache trong PostgreSQL.

## 6. Cách chạy local

Database phải chạy trước.

```powershell
# Terminal backend
cd D:/Work/Project/Website/GoGetSomeFoodFerris/backend
go run ./cmd/api/

# Terminal frontend
cd D:/Work/Project/Website/GoGetSomeFoodFerris/frontend
bun run dev

# Terminal crawler
cd D:/Work/Project/Website/GoGetSomeFoodFerris/crawler
cargo run
```

Chạy một crawler cycle có giới hạn:

```powershell
cd D:/Work/Project/Website/GoGetSomeFoodFerris/crawler
cargo run -- --once
```

URL thường dùng: frontend `http://127.0.0.1:5173`, backend `http://127.0.0.1:8080`, health `GET /healthz`.

## 7. Verification đã pass ở handoff gần nhất

```text
database schema validation             PASS
go test ./... -count=1                 PASS
go vet ./...                           PASS
cargo test                             PASS
cargo check                            PASS
PostgreSQL crawler contract            1 passed
svelte-check                           0 errors, 0 warnings
frontend unit tests                    43 passed
frontend production build              PASS
API health and unauthenticated 401     PASS
Baron trace                            standard/standard, passed
```

## 8. Quy trình tiếp quản

1. Đọc file này và các file context ở mục 2.
2. Chạy `git status --short --branch`.
3. Xác định task thuộc frontend, backend, database hay crawler.
4. Đọc đúng module liên quan, không dump toàn bộ project.
5. Không sửa code nếu chưa nêu ngắn nguyên nhân và phạm vi.
6. Nếu thay đổi behavior/API/database, viết hoặc cập nhật test trước.
7. Chạy verification tương ứng sau khi sửa.
8. Báo cáo file đã đổi, test đã chạy và limitation còn lại.
9. Chỉ commit/push khi user yêu cầu rõ.

Source comments chỉ dùng cho invariant hoặc logic không hiển nhiên. Không thêm explanatory copy hoặc chú thích trang trí vô nghĩa vào UI.

## 9. Việc nên làm tiếp theo

Ưu tiên kiểm chứng end-to-end trên một Job Link do user cấp:

```text
Admin add Job Link
→ bấm Crawl ngay
→ crawler nhận crawl request
→ crawl run được ghi nhận
→ structured jobs vào job_cache
→ admin gán canonical Job Location trong Job Cache
→ client lọc/match theo canonical location
```

Nếu nguồn client-rendered vẫn trả `extraction_not_authoritative`, hãy thiết kế generic extraction/discovery contract trước. Không tạo mock job và không viết crawler riêng cho website đó.
