package service

import (
	"context"
	"errors"
	"testing"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/google/uuid"
)

type testLocationRepository struct {
	created         repository.LocationWrite
	updated         repository.LocationWrite
	updatedID       uuid.UUID
	assignedJobID   uuid.UUID
	assignedTo      *uuid.UUID
	createdLocation model.AdminLocation
}

func (r *testLocationRepository) ListLocations(context.Context, int, int) (model.AdminLocationPage, error) {
	return model.AdminLocationPage{Items: []model.AdminLocation{r.createdLocation}, Page: 1, PageSize: repository.AdminPageSize, Total: 1}, nil
}

func (r *testLocationRepository) ListLocationOptions(context.Context) ([]model.AdminLocationOption, error) {
	return nil, nil
}

func (r *testLocationRepository) ListActiveLocations(context.Context) ([]model.ClientLocation, error) {
	return nil, nil
}

func (r *testLocationRepository) CreateLocation(_ context.Context, write repository.LocationWrite) (model.AdminLocation, error) {
	r.created = write
	return r.createdLocation, nil
}

func (r *testLocationRepository) UpdateLocation(_ context.Context, id uuid.UUID, write repository.LocationWrite) (model.AdminLocation, error) {
	r.updatedID = id
	r.updated = write
	return r.createdLocation, nil
}

func (r *testLocationRepository) AssignJobLocation(_ context.Context, jobID uuid.UUID, locationID *uuid.UUID) error {
	r.assignedJobID = jobID
	r.assignedTo = locationID
	return nil
}

func TestLocationServiceNormalizesCanonicalFields(t *testing.T) {
	repo := &testLocationRepository{createdLocation: model.AdminLocation{ID: uuid.New()}}
	service := NewLocationService(repo)

	_, err := service.Create(context.Background(), LocationInput{
		DisplayName: "  Thành phố Hồ Chí Minh ",
		Province:    " Hồ Chí Minh ",
		Country:     " Vietnam ",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if repo.created.DisplayName != "Thành phố Hồ Chí Minh" || repo.created.Province != "Hồ Chí Minh" || repo.created.Country != "Vietnam" {
		t.Fatalf("normalized location = %#v", repo.created)
	}
	if repo.created.CanonicalKey != "thanh pho ho chi minh" {
		t.Fatalf("canonical key = %q", repo.created.CanonicalKey)
	}
}

func TestLocationServiceRejectsBlankCanonicalFields(t *testing.T) {
	repo := &testLocationRepository{}
	service := NewLocationService(repo)
	_, err := service.Create(context.Background(), LocationInput{DisplayName: " ", Province: "Hà Nội", Country: "Vietnam", IsActive: true})
	if !errors.Is(err, ErrInvalidLocation) {
		t.Fatalf("Create() error = %v, want ErrInvalidLocation", err)
	}
}

func TestLocationServiceAssignsAndClearsCanonicalLocation(t *testing.T) {
	repo := &testLocationRepository{}
	service := NewLocationService(repo)
	jobID := uuid.New()
	locationID := uuid.New()

	if err := service.AssignJobLocation(context.Background(), jobID, &locationID); err != nil {
		t.Fatalf("AssignJobLocation() error = %v", err)
	}
	if repo.assignedJobID != jobID || repo.assignedTo == nil || *repo.assignedTo != locationID {
		t.Fatalf("assignment = %s/%v", repo.assignedJobID, repo.assignedTo)
	}
	if err := service.AssignJobLocation(context.Background(), jobID, nil); err != nil {
		t.Fatalf("clear location error = %v", err)
	}
	if repo.assignedTo != nil {
		t.Fatalf("location should be cleared, got %v", *repo.assignedTo)
	}
}
