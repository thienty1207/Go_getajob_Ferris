package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/google/uuid"
)

type testJobLinkRepository struct {
	created  repository.JobLinkWrite
	updated  repository.JobLinkWrite
	page     model.JobLinkPage
	item     model.JobLink
	disabled uuid.UUID
	statusID uuid.UUID
	status   string
	deleted  uuid.UUID
}

func (r *testJobLinkRepository) ListJobLinks(context.Context, int, int) (model.JobLinkPage, error) {
	return r.page, nil
}

func (r *testJobLinkRepository) CreateJobLink(_ context.Context, write repository.JobLinkWrite) (model.JobLink, error) {
	r.created = write
	return r.item, nil
}

func (r *testJobLinkRepository) UpdateJobLink(_ context.Context, write repository.JobLinkWrite) (model.JobLink, error) {
	r.updated = write
	return r.item, nil
}

func (r *testJobLinkRepository) DisableJobLink(_ context.Context, id uuid.UUID) error {
	r.disabled = id
	return nil
}

func (r *testJobLinkRepository) SetJobLinkStatus(_ context.Context, id uuid.UUID, status string) error {
	r.statusID = id
	r.status = status
	return nil
}

func (r *testJobLinkRepository) DeleteJobLink(_ context.Context, id uuid.UUID) error {
	r.deleted = id
	return nil
}

func TestJobLinkServiceNormalizesApprovedURLAndDerivesMetadata(t *testing.T) {
	repository := &testJobLinkRepository{item: model.JobLink{ID: uuid.New()}}
	service := NewJobLinkService(repository)

	result, err := service.Create(context.Background(), JobLinkInput{
		URL:        " HTTPS://Jobs.Example.com/careers#fragment ",
		ApprovedBy: "admin@example.com",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if repository.created.BaseURL != "https://jobs.example.com/careers/" {
		t.Fatalf("normalized URL = %q", repository.created.BaseURL)
	}
	if repository.created.DisplayName != "jobs.example.com" {
		t.Fatalf("display name = %q", repository.created.DisplayName)
	}
	if repository.created.ApprovalStatus != "ACTIVE" || repository.created.SourceType != "EXPLICIT_PERMISSION" {
		t.Fatalf("approval = %q source type = %q", repository.created.ApprovalStatus, repository.created.SourceType)
	}
	if repository.created.ApprovedBy == nil || *repository.created.ApprovedBy != "admin@example.com" || repository.created.ApprovedAt == nil || repository.created.ApprovedAt.IsZero() {
		t.Fatalf("approval evidence = %#v", repository.created)
	}
	if repository.created.ID == uuid.Nil || repository.created.SourceKey == "" {
		t.Fatalf("generated identity = %#v", repository.created)
	}
	if result.ID == uuid.Nil {
		t.Fatalf("service returned empty row = %#v", result)
	}
}

func TestJobLinkServiceCanonicalizesDefaultPorts(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{name: "https default", raw: "https://jobs.example.com:443/careers", want: "https://jobs.example.com/careers/"},
		{name: "http default", raw: "http://jobs.example.com:80/careers", want: "http://jobs.example.com/careers/"},
		{name: "https leading-zero default", raw: "https://jobs.example.com:000443/careers", want: "https://jobs.example.com/careers/"},
		{name: "http leading-zero default", raw: "http://jobs.example.com:00080/careers", want: "http://jobs.example.com/careers/"},
		{name: "https leading-zero custom", raw: "https://jobs.example.com:0008443/careers", want: "https://jobs.example.com:8443/careers/"},
		{name: "https custom", raw: "https://jobs.example.com:8443/careers", want: "https://jobs.example.com:8443/careers/"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repository := &testJobLinkRepository{item: model.JobLink{ID: uuid.New()}}
			service := NewJobLinkService(repository)
			if _, err := service.Create(context.Background(), JobLinkInput{URL: testCase.raw, ApprovedBy: "admin@example.com"}); err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if repository.created.BaseURL != testCase.want {
				t.Fatalf("normalized URL = %q, want %q", repository.created.BaseURL, testCase.want)
			}
		})
	}
}

func TestJobLinkServiceRejectsUnsafeURL(t *testing.T) {
	cases := []string{
		"javascript:alert(1)",
		"ftp://jobs.example.com/careers",
		"https://user:password@jobs.example.com/careers",
		"https:///missing-host",
		"https://jobs.example.com/careers\nhttps://other.example.com",
		"https://jobs.example.com/careers/%2e%2e/admin",
		"https://jobs.example.com/careers/%2fadmin",
		"https://jobs.example.com:0/careers",
		"https://jobs.example.com:65536/careers",
		"https://100.64.0.1/careers",
		"https://192.0.2.1/careers",
		"https://[2001:db8::1]/careers",
		"https://[3000::1]/careers",
		"https://[ff00::1]/careers",
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			repository := &testJobLinkRepository{}
			service := NewJobLinkService(repository)
			_, err := service.Create(context.Background(), JobLinkInput{URL: raw, ApprovedBy: "admin@example.com"})
			if !errors.Is(err, ErrInvalidJobLinkURL) {
				t.Fatalf("Create(%q) error = %v, want ErrInvalidJobLinkURL", raw, err)
			}
			if repository.created.ID != uuid.Nil {
				t.Fatal("unsafe URL reached repository")
			}
		})
	}
}

