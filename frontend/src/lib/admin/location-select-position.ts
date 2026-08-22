/**
 * Thuần tính toán vị trí cho menu Location dropdown.
 *
 * Menu được render dạng `position: fixed` trong viewport (thoát khỏi mọi
 * ancestor bị clip: `.job-table-wrap` overflow-x, `.admin-root` overflow:hidden).
 * Vì fixed nên tọa độ phải dựa trên rect của trigger đối với viewport, chứ không
 * phải offset của container.
 */

export const MENU_GAP = 4; // cách mép trigger
export const MENU_MAX_HEIGHT = 200; // tương đương max-height cũ khi menu tự scroll
export const VIEWPORT_EDGE_GAP = 8; // giữ khoảng cách an toàn với mép viewport

export interface Rect {
	top: number;
	left: number;
	width: number;
	bottom: number;
}

export interface ViewportSize {
	width: number;
	height: number;
}

export interface MenuPosition {
	openUp: boolean;
	width: number;
	left: number;
	top: number | undefined;
	bottom: number | undefined;
	maxHeight: number;
}

export function positionMenu(trigger: Rect, viewport: ViewportSize): MenuPosition {
	const spaceBelow = viewport.height - trigger.bottom;
	const spaceAbove = trigger.top;

	const width = Math.min(trigger.width, viewport.width - VIEWPORT_EDGE_GAP * 2);

	// Kẹp left trong giới hạn viewport; trigger.left đã là tọa độ viewport (fixed).
	let left = Math.min(trigger.left, viewport.width - VIEWPORT_EDGE_GAP - width);
	left = Math.max(VIEWPORT_EDGE_GAP, left);

	// Chỉ lật lên khi phía dưới không đủ chỗ và phía trên thực sự nhiều hơn phía dưới,
	// để tránh vượt ngang cả table đến một trigger ở hàng đầu.
	const openUp = spaceBelow < MENU_GAP + MENU_MAX_HEIGHT && spaceAbove > spaceBelow;

	if (openUp) {
		const maxHeight = Math.max(1, Math.min(MENU_MAX_HEIGHT, spaceAbove - MENU_GAP - VIEWPORT_EDGE_GAP));
		const bottom = viewport.height - trigger.top + MENU_GAP;
		return { openUp, width, left, top: undefined, bottom, maxHeight };
	}

	const maxHeight = MENU_MAX_HEIGHT;
	const top = trigger.bottom + MENU_GAP;
	return { openUp, width, left, top, bottom: undefined, maxHeight };
}
