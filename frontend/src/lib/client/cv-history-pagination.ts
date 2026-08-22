// A delete can remove the only row on the last page. Clamp to the new last
// page so the UI does not incorrectly present a global "no CV" state.
export function clampCVHistoryPage(currentPage: number, total: number, pageSize: number): number {
	if (!Number.isInteger(currentPage) || currentPage < 1 || !Number.isInteger(total) || total < 0 || !Number.isInteger(pageSize) || pageSize < 1) {
		return 1;
	}
	return Math.min(currentPage, Math.max(1, Math.ceil(total / pageSize)));
}
