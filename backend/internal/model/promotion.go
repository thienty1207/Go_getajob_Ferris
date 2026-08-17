package model

import "github.com/google/uuid"

// Promotion is the metadata sent to the client carousel. Image bytes are
// intentionally absent so list responses stay small and the binary endpoint
// can apply its own cache and integrity headers.
type Promotion struct {
	ID          uuid.UUID
	Slot        int16
	ImageURL    string
	AltText     string
	Eyebrow     *string
	Title       *string
	Body        *string
	TargetURL   *string
	ContentHash string
}
