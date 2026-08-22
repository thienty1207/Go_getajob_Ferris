import { describe, expect, it } from 'vitest';
import { positionMenu, MENU_GAP, MENU_MAX_HEIGHT, VIEWPORT_EDGE_GAP } from './location-select-position';

const viewport = { width: 1280, height: 800 };

describe('positionMenu', () => {
	it('mở xuống dưới trigger khi có đủ không gian bên dưới', () => {
		const pos = positionMenu({ top: 100, left: 300, width: 180, bottom: 130 }, viewport);
		expect(pos.openUp).toBe(false);
		expect(pos.top).toBe(130 + MENU_GAP);
		expect(pos.maxHeight).toBe(MENU_MAX_HEIGHT);
	});

	it('lật lên trên khi không đủ không gian bên dưới (row cuối gần đáy viewport)', () => {
		// Trigger nằm sát đáy viewport: bottom = 790, chỉ còn 10px phía dưới.
		const pos = positionMenu({ top: 760, left: 300, width: 180, bottom: 790 }, viewport);
		expect(pos.openUp).toBe(true);
		expect(pos.bottom).toBe(viewport.height - 760 + MENU_GAP);
		expect(pos.top).toBeUndefined();
	});

	it('mở xuống với maxHeight mặc định khi phía dưới đủ chỗ', () => {
		const pos = positionMenu({ top: 100, left: 300, width: 180, bottom: 130 }, viewport);
		expect(pos.openUp).toBe(false);
		expect(pos.top).toBe(130 + MENU_GAP);
		expect(pos.maxHeight).toBe(MENU_MAX_HEIGHT);
	});

	it('giới hạn maxHeight theo không gian thật phía trên khi lật lên', () => {
		// Đáy, phía dưới 10px; nhưng chỉ còn 60px phía trên trigger.
		const pos = positionMenu({ top: 60, left: 300, width: 180, bottom: 790 }, viewport);
		expect(pos.openUp).toBe(true);
		expect(pos.maxHeight).toBeLessThanOrEqual(60 - MENU_GAP - VIEWPORT_EDGE_GAP);
		expect(pos.maxHeight).toBeGreaterThan(0);
	});

	it('kẹp left để menu không tràn ra ngoài mép phải viewport', () => {
		const pos = positionMenu({ top: 100, left: 1250, width: 180, bottom: 130 }, viewport);
		expect(pos.left + pos.width).toBeLessThanOrEqual(viewport.width - VIEWPORT_EDGE_GAP);
		expect(pos.left).toBeGreaterThanOrEqual(VIEWPORT_EDGE_GAP);
	});

	it('kẹp left khi trigger bị cuộn ngang ra ngoài mép trái (table overflow-x)', () => {
		const pos = positionMenu({ top: 100, left: -40, width: 180, bottom: 130 }, viewport);
		expect(pos.left).toBe(VIEWPORT_EDGE_GAP);
	});

	it('thu hẹp width khi viewport hẹp hơn trigger', () => {
		const narrow = { width: 200, height: 600 };
		const pos = positionMenu({ top: 100, left: 10, width: 180, bottom: 130 }, narrow);
		expect(pos.width).toBeLessThanOrEqual(narrow.width - VIEWPORT_EDGE_GAP * 2);
		expect(pos.left).toBeGreaterThanOrEqual(VIEWPORT_EDGE_GAP);
		expect(pos.left + pos.width).toBeLessThanOrEqual(narrow.width - VIEWPORT_EDGE_GAP);
	});
});
