package services

import (
	"context"
	"errors"

	"chess-ws-go/internal/models"
	"chess-ws-go/internal/repositories"

	"github.com/google/uuid"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

// UserService handles user-related operations
type UserService struct {
	userRepo repositories.UserRepository
}

// NewUserService creates a new user service
func NewUserService(userRepo repositories.UserRepository) *UserService {
	return &UserService{
		userRepo: userRepo,
	}
}

// UpdateProfile updates a user's profile information
func (s *UserService) UpdateProfile(
	ctx context.Context,
	userID string,
	displayName *string,
	email *string,
) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if err == repositories.ErrUserNotFound {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if displayName != nil {
		user.DisplayName = *displayName
	}
	if email != nil {
		// Check if email is already taken
		existingUser, err := s.userRepo.GetByEmail(ctx, *email)
		if err == nil && existingUser.ID != userID {
			return nil, errors.New("email already taken")
		}
		user.Email = *email
		// Optionally require email verification again
		user.IsVerified = false
		user.VerificationToken = uuid.New().String()
		// TODO: Send verification email
	}

	err = s.userRepo.Update(ctx, user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// DeleteUser deletes a user account
func (s *UserService) DeleteUser(ctx context.Context, userID string) error {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if err == repositories.ErrUserNotFound {
			return ErrUserNotFound
		}
		return err
	}

	// Delete user's refresh tokens
	err = s.userRepo.DeleteUserRefreshTokens(ctx, userID)
	if err != nil {
		return err
	}

	// Delete user
	return s.userRepo.Delete(ctx, userID)
}

// ListUsers returns a paginated list of users with optional search
func (s *UserService) ListUsers(
	ctx context.Context,
	page int,
	limit int,
	search string,
) ([]*models.User, int, error) {
	// TODO: Implement repository method for paginated user listing with search
	return nil, 0, errors.New("not implemented")
}

// RequestPasswordReset initiates the password reset process
func (s *UserService) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if err == repositories.ErrUserNotFound {
			return nil // Return success to prevent email enumeration
		}
		return err
	}

	// Generate reset token
	resetToken := uuid.New().String()
	user.PasswordResetToken = resetToken
	// TODO: Set expiration time for reset token
	// TODO: Send password reset email

	err = s.userRepo.Update(ctx, user)
	if err != nil {
		return err
	}

	return nil
}

// ConfirmPasswordReset completes the password reset process
func (s *UserService) ConfirmPasswordReset(
	ctx context.Context,
	token string,
	newPassword string,
) error {
	// TODO: Implement repository method to find user by reset token
	return errors.New("not implemented")
}
