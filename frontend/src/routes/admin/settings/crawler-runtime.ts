export function countdownSeconds(nextCycleAt: string | undefined, now = Date.now()): number | null {
	if (!nextCycleAt) return null;
	const target = Date.parse(nextCycleAt);
	if (!Number.isFinite(target)) return null;
	return Math.max(0, Math.ceil((target - now) / 1000));
}

export function formatCountdown(seconds: number): string {
	const safeSeconds = Math.max(0, Math.floor(seconds));
	const hours = Math.floor(safeSeconds / 3600);
	const minutes = Math.floor((safeSeconds % 3600) / 60);
	const remainingSeconds = safeSeconds % 60;
	return [hours, minutes, remainingSeconds].map((value) => String(value).padStart(2, '0')).join(':');
}
