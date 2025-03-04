package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
)

type Role string

const (
	RoleAdmin     Role = "ADMIN"
	RoleModerator Role = "MODERATOR"
	RolePlayer    Role = "PLAYER"
	RoleSpectator Role = "SPECTATOR"
)

type Permission string

const (
	PermissionCreateGame   Permission = "CREATE_GAME"
	PermissionJoinGame     Permission = "JOIN_GAME"
	PermissionWatchGame    Permission = "WATCH_GAME"
	PermissionModerateChat Permission = "MODERATE_CHAT"
	PermissionManageUsers  Permission = "MANAGE_USERS"
)

// Claims represents the claims in the JWT token
type Claims struct {
	jwt.RegisteredClaims
	UserID      string       `json:"user_id"`
	Username    string       `json:"username"`
	Role        Role         `json:"role"`
	Permissions []Permission `json:"permissions"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type JWTMaker struct {
	secretKey string
}

func NewJWTMaker(secretKey string) *JWTMaker {
	return &JWTMaker{secretKey: secretKey}
}

// CreateToken creates a new token for a specific username and duration
func (maker *JWTMaker) CreateToken(
	userID string,
	username string,
	role Role,
	permissions []Permission,
	duration time.Duration,
) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
		UserID:      userID,
		Username:    username,
		Role:        role,
		Permissions: permissions,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(maker.secretKey))
}

// VerifyToken checks if the token is valid
func (maker *JWTMaker) VerifyToken(tokenString string) (*Claims, error) {
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		_, ok := token.Method.(*jwt.SigningMethodHMAC)
		if !ok {
			return nil, ErrInvalidToken
		}
		return []byte(maker.secretKey), nil
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, keyFunc)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// HasPermission checks if the claims include a specific permission
func (c *Claims) HasPermission(permission Permission) bool {
	for _, p := range c.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// IsAdmin checks if the user has admin role
func (c *Claims) IsAdmin() bool {
	return c.Role == RoleAdmin
}
