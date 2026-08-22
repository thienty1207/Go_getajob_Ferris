package repository

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
)

func TestScanRepositoryContract(t *testing.T) {
	var repository ScanRepository = newTestScanRepository()

	locationID := uuid.New()
	id, err := repository.CreateScan(context.Background(), locationID, 25)
	if err != nil {
		t.Fatalf("CreateScan() error = %v", err)
	}
	errorCode := "parser_not_configured"
	if err := repository.SetStatus(context.Background(), id, model.StatusFailed, &errorCode); err != nil {
		t.Fatalf("SetStatus() error = %v", err)
	}

	scan, err := repository.GetScan(context.Background(), id)
	if err != nil {
		t.Fatalf("GetScan() error = %v", err)
	}
	if scan.ID != id || scan.Status != model.StatusFailed || scan.ErrorCode != errorCode {
		t.Fatalf("GetScan() = %#v, want failed scan %s", scan, errorCode)
	}
}

func TestScanRepositoryReturnsNotFound(t *testing.T) {
	var repository ScanRepository = newTestScanRepository()
	_, err := repository.GetScan(context.Background(), uuid.New())
	if !errors.Is(err, ErrScanNotFound) {
		t.Fatalf("GetScan() error = %v, want ErrScanNotFound", err)
	}
}

func TestPublicMatchQueryUsesApprovedActiveView(t *testing.T) {
	query := strings.ToLower(completedMatchesQuery)
	if !strings.Contains(query, "active_job_cache") || !strings.Contains(query, "jobs.status = 'active'") {
		t.Fatal("completed match query must read through active_job_cache")
	}
	if strings.Contains(query, "from public.job_cache") || strings.Contains(query, "join public.job_cache") {
		t.Fatal("completed match query must not expose the base job_cache table directly")
	}
	if !strings.Contains(query, "limit 100") {
		t.Fatal("completed match query must cap public results at 100 rows")
	}
}

func TestMatchCandidateQueryUsesCanonicalLocationOnly(t *testing.T) {
	query := strings.ToLower(listMatchCandidatesQuery)
	for _, required := range []string{"active_job_cache", "location_id", "jobs.location_id = scan_context.location_id", "jobs.status = 'active'"} {
		if !strings.Contains(query, required) {
			t.Fatalf("candidate query missing %q: %s", required, listMatchCandidatesQuery)
		}
	}
	for _, forbidden := range []string{"radius_km", "6371", "work_mode = 'remote'"} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("candidate query still applies deprecated radius/remote rule %q: %s", forbidden, listMatchCandidatesQuery)
		}
	}
	if strings.Contains(query, "description") || strings.Contains(query, "raw") {
		t.Fatalf("candidate query exposes raw job content: %s", listMatchCandidatesQuery)
	}
}

func TestScanQueriesPersistAndEnforceClientOwnership(t *testing.T) {
	createQuery := strings.ToLower(createScanQuery)
	if !strings.Contains(createQuery, "client_user_id") || !strings.Contains(createQuery, "$2") {
		t.Fatalf("create scan query must persist client owner: %s", createScanQuery)
	}
	getQuery := strings.ToLower(getScanQuery)
	for _, required := range []string{"client_user_id", "$2::uuid", "client_user_id = $2"} {
		if !strings.Contains(getQuery, required) {
			t.Fatalf("get scan query missing ownership guard %q: %s", required, getScanQuery)
		}
	}
}

func TestCompleteScanQueryPersistsOnlyStructuredProfileFields(t *testing.T) {
	query := strings.ToLower(completeScanProfileQuery + completeScanStatusQuery + insertScanMatchQuery + finishScanQuery)
	for _, required := range []string{"structured_profiles", "roles", "skills", "years_of_experience", "seniority", "domains", "education", "certifications", "scan_matches", "match_percent", "status = 'completed'"} {
		if !strings.Contains(query, required) {
			t.Fatalf("complete scan queries missing %q: %s", required, query)
		}
	}
	for _, forbidden := range []string{"raw_cv", "raw_jd", "description", "email", "phone"} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("complete scan queries contain forbidden field %q: %s", forbidden, query)
		}
	}
}

