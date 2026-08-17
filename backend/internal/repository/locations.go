package repository

import (
	"context"
	"errors"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
)

var (
	ErrLocationNotFound = errors.New("location not found")
	ErrLocationConflict = errors.New("location already exists")
	ErrJobNotFound      = errors.New("job not found")
)

type LocationWrite struct {
	DisplayName  string
	Province     string
	Country      string
	CanonicalKey string
	IsActive     bool
}

type LocationRepository interface {
	ListLocations(context.Context, int, int) (model.AdminLocationPage, error)
	ListLocationOptions(context.Context) ([]model.AdminLocationOption, error)
	ListActiveLocations(context.Context) ([]model.ClientLocation, error)
	CreateLocation(context.Context, LocationWrite) (model.AdminLocation, error)
	UpdateLocation(context.Context, uuid.UUID, LocationWrite) (model.AdminLocation, error)
	AssignJobLocation(context.Context, uuid.UUID, *uuid.UUID) error
}
