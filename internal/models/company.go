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

	// Interactive Draft fields
	Employees []Employee `json:"employees,omitempty"`
}

// Employee represents a contact/person associated with a company.
type Employee struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Department string    `json:"department,omitempty"`
}