func TestCreateScanQueryEntersParsingAtomically(t *testing.T) {
	query := strings.ToLower(createScanQuery)
	parsingQuery := strings.ToLower(markParsingStatusQuery)
	if !strings.Contains(query, "insert into public.scans") || !strings.Contains(query, "select 'received'") || !strings.Contains(query, "from public.locations") || !strings.Contains(query, "returning id") {
		t.Fatalf("create scan query = %q, want canonical location insert", createScanQuery)
	}
	if !strings.Contains(parsingQuery, "update public.scans") || !strings.Contains(parsingQuery, "status = 'parsing'") || !strings.Contains(parsingQuery, "status = 'received'") {
		t.Fatalf("parsing transition query = %q, want guarded received-to-parsing update", markParsingStatusQuery)
	}
}

func TestStatusQueryGuardsLegalTransitions(t *testing.T) {
	query := strings.ToLower(setScanStatusQuery)
	for _, required := range []string{"status = 'received'", "status = 'parsing'", "status = 'matching'", "$2 in ('completed'"} {
		if !strings.Contains(query, required) {
			t.Fatalf("status query does not guard %q: %q", required, setScanStatusQuery)
		}
	}
}

func TestStatusQueryAllowsIdempotentFailedRetry(t *testing.T) {
	query := strings.ToLower(setScanStatusQuery)
	if !strings.Contains(query, "status = 'failed'") || !strings.Contains(query, "error_code = $3") {
		t.Fatalf("status query = %q, want idempotent failed retry guard", setScanStatusQuery)
	}
}

func TestCreateScanHasCommitRecoveryQuery(t *testing.T) {
	query := strings.ToLower(verifyCommittedScanQuery)
	if !strings.Contains(query, "select status") || !strings.Contains(query, "from public.scans") || !strings.Contains(query, "where id = $1") {
		t.Fatalf("commit recovery query = %q, want status verification by scan ID", verifyCommittedScanQuery)
	}
}

func TestScanModelDoesNotContainRawPersistenceFields(t *testing.T) {
	for _, field := range []string{"RawCV", "RawCVPath", "FullJD", "Description", "SourcePayload"} {
		if strings.Contains(strings.ToLower(fieldNames(model.Scan{})), strings.ToLower(field)) {
			t.Fatalf("model.Scan unexpectedly contains %q", field)
		}
	}
}

func fieldNames(value model.Scan) string {
	typ := reflect.TypeOf(value)
	fields := make([]string, 0, typ.NumField())
	for index := 0; index < typ.NumField(); index++ {
		fields = append(fields, typ.Field(index).Name)
	}
	return strings.Join(fields, ",")
}

type testScanRepository struct {
	scans map[uuid.UUID]model.Scan
}

func newTestScanRepository() *testScanRepository {
	return &testScanRepository{scans: make(map[uuid.UUID]model.Scan)}
}

func (r *testScanRepository) CreateScan(_ context.Context, locationID uuid.UUID, radiusKm float64) (uuid.UUID, error) {
	id := uuid.New()
	r.scans[id] = model.Scan{ID: id, Status: model.StatusReceived, LocationID: &locationID, RadiusKm: radiusKm}
	return id, nil
}

func (r *testScanRepository) SetStatus(_ context.Context, id uuid.UUID, status model.ScanStatus, errorCode *string) error {
	scan, ok := r.scans[id]
	if !ok {
		return ErrScanNotFound
	}
	scan.Status = status
	scan.ErrorCode = ""
	if errorCode != nil {
		scan.ErrorCode = *errorCode
	}
	r.scans[id] = scan
	return nil
}

func (r *testScanRepository) GetScan(_ context.Context, id uuid.UUID) (model.Scan, error) {
	scan, ok := r.scans[id]
	if !ok {
		return model.Scan{}, ErrScanNotFound
	}
	return scan, nil
}
