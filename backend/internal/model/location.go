package model

import (
	"time"

	"github.com/google/uuid"
)

type AdminLocation struct {
	ID           uuid.UUID
	DisplayName  string
	Province     string
	Country      string
	CanonicalKey string
	Latitude     *float64
	Longitude    *float64
	IsActive     bool
	JobCount     int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type AdminLocationPage struct {
	Items    []AdminLocation
	Page     int
	PageSize int
	Total    int
}

type AdminLocationOption struct {
	ID          uuid.UUID
	DisplayName string
	IsActive    bool
}

type ClientLocation struct {
	ID          uuid.UUID
	DisplayName string
	Province    string
	Country     string
	Latitude    *float64
	Longitude   *float64
	IsActive    bool
}
