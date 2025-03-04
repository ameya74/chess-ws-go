package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"chess-ws-go/internal/models"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
)

// UserRepository defines the interface for user data access
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id string) error

	// Permission methods
	AddPermission(ctx context.Context, userID string, permission string) error
	RemovePermission(ctx context.Context, userID string, permission string) error
	GetPermissions(ctx context.Context, userID string) ([]string, error)

	// Token methods
	SaveRefreshToken(ctx context.Context, token *models.RefreshToken) error
	GetRefreshToken(ctx context.Context, tokenID string) (*models.RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, tokenID string) error
	DeleteUserRefreshTokens(ctx context.Context, userID string) error
}

// SQLUserRepository implements UserRepository using SQL database
type SQLUserRepository struct {
	db *sqlx.DB
}

// NewSQLUserRepository creates a new SQL-based user repository
func NewSQLUserRepository(db *sqlx.DB) UserRepository {
	return &SQLUserRepository{db: db}
}

// Create adds a new user to the database
func (r *SQLUserRepository) Create(ctx context.Context, user *models.User) error {
	// Generate UUID if not provided
	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	// Set timestamps
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	query := `
		INSERT INTO users (
			id, username, email, password_hash, role, display_name, 
			is_verified, verification_token, elo_rating, 
			failed_login_attempts, created_at, updated_at
		) VALUES (
			:id, :username, :email, :password_hash, :role, :display_name, 
			:is_verified, :verification_token, :elo_rating, 
			:failed_login_attempts, :created_at, :updated_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, user)
	return err
}

// GetByID retrieves a user by ID
func (r *SQLUserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	var user models.User

	query := `
		SELECT * FROM users 
		WHERE id = $1
	`

	err := r.db.GetContext(ctx, &user, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// GetByUsername retrieves a user by username
func (r *SQLUserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User

	query := `
		SELECT * FROM users 
		WHERE username = $1
	`

	err := r.db.GetContext(ctx, &user, query, username)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// GetByEmail retrieves a user by email
func (r *SQLUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User

	query := `
		SELECT * FROM users 
		WHERE email = $1
	`

	err := r.db.GetContext(ctx, &user, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// Update updates an existing user
func (r *SQLUserRepository) Update(ctx context.Context, user *models.User) error {
	user.UpdatedAt = time.Now()

	query := `
		UPDATE users SET
			username = :username,
			email = :email,
			password_hash = :password_hash,
			role = :role,
			display_name = :display_name,
			is_verified = :is_verified,
			verification_token = :verification_token,
			elo_rating = :elo_rating,
			failed_login_attempts = :failed_login_attempts,
			last_login_at = :last_login_at,
			updated_at = :updated_at
		WHERE id = :id
	`

	result, err := r.db.NamedExecContext(ctx, query, user)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

// Delete removes a user by ID
func (r *SQLUserRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

// AddPermission adds a permission to a user
func (r *SQLUserRepository) AddPermission(ctx context.Context, userID string, permission string) error {
	query := `
		INSERT INTO user_permissions (user_id, permission)
		VALUES ($1, $2)
		ON CONFLICT (user_id, permission) DO NOTHING
	`

	_, err := r.db.ExecContext(ctx, query, userID, permission)
	return err
}

// RemovePermission removes a permission from a user
func (r *SQLUserRepository) RemovePermission(ctx context.Context, userID string, permission string) error {
	query := `
		DELETE FROM user_permissions
		WHERE user_id = $1 AND permission = $2
	`

	_, err := r.db.ExecContext(ctx, query, userID, permission)
	return err
}

// GetPermissions retrieves all permissions for a user
func (r *SQLUserRepository) GetPermissions(ctx context.Context, userID string) ([]string, error) {
	var permissions []string

	query := `
		SELECT permission FROM user_permissions
		WHERE user_id = $1
	`

	err := r.db.SelectContext(ctx, &permissions, query, userID)
	if err != nil {
		return nil, err
	}

	return permissions, nil
}

// SaveRefreshToken saves a refresh token to the database
func (r *SQLUserRepository) SaveRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	// Generate UUID if not provided
	if token.ID == "" {
		token.ID = uuid.New().String()
	}

	// Set created timestamp if not provided
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO refresh_tokens (id, user_id, token, expires_at, created_at)
		VALUES (:id, :user_id, :token, :expires_at, :created_at)
	`

	_, err := r.db.NamedExecContext(ctx, query, token)
	return err
}

// GetRefreshToken retrieves a refresh token by ID
func (r *SQLUserRepository) GetRefreshToken(ctx context.Context, tokenID string) (*models.RefreshToken, error) {
	var token models.RefreshToken

	query := `
		SELECT * FROM refresh_tokens
		WHERE id = $1
	`

	err := r.db.GetContext(ctx, &token, query, tokenID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("refresh token not found")
		}
		return nil, err
	}

	return &token, nil
}

// DeleteRefreshToken deletes a refresh token by ID
func (r *SQLUserRepository) DeleteRefreshToken(ctx context.Context, tokenID string) error {
	query := `DELETE FROM refresh_tokens WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, tokenID)
	return err
}

// DeleteUserRefreshTokens deletes all refresh tokens for a user
func (r *SQLUserRepository) DeleteUserRefreshTokens(ctx context.Context, userID string) error {
	query := `DELETE FROM refresh_tokens WHERE user_id = $1`

	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}
