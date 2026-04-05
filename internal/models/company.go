package models

import (
	"time"

	"github.com/google/uuid"
)

// Company represents a business entity in the CRM.
type Company struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Industry  string     `json:"industry"`
	Website   string     `json:"website"`
	Email     string     `json:"email"`
	Phone     string     `json:"phone"`
	CreatedBy *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
