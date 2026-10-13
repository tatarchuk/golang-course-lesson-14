package model

import (
	"fmt"
	"strings"
	"time"
)

// User roles; the same list is enforced by the CHECK constraint on users.role.
const (
	UserRoleMember = "member"
	UserRoleStaff  = "staff"
	UserRoleAdmin  = "admin"
)

// Password length limits: bcrypt reads at most 72 bytes of the password. MaxEmailLength is
// the RFC 5321 limit, so nobody can register a megabyte-long "address".
const (
	MinPasswordLength = 8
	MaxPasswordLength = 72
	MaxEmailLength    = 254
)

// emailProblems checks the shape of a non-blank e-mail: at most MaxEmailLength bytes and
// exactly one "@" with something on both sides. Deliverability is not checked here; a blank
// value is reported by requiredProblem instead.
func emailProblems(email string) []string {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil
	}
	var problems []string
	if len(email) > MaxEmailLength {
		problems = append(problems, fmt.Sprintf("email must be at most %d bytes", MaxEmailLength))
	}
	local, domain, ok := strings.Cut(email, "@")
	if !ok || local == "" || domain == "" || strings.Contains(domain, "@") || strings.ContainsAny(email, " \t\r\n") {
		problems = append(problems, "email must look like name@domain")
	}
	return problems
}

// User is an account of the catalogue. PasswordHash is never serialised: the json:"-" tag
// keeps it out of every response, whatever struct is encoded.
type User struct {
	ID           int64     `json:"id" gorm:"primaryKey"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	PasswordHash string    `json:"-" gorm:"column:password_hash"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName pins the table name.
func (User) TableName() string { return "users" }

// ValidRole reports whether role is one of the three roles the schema allows.
func ValidRole(role string) bool {
	return role == UserRoleMember || role == UserRoleStaff || role == UserRoleAdmin
}

// NormalizeEmail trims and lower-cases an e-mail, so lookups and the UNIQUE constraint
// treat "Reader@Albums.local" and "reader@albums.local" as one account.
func NormalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// RegisterInput is the body of POST /auth/register. It has no role field on purpose:
// decoding a request into it can never touch Role or PasswordHash (no mass assignment).
type RegisterInput struct {
	Email    string `json:"email" validate:"required" example:"reader@albums.local"`
	Password string `json:"password" validate:"required" example:"Correct-Horse-Battery-1"`
}

// Validate requires both fields and a password of 8..72 bytes.
func (in RegisterInput) Validate() error {
	problems := requiredProblem(field{"email", in.Email}, field{"password", in.Password})
	problems = append(problems, emailProblems(in.Email)...)
	if n := len(in.Password); strings.TrimSpace(in.Password) != "" && (n < MinPasswordLength || n > MaxPasswordLength) {
		problems = append(problems, fmt.Sprintf("password must be between %d and %d bytes", MinPasswordLength, MaxPasswordLength))
	}
	return validationError(problems)
}

// LoginInput is the body of POST /auth/login.
type LoginInput struct {
	Email    string `json:"email" validate:"required" example:"reader@albums.local"`
	Password string `json:"password" validate:"required" example:"Correct-Horse-Battery-1"`
}

// Validate requires both fields; the password only has to fit bcrypt's limit.
func (in LoginInput) Validate() error {
	problems := requiredProblem(field{"email", in.Email}, field{"password", in.Password})
	problems = append(problems, emailProblems(in.Email)...)
	if len(in.Password) > MaxPasswordLength {
		problems = append(problems, fmt.Sprintf("password must be at most %d bytes", MaxPasswordLength))
	}
	return validationError(problems)
}

// RoleInput is the body of PUT /users/{id}/role: the only field an administrator can change.
type RoleInput struct {
	Role string `json:"role" validate:"required" example:"staff"`
}

// Validate requires a role from the schema's list.
func (in RoleInput) Validate() error {
	problems := requiredProblem(field{"role", in.Role})
	if strings.TrimSpace(in.Role) != "" && !ValidRole(in.Role) {
		problems = append(problems, "role must be one of: member, staff, admin")
	}
	return validationError(problems)
}
