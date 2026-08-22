package repository

import (
	"context"
	"strings"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

const countAdminClientUsersQuery = `
SELECT count(*)
FROM public.client_users
WHERE $1::text = ''
   OR email ILIKE '%' || $1::text || '%'
   OR display_name ILIKE '%' || $1::text || '%'
   OR id::text ILIKE '%' || $1::text || '%'`

const listAdminClientUsersQuery = `
SELECT id, email, display_name, avatar_url, provider, created_at, last_login_at
FROM public.client_users
WHERE $1::text = ''
   OR email ILIKE '%' || $1::text || '%'
   OR display_name ILIKE '%' || $1::text || '%'
   OR id::text ILIKE '%' || $1::text || '%'
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3`

type PostgresAdminClientUserRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresAdminClientUserRepository(pool *pgxpool.Pool) *PostgresAdminClientUserRepository {
	return &PostgresAdminClientUserRepository{pool: pool}
}

func (r *PostgresAdminClientUserRepository) ListAdminClientUsers(ctx context.Context, page, pageSize int, filter AdminClientUserFilter) (model.AdminClientUserPage, error) {
	search := strings.TrimSpace(filter.Search)
	var total int
	if err := r.pool.QueryRow(ctx, countAdminClientUsersQuery, search).Scan(&total); err != nil {
		return model.AdminClientUserPage{}, err
	}
	rows, err := r.pool.Query(ctx, listAdminClientUsersQuery, search, pageSize, (page-1)*pageSize)
	if err != nil {
		return model.AdminClientUserPage{}, err
	}
	defer rows.Close()
	pageResult := model.AdminClientUserPage{Page: page, PageSize: pageSize, Total: total, Items: make([]model.AdminClientUser, 0)}
	for rows.Next() {
		var user model.AdminClientUser
		if err := rows.Scan(&user.ID, &user.Email, &user.DisplayName, &user.AvatarURL, &user.Provider, &user.CreatedAt, &user.LastLoginAt); err != nil {
			return model.AdminClientUserPage{}, err
		}
		pageResult.Items = append(pageResult.Items, user)
	}
	if err := rows.Err(); err != nil {
		return model.AdminClientUserPage{}, err
	}
	return pageResult, nil
}

var _ AdminClientUserRepository = (*PostgresAdminClientUserRepository)(nil)
