package model

import "time"

// User roles; the same list is enforced by the CHECK constraint on users.role.
const (
	UserRoleMember = "member"
	UserRoleStaff  = "staff"
	UserRoleAdmin  = "admin"
)

// User is an account of the catalogue. It is part of the schema only: no endpoint
// exposes users yet.
type User struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName pins the table name.
func (User) TableName() string { return "users" }