func TestJobLinkServiceKeepsQueryEscapesAndRejectsOnlyAmbiguousPathEscapes(t *testing.T) {
	repository := &testJobLinkRepository{item: model.JobLink{ID: uuid.New()}}
	service := NewJobLinkService(repository)
	_, err := service.Create(context.Background(), JobLinkInput{
		URL:        "https://jobs.example.com/careers?filter=%2e%2e",
		ApprovedBy: "admin@example.com",
	})
	if err != nil {
		t.Fatalf("query escape should be retained: %v", err)
	}
	if repository.created.BaseURL != "https://jobs.example.com/careers/?filter=%2e%2e" {
		t.Fatalf("normalized query URL = %q", repository.created.BaseURL)
	}
}

func TestJobLinkServiceCanonicalizesIPv6Host(t *testing.T) {
	repository := &testJobLinkRepository{item: model.JobLink{ID: uuid.New()}}
	service := NewJobLinkService(repository)
	_, err := service.Create(context.Background(), JobLinkInput{
		URL:        "https://[2606:4700:4700::1111]:443/careers",
		ApprovedBy: "admin@example.com",
	})
	if err != nil {
		t.Fatalf("IPv6 URL error = %v", err)
	}
	if repository.created.BaseURL != "https://[2606:4700:4700::1111]/careers/" {
		t.Fatalf("normalized IPv6 URL = %q", repository.created.BaseURL)
	}
}

func TestJobLinkServiceCanonicalizesTerminalDotAndExpandedIPv6(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "terminal dot",
			raw:  "https://jobs.example.com./careers",
			want: "https://jobs.example.com/careers/",
		},
		{
			name: "expanded IPv6",
			raw:  "https://[2606:4700:4700:0:0:0:0:1111]/careers",
			want: "https://[2606:4700:4700::1111]/careers/",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repository := &testJobLinkRepository{item: model.JobLink{ID: uuid.New()}}
			service := NewJobLinkService(repository)
			if _, err := service.Create(context.Background(), JobLinkInput{URL: testCase.raw, ApprovedBy: "admin@example.com"}); err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if repository.created.BaseURL != testCase.want {
				t.Fatalf("normalized URL = %q, want %q", repository.created.BaseURL, testCase.want)
			}
		})
	}
}

func TestJobLinkServiceKeepsSourceIdentityWhenURLChanges(t *testing.T) {
	id := uuid.New()
	repository := &testJobLinkRepository{item: model.JobLink{ID: id}}
	service := NewJobLinkService(repository)

	_, err := service.Update(context.Background(), id, JobLinkInput{
		URL:        "https://new.example.com/jobs",
		ApprovedBy: "owner@example.com",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if repository.updated.ID != id {
		t.Fatalf("updated ID = %s, want %s", repository.updated.ID, id)
	}
	if repository.updated.BaseURL != "https://new.example.com/jobs/" {
		t.Fatalf("updated URL = %q", repository.updated.BaseURL)
	}
	if repository.updated.SourceKey != "" {
		t.Fatalf("update must not replace source key: %q", repository.updated.SourceKey)
	}
	if repository.updated.ApprovedBy == nil || *repository.updated.ApprovedBy != "owner@example.com" || repository.updated.ApprovedAt == nil || repository.updated.ApprovedAt.Before(time.Now().Add(-time.Minute)) {
		t.Fatalf("updated approval evidence = %#v", repository.updated)
	}
}

func TestJobLinkServiceSetsExplicitStatus(t *testing.T) {
	id := uuid.New()
	repository := &testJobLinkRepository{}
	service := NewJobLinkService(repository)

	if err := service.SetStatus(context.Background(), id, "DISABLED"); err != nil {
		t.Fatalf("SetStatus() error = %v", err)
	}
	if repository.statusID != id || repository.status != "DISABLED" {
		t.Fatalf("status update = %s/%q, want %s/DISABLED", repository.statusID, repository.status, id)
	}
}

func TestJobLinkServiceHardDeleteDelegatesDelete(t *testing.T) {
	id := uuid.New()
	repository := &testJobLinkRepository{}
	service := NewJobLinkService(repository)

	if err := service.Delete(context.Background(), id); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if repository.deleted != id {
		t.Fatalf("deleted ID = %s, want %s", repository.deleted, id)
	}
}
