package repository

import (
	"strings"
	"testing"
)

func TestJobLinkQueriesUseBoundParametersAndSeparateDeleteAndStatus(t *testing.T) {
	queries := []string{listJobLinksQuery, createJobLinkQuery, updateJobLinkQuery, disableJobLinkQuery, deleteJobLinkQuery}
	for _, query := range queries {
		if !strings.Contains(query, "$1") {
			t.Fatalf("query has no positional parameter: %s", query)
		}
	}
	if !strings.Contains(strings.ToUpper(disableJobLinkQuery), "APPROVAL_STATUS = 'DISABLED'") {
		t.Fatalf("status query must preserve source row and mark it disabled: %s", disableJobLinkQuery)
	}
	if !strings.Contains(strings.ToUpper(deleteJobLinkQuery), "DELETE FROM PUBLIC.JOB_SOURCES") {
		t.Fatalf("delete query must remove the source row: %s", deleteJobLinkQuery)
	}
	if !strings.Contains(strings.ToUpper(updateJobLinkQuery), "APPROVAL_STATUS = 'ACTIVE'") {
		t.Fatalf("update query must re-approve a disabled source after an admin edits it: %s", updateJobLinkQuery)
	}
	listQuery := strings.ToLower(listJobLinksQuery)
	if strings.Contains(listQuery, "raw_html") || strings.Contains(listQuery, "raw_jd") || strings.Contains(listQuery, "description") || strings.Contains(listQuery, "content") {
		t.Fatalf("job-link list query must not expose fetched page content: %s", listJobLinksQuery)
	}
}
