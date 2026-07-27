package util

import "github.com/google/uuid"

// NewString returns a new random UUID as a string.
func NewString() string {
	return uuid.New().String()
}
