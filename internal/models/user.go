package models

import (
	"chess-ws-go/internal/auth"
	"time"
)

// User represents a user in the chess application
type User struct {
	ID           string    `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Role         auth.Role `json:"role" db:"role"`
	DisplayName  string    `json:"display_name" db:"display_name"`

	// Account status
	IsVerified         bool   `json:"is_verified" db:"is_verified"`
	VerificationToken  string `json:"-" db:"verification_token"`
	PasswordResetToken string `json:"-" db:"password_reset_token"`

	// Chess stats
	EloRating int `json:"elo_rating" db:"elo_rating"`

	// Security
	FailedLoginAttempts int        `json:"-" db:"failed_login_attempts"`
	LastLoginAt         *time.Time `json:"last_login_at" db:"last_login_at"`

	// Timestamps
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// UserPermission represents a permission assigned to a user
type UserPermission struct {
	UserID     string          `db:"user_id"`
	Permission auth.Permission `db:"permission"`
}

// RefreshToken represents a JWT refresh token
type RefreshToken struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	Token     string    `db:"token"`
	ExpiresAt time.Time `db:"expires_at"`
	CreatedAt time.Time `db:"created_at"`
}

// NewUser creates a new user with default values
func NewUser(id, username, email, passwordHash string) *User {
	now := time.Now()
	return &User{
		ID:                  id,
		Username:            username,
		Email:               email,
		PasswordHash:        passwordHash,
		Role:                auth.RolePlayer, // Default role
		DisplayName:         username,        // Default to username
		IsVerified:          false,           // Requires verification
		EloRating:           1200,            // Default ELO rating
		FailedLoginAttempts: 0,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}
