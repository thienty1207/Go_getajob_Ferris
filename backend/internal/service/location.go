package service

import (
	"context"
	"errors"
	"strings"

	locationkey "github.com/gogetsomefoodferris/backend/internal/location"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrInvalidLocation   = errors.New("invalid location")
	ErrInvalidLocationID = errors.New("invalid location ID")
)

type LocationInput struct {
	DisplayName string
	Province    string
	Country     string
	IsActive    bool
}

type LocationService struct {
	repository repository.LocationRepository
}

func NewLocationService(locationRepository repository.LocationRepository) *LocationService {
	return &LocationService{repository: locationRepository}
}

func (s *LocationService) List(ctx context.Context, page, pageSize int) (model.AdminLocationPage, error) {
	return s.repository.ListLocations(ctx, page, pageSize)
}

func (s *LocationService) Options(ctx context.Context) ([]model.AdminLocationOption, error) {
	return s.repository.ListLocationOptions(ctx)
}

func (s *LocationService) ListActive(ctx context.Context) ([]model.ClientLocation, error) {
	return s.repository.ListActiveLocations(ctx)
}

func (s *LocationService) Create(ctx context.Context, input LocationInput) (model.AdminLocation, error) {
	write, err := normalizeLocationInput(input)
	if err != nil {
		return model.AdminLocation{}, err
	}
	return s.repository.CreateLocation(ctx, write)
}

func (s *LocationService) Update(ctx context.Context, id uuid.UUID, input LocationInput) (model.AdminLocation, error) {
	if id == uuid.Nil {
		return model.AdminLocation{}, ErrInvalidLocationID
	}
	write, err := normalizeLocationInput(input)
	if err != nil {
		return model.AdminLocation{}, err
	}
	return s.repository.UpdateLocation(ctx, id, write)
}

func (s *LocationService) AssignJobLocation(ctx context.Context, jobID uuid.UUID, locationID *uuid.UUID) error {
	if jobID == uuid.Nil || (locationID != nil && *locationID == uuid.Nil) {
		return ErrInvalidLocationID
	}
	return s.repository.AssignJobLocation(ctx, jobID, locationID)
}

func normalizeLocationInput(input LocationInput) (repository.LocationWrite, error) {
	displayName := strings.TrimSpace(input.DisplayName)
	province := strings.TrimSpace(input.Province)
	country := strings.TrimSpace(input.Country)
	if displayName == "" || province == "" || country == "" || len(displayName) > 200 || len(province) > 160 || len(country) > 120 {
		return repository.LocationWrite{}, ErrInvalidLocation
	}
	canonicalKey := locationkey.Normalize(displayName)
	if canonicalKey == "" {
		return repository.LocationWrite{}, ErrInvalidLocation
	}
	return repository.LocationWrite{DisplayName: displayName, Province: province, Country: country, CanonicalKey: canonicalKey, IsActive: input.IsActive}, nil
}

var _ repository.LocationRepository = (*repository.PostgresLocationRepository)(nil)
