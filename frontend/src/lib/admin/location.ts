/**
 * Chuyển giá trị location option sang payload gửi lên API.
 *
 * Backend (AssignJobLocation) chỉ chấp nhận `location_id` là một UUID string
 * hoặc `null` (để clear location). Chuỗi rỗng (`""`) làm `uuid.Parse("")` fail
 * -> 400 invalid_location_id, nên phải trả về `null`.
 */
export function toLocationPayload(id: string | null | undefined): string | null {
	return id ? id : null;
}
