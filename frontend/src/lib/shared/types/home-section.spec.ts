import { describe, expect, it } from 'vitest';
import { createEditableHomeSectionSlots } from './home-section';

describe('editable Home section slots', () => {
	it('keeps all four editor slots visible when the database has no section rows', () => {
		const slots = createEditableHomeSectionSlots([]);

		expect(slots.map((section) => section.slot)).toEqual([1, 2, 3, 4]);
		expect(slots[0]).toMatchObject({ slot: 1, layout: 'CONTENT_LEFT', isActive: false, title: '', body: '', media: [] });
		expect(slots[1]).toMatchObject({ slot: 2, layout: 'IMAGE_LEFT', isActive: false, title: '', body: '', media: [] });
		expect(slots[3]).toMatchObject({ slot: 4, layout: 'MEDIA_STRIP', isActive: false, media: [] });
	});
});
