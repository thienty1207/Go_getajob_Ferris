package repository

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
)

func TestListHomeSectionsQueryKeepsAdminDraftsAndFiltersPublicRows(t *testing.T) {
	if !strings.Contains(listHomeSectionsQuery, "WHERE NOT $1::boolean") {
		t.Fatal("home section list query must treat publicOnly=true as the active-content filter")
	}
	if strings.Contains(listHomeSectionsQuery, "WHERE $1::boolean") {
		t.Fatal("home section list query must not treat publicOnly=true as include-all")
	}
	if !strings.Contains(listHomeSectionsQuery, "hs.is_active = true") {
		t.Fatal("public home section reads must filter inactive sections in PostgreSQL")
	}
}

func TestHomeSectionMediaReadScopeKeepsDraftMediaAdminOnly(t *testing.T) {
	if includeInactiveForHomeRead(true) {
		t.Fatal("public Home reads must exclude inactive media")
	}
	if !includeInactiveForHomeRead(false) {
		t.Fatal("admin Home reads must include inactive media drafts")
	}
}

func TestPostgresHomeSectionUpsertEnqueuesSupersededAssetBeforeCommit(t *testing.T) {
	oldPublicID := "ferris/home-sections/slot-1-old"
	newPublicID := "ferris/home-sections/slot-1-new"
	tx := &scriptedHomeSectionTx{rows: []homeRowScanner{
		&scriptedHomeRow{values: []any{stringPointer(oldPublicID)}},
		&scriptedHomeRow{values: homeSectionRowValues(newPublicID)},
	}}
	repo := &PostgresHomeSectionRepository{db: &scriptedHomeSectionDB{tx: tx}}

	section, err := repo.UpsertHomeSection(context.Background(), model.HomeSectionWrite{
		Slot: 1, Layout: model.HomeSectionContentLeft, IsActive: true,
		CloudinaryPublicID: stringPointer(newPublicID),
	})
	if err != nil {
		t.Fatalf("UpsertHomeSection() error = %v", err)
	}
	if section.CloudinaryPublicID != newPublicID {
		t.Fatalf("UpsertHomeSection() public ID = %q, want %q", section.CloudinaryPublicID, newPublicID)
	}
	if len(tx.execArguments) != 1 || len(tx.execArguments[0]) != 1 || tx.execArguments[0][0] != oldPublicID {
		t.Fatalf("cleanup enqueue args = %#v, want old public ID", tx.execArguments)
	}
	if strings.Join(tx.events, ",") != "lock-section,upsert-section,enqueue-cleanup,commit" {
		t.Fatalf("transaction events = %#v, want metadata write and enqueue before one commit", tx.events)
	}
}

func TestPostgresHomeSectionDeleteEnqueuesAssetBeforeCommit(t *testing.T) {
	mediaID := uuid.New()
	publicID := "ferris/home-sections/slot-4-owned"
	tx := &scriptedHomeSectionTx{rows: []homeRowScanner{
		&scriptedHomeRow{values: homeSectionMediaRowValues(mediaID, publicID)},
	}}
	repo := &PostgresHomeSectionRepository{db: &scriptedHomeSectionDB{tx: tx}}

	deleted, err := repo.DeleteHomeSectionMedia(context.Background(), mediaID)
	if err != nil {
		t.Fatalf("DeleteHomeSectionMedia() error = %v", err)
	}
	if deleted.ID != mediaID || deleted.CloudinaryPublicID != publicID {
		t.Fatalf("DeleteHomeSectionMedia() = %#v, want deleted media metadata", deleted)
	}
	if len(tx.execArguments) != 1 || tx.execArguments[0][0] != publicID {
		t.Fatalf("cleanup enqueue args = %#v, want deleted public ID", tx.execArguments)
	}
	if strings.Join(tx.events, ",") != "delete-media,enqueue-cleanup,commit" {
		t.Fatalf("transaction events = %#v, want delete and enqueue before one commit", tx.events)
	}
}

func TestPostgresHomeSectionMutationRollsBackWhenCleanupCannotBeEnqueued(t *testing.T) {
	oldPublicID := "ferris/home-sections/slot-1-old"
	newPublicID := "ferris/home-sections/slot-1-new"
	tx := &scriptedHomeSectionTx{
		rows: []homeRowScanner{
			&scriptedHomeRow{values: []any{stringPointer(oldPublicID)}},
			&scriptedHomeRow{values: homeSectionRowValues(newPublicID)},
		},
		execErr: errors.New("queue unavailable"),
	}
	repo := &PostgresHomeSectionRepository{db: &scriptedHomeSectionDB{tx: tx}}

	_, err := repo.UpsertHomeSection(context.Background(), model.HomeSectionWrite{
		Slot: 1, Layout: model.HomeSectionContentLeft,
		CloudinaryPublicID: stringPointer(newPublicID),
	})
	if err == nil {
		t.Fatal("UpsertHomeSection() error = nil, want atomic enqueue failure")
	}
	if tx.commitCalls != 0 || tx.rollbackCalls != 1 {
		t.Fatalf("commitCalls=%d rollbackCalls=%d, want rollback without commit", tx.commitCalls, tx.rollbackCalls)
	}
}

