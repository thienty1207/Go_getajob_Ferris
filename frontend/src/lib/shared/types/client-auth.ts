export interface ClientUser {
	id: string;
	email: string;
	displayName: string;
	avatarUrl?: string;
	provider: 'google';
	createdAt: string;
	lastLoginAt: string;
}
