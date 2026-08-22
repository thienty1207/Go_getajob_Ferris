package repository

import (
	"strings"
	"testing"
)

func TestClientCVHistoryQueryReturnsStructuredProfileOnly(t *testing.T) {
	query := strings.ToLower(listClientCVHistoryQuery)
	for _, required := range []string{"scans", "client_user_id", "structured_profiles", "scan_matches", "roles", "skills", "education", "certifications", "order by scans.created_at desc"} {
		if !strings.Contains(query, required) {
			t.Fatalf("history query missing %q: %s", required, listClientCVHistoryQuery)
		}
	}
	for _, forbidden := range []string{"raw_cv", "raw_path", "email", "phone", "address"} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("history query exposes forbidden field %q: %s", forbidden, listClientCVHistoryQuery)
		}
	}
}

func TestDeleteClientCVQueriesScopeByOwnerAndCleanOrphanProfile(t *testing.T) {
	scanQuery := strings.ToLower(deleteClientCVQuery)
	for _, required := range []string{"delete from public.scans", "client_user_id = $2", "returning profile_id"} {
		if !strings.Contains(scanQuery, required) {
			t.Fatalf("scan delete query missing %q: %s", required, deleteClientCVQuery)
		}
	}
	lockQuery := strings.ToLower(lockCVProfileQuery)
	for _, required := range []string{"select id", "from public.structured_profiles", "for update"} {
		if !strings.Contains(lockQuery, required) {
			t.Fatalf("profile lock query missing %q: %s", required, lockCVProfileQuery)
		}
	}
	profileQuery := strings.ToLower(deleteOrphanCVProfileQuery)
	for _, required := range []string{"delete from public.structured_profiles", "not exists", "remaining_scans.profile_id = profiles.id"} {
		if !strings.Contains(profileQuery, required) {
			t.Fatalf("profile delete query missing %q: %s", required, deleteOrphanCVProfileQuery)
		}
	}
}