type scriptedHomeSectionDB struct {
	tx *scriptedHomeSectionTx
}

func (db *scriptedHomeSectionDB) Begin(context.Context) (homeSectionTransaction, error) {
	return db.tx, nil
}

func (*scriptedHomeSectionDB) Query(context.Context, string, ...any) (homeSectionRows, error) {
	return nil, errors.New("unexpected Query call")
}

func (*scriptedHomeSectionDB) QueryRow(context.Context, string, ...any) homeRowScanner {
	return &scriptedHomeRow{err: errors.New("unexpected QueryRow call")}
}

func (*scriptedHomeSectionDB) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected Exec call")
}

type scriptedHomeSectionTx struct {
	rows          []homeRowScanner
	execErr       error
	events        []string
	execArguments [][]any
	commitCalls   int
	rollbackCalls int
}

func (tx *scriptedHomeSectionTx) QueryRow(_ context.Context, query string, _ ...any) homeRowScanner {
	switch query {
	case lockHomeSectionAssetQuery:
		tx.events = append(tx.events, "lock-section")
	case upsertHomeSectionQuery:
		tx.events = append(tx.events, "upsert-section")
	case deleteHomeSectionMediaQuery:
		tx.events = append(tx.events, "delete-media")
	default:
		tx.events = append(tx.events, "unexpected-query")
	}
	if len(tx.rows) == 0 {
		return &scriptedHomeRow{err: errors.New("no scripted row")}
	}
	row := tx.rows[0]
	tx.rows = tx.rows[1:]
	return row
}

func (tx *scriptedHomeSectionTx) Exec(_ context.Context, query string, arguments ...any) error {
	if query == enqueueHomeAssetCleanupQuery {
		tx.events = append(tx.events, "enqueue-cleanup")
	} else {
		tx.events = append(tx.events, "unexpected-exec")
	}
	tx.execArguments = append(tx.execArguments, append([]any(nil), arguments...))
	return tx.execErr
}

func (tx *scriptedHomeSectionTx) Commit(context.Context) error {
	tx.commitCalls++
	tx.events = append(tx.events, "commit")
	return nil
}

func (tx *scriptedHomeSectionTx) Rollback(context.Context) error {
	tx.rollbackCalls++
	return nil
}

type scriptedHomeRow struct {
	values []any
	err    error
}

func (row *scriptedHomeRow) Scan(destinations ...any) error {
	if row.err != nil {
		return row.err
	}
	if len(destinations) != len(row.values) {
		return fmt.Errorf("scan destination count = %d, values = %d", len(destinations), len(row.values))
	}
	for index, destination := range destinations {
		target := reflect.ValueOf(destination)
		if target.Kind() != reflect.Pointer || target.IsNil() {
			return fmt.Errorf("scan destination %d is not a pointer", index)
		}
		if row.values[index] == nil {
			target.Elem().Set(reflect.Zero(target.Elem().Type()))
			continue
		}
		value := reflect.ValueOf(row.values[index])
		if !value.Type().AssignableTo(target.Elem().Type()) {
			return fmt.Errorf("scan value %d type %s is not assignable to %s", index, value.Type(), target.Elem().Type())
		}
		target.Elem().Set(value)
	}
	return nil
}

func homeSectionRowValues(publicID string) []any {
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)
	return []any{
		uuid.New(), int16(1), model.HomeSectionContentLeft, true,
		stringPointer("eyebrow"), stringPointer("title"), stringPointer("body"), stringPointer("alt"),
		stringPointer(strings.Repeat("a", 64)), stringPointer("CLOUDINARY"), stringPointer(publicID),
		stringPointer("https://res.cloudinary.com/example/image/upload/home.png"), stringPointer("asset-id"),
		(*string)(nil), stringPointer("admin@example.com"), now, now,
	}
}

func homeSectionMediaRowValues(id uuid.UUID, publicID string) []any {
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)
	return []any{
		id, int16(0), true, "alt", strings.Repeat("b", 64), "CLOUDINARY", publicID,
		"https://res.cloudinary.com/example/image/upload/media.png", "asset-id", (*string)(nil), now, now,
	}
}

func stringPointer(value string) *string { return &value }
