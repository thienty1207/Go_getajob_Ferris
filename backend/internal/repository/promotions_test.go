package repository

import (
	"strings"
	"testing"
)

func TestPromotionRepositoryContractQueries(t *testing.T) {
	listQuery := strings.ToLower(listActivePromotionsQuery)
	for _, required := range []string{"from public.promotion_slides", "is_active = true", "order by slot asc", "limit 3"} {
		if !strings.Contains(listQuery, required) {
			t.Fatalf("list query missing %q: %s", required, listActivePromotionsQuery)
		}
	}
	if strings.Contains(listQuery, "image_bytes") {
		t.Fatal("metadata query must not select image_bytes")
	}

	imageQuery := strings.ToLower(activePromotionImageQuery)
	for _, required := range []string{"image_bytes", "mime_type", "content_hash", "where slot = $1", "is_active = true"} {
		if !strings.Contains(imageQuery, required) {
			t.Fatalf("image query missing %q: %s", required, activePromotionImageQuery)
		}
	}

	upsertQuery := strings.ToLower(upsertPromotionQuery)
	for _, required := range []string{"insert into public.promotion_slides", "on conflict (slot)", "$1", "$2", "$3", "returning"} {
		if !strings.Contains(upsertQuery, required) {
			t.Fatalf("upsert query missing %q: %s", required, upsertPromotionQuery)
		}
	}
	if strings.Contains(upsertQuery, "fmt.sprintf") || strings.Contains(upsertQuery, "+") {
		t.Fatal("promotion SQL must not be built with string concatenation")
	}
}

func TestPromotionRepositoryDeleteIsIdempotent(t *testing.T) {
	query := strings.ToLower(deletePromotionQuery)
	if !strings.Contains(query, "delete from public.promotion_slides") || !strings.Contains(query, "where slot = $1") {
		t.Fatalf("delete query = %s, want slot-parameterized delete", deletePromotionQuery)
	}
}
