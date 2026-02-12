package models

import "time"

// Tag represents a label that can be applied to issues.
type Tag struct {
	ID        string
	Name      string
	CreatedAt time.Time
}
