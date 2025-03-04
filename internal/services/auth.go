package services

import (
	"context"
	"errors"
	"time"

	"chess-ws-go/internal/auth"
	"chess-ws-go/internal/config"
	"chess-ws-go/internal/models"
	"chess-ws-go/internal/repositories"

	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserExists         = errors.New("user already exists")
	ErrUserNotVerified    = errors.New("user not verified")
)

// AuthService handles authentication operations
type AuthService struct {
	userRepo  repositories.UserRepository
	jwtMaker  *auth.JWTMaker
	jwtConfig *config.JWTConfig
}

// NewAuthService creates a new authentication service
func NewAuthService(
	userRepo repositories.UserRepository,
	jwtConfig *config.JWTConfig,
) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		jwtMaker:  auth.NewJWTMaker(jwtConfig.SecretKey),
		jwtConfig: jwtConfig,
	}
}

// RegisterUser registers a new user
func (s *AuthService) RegisterUser(
	ctx context.Context,
	username string,
	email string,
	password string,
) (*models.User, error) {
	// Check if user already exists
	_, err := s.userRepo.GetByUsername(ctx, username)
	if err == nil {
		return nil, ErrUserExists
	} else if err != repositories.ErrUserNotFound {
		return nil, err
	}

	// Check if email already exists
	_, err = s.userRepo.GetByEmail(ctx, email)
	if err == nil {
		return nil, ErrUserExists
	} else if err != repositories.ErrUserNotFound {
		return nil, err
	}

	// Hash password
	passwordHash, err := auth.HashPassword(password, nil)
	if err != nil {
		return nil, err
	}

	// Create user
	user := models.NewUser(
		uuid.New().String(),
		username,
		email,
		passwordHash,
	)

	// Generate verification token
	user.VerificationToken = uuid.New().String()

	// Save user
	err = s.userRepo.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	// TODO: Send verification email

	return user, nil
}

// Login authenticates a user and returns tokens
func (s *AuthService) Login(
	ctx context.Context,
	usernameOrEmail string,
	password string,
) (*auth.TokenPair, error) {
	// Find user by username or email
	var user *models.User
	var err error

	// Try username first
	user, err = s.userRepo.GetByUsername(ctx, usernameOrEmail)
	if err != nil {
		if err == repositories.ErrUserNotFound {
			// Try email
			user, err = s.userRepo.GetByEmail(ctx, usernameOrEmail)
			if err != nil {
				return nil, ErrInvalidCredentials
			}
		} else {
			return nil, err
		}
	}

	// Check if user is verified
	if !user.IsVerified {
		return nil, ErrUserNotVerified
	}

	// Verify password
	match, err := auth.VerifyPassword(password, user.PasswordHash)
	if err != nil {
		return nil, err
	}

	if !match {
		// Increment failed login attempts
		user.FailedLoginAttempts++
		_ = s.userRepo.Update(ctx, user)
		return nil, ErrInvalidCredentials
	}

	// Reset failed login attempts and update last login time
	now := time.Now()
	user.FailedLoginAttempts = 0
	user.LastLoginAt = &now
	_ = s.userRepo.Update(ctx, user)

	// Get user permissions
	permissions, err := s.userRepo.GetPermissions(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// Convert string permissions to auth.Permission
	authPermissions := make([]auth.Permission, len(permissions))
	for i, p := range permissions {
		authPermissions[i] = auth.Permission(p)
	}

	// Create access token
	accessToken, err := s.jwtMaker.CreateToken(
		user.ID,
		user.Username,
		user.Role,
		authPermissions,
		time.Duration(s.jwtConfig.AccessTokenDuration)*time.Minute,
	)
	if err != nil {
		return nil, err
	}

	// Create refresh token
	refreshToken, err := s.jwtMaker.CreateToken(
		user.ID,
		user.Username,
		user.Role,
		nil, // No permissions in refresh token
		time.Duration(s.jwtConfig.RefreshTokenDuration)*time.Hour,
	)
	if err != nil {
		return nil, err
	}

	// Save refresh token to database
	refreshTokenModel := &models.RefreshToken{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(time.Duration(s.jwtConfig.RefreshTokenDuration) * time.Hour),
		CreatedAt: time.Now(),
	}

	err = s.userRepo.SaveRefreshToken(ctx, refreshTokenModel)
	if err != nil {
		return nil, err
	}

	return &auth.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// RefreshToken refreshes an access token using a refresh token
func (s *AuthService) RefreshToken(
	ctx context.Context,
	refreshToken string,
) (*auth.TokenPair, error) {
	// Verify refresh token
	claims, err := s.jwtMaker.VerifyToken(refreshToken)
	if err != nil {
		return nil, err
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	// Get user permissions
	permissions, err := s.userRepo.GetPermissions(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// Convert string permissions to auth.Permission
	authPermissions := make([]auth.Permission, len(permissions))
	for i, p := range permissions {
		authPermissions[i] = auth.Permission(p)
	}

	// Create new access token
	newAccessToken, err := s.jwtMaker.CreateToken(
		user.ID,
		user.Username,
		user.Role,
		authPermissions,
		time.Duration(s.jwtConfig.AccessTokenDuration)*time.Minute,
	)
	if err != nil {
		return nil, err
	}

	// Create new refresh token
	newRefreshToken, err := s.jwtMaker.CreateToken(
		user.ID,
		user.Username,
		user.Role,
		nil, // No permissions in refresh token
		time.Duration(s.jwtConfig.RefreshTokenDuration)*time.Hour,
	)
	if err != nil {
		return nil, err
	}

	// Save new refresh token to database
	refreshTokenModel := &models.RefreshToken{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Token:     newRefreshToken,
		ExpiresAt: time.Now().Add(time.Duration(s.jwtConfig.RefreshTokenDuration) * time.Hour),
		CreatedAt: time.Now(),
	}

	err = s.userRepo.SaveRefreshToken(ctx, refreshTokenModel)
	if err != nil {
		return nil, err
	}

	return &auth.TokenPair{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

// VerifyEmail verifies a user's email
func (s *AuthService) VerifyEmail(
	ctx context.Context,
	token string,
) error {
	// TODO: Implement a proper method to find user by verification token
	// For now, this is a placeholder implementation

	// In a real implementation, we would add a method to the repository:
	// user, err := s.userRepo.GetByVerificationToken(ctx, token)

	// For now, we'll return an error indicating this needs to be implemented
	return errors.New("verification token lookup not implemented")

	// Once implemented, the code would look like:
	/*
		user, err := s.userRepo.GetByVerificationToken(ctx, token)
		if err != nil {
			return err
		}

		// Mark user as verified
		user.IsVerified = true
		user.VerificationToken = ""

		// Update user
		return s.userRepo.Update(ctx, user)
	*/
}
