package model

import (
	"time"

	"github.com/google/uuid"
)

// ScanStatus is the persisted scan lifecycle. The repository and database
// both guard legal transitions so a client cannot skip directly to results.
type ScanStatus string

const (
	StatusReceived  ScanStatus = "RECEIVED"
	StatusParsing   ScanStatus = "PARSING"
	StatusMatching  ScanStatus = "MATCHING"
	StatusCompleted ScanStatus = "COMPLETED"
	StatusFailed    ScanStatus = "FAILED"
)

// IsValid reports whether a status belongs to the supported lifecycle.
func (status ScanStatus) IsValid() bool {
	switch status {
	case StatusReceived, StatusParsing, StatusMatching, StatusCompleted, StatusFailed:
		return true
	default:
		return false
	}
}

// IsProcessing identifies states the client should continue polling.
func (status ScanStatus) IsProcessing() bool {
	return status == StatusReceived || status == StatusParsing || status == StatusMatching
}

// Scan is the structured scan state returned by the service. It contains no
// raw CV path or raw CV contents.
type Scan struct {
	ID         uuid.UUID
	Status     ScanStatus
	ErrorCode  string
	LocationID *uuid.UUID
	Location   string
	Latitude   *float64
	Longitude  *float64
	RadiusKm   float64
	Matches    []JobMatch
}

// JobSalary preserves the source display/currency without converting values.
type JobSalary struct {
	Display  string
	Currency string
}

// JobMatch is the small public job-card projection returned after matching.
type JobMatch struct {
	ID             uuid.UUID
	MatchPercent   float64
	Title          string
	Company        string
	Location       string
	DistanceKm     *float64
	EmploymentType string
	WorkMode       string
	Salary         *JobSalary
	SkillTags      []string
	OriginalURL    string
}

type EducationRecord struct {
	Institution  string `json:"institution,omitempty"`
	Degree       string `json:"degree,omitempty"`
	FieldOfStudy string `json:"field_of_study,omitempty"`
	StartYear    *int   `json:"start_year,omitempty"`
	EndYear      *int   `json:"end_year,omitempty"`
	Grade        string `json:"grade,omitempty"`
}

type CertificationRecord struct {
	CertificateName string `json:"certificate_name,omitempty"`
	Issuer          string `json:"issuer,omitempty"`
	IssuedYear      *int   `json:"issued_year,omitempty"`
	ExpiresYear     *int   `json:"expires_year,omitempty"`
}

type StructuredProfile struct {
	Roles             []string              `json:"roles"`
	Skills            []string              `json:"skills"`
	YearsOfExperience float64               `json:"years_of_experience"`
	Seniority         string                `json:"seniority"`
	Domains           []string              `json:"domains"`
	Education         []EducationRecord     `json:"education"`
	Certifications    []CertificationRecord `json:"certifications"`
}

// ClientCVHistoryItem is the durable, owner-scoped view of one submitted CV.
// It deliberately contains structured profile fields only; the uploaded file
// and raw extracted text never enter this response model.
type ClientCVHistoryItem struct {
	ScanID     uuid.UUID
	Status     ScanStatus
	Location   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Profile    *StructuredProfile
	MatchCount int
}

type JobCandidate struct {
	ID                uuid.UUID
	Title             string
	Company           string
	Location          string
	Role              string
	RequiredSkills    []string
	PreferredSkills   []string
	Seniority         string
	MinimumExperience *float64
	Domains           []string
	EmploymentType    string
	WorkMode          string
	SalaryDisplay     *string
	SalaryCurrency    *string
	OriginalURL       string
	DistanceKm        *float64
}

type ScoredJobMatch struct {
	JobID                       uuid.UUID
	RequiredSkillsPoints        float64
	RoleRelevancePoints         float64
	ExperiencePoints            float64
	SeniorityPoints             float64
	PreferredSkillsDomainPoints float64
	MatchPercent                float64
	DistanceKm                  *float64
}
