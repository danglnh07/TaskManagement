package security

import (
	"fmt"
	"time"

	"github.com/danglnh07/TaskManagement/db"
	"github.com/danglnh07/TaskManagement/util"
	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	config *util.Config
}

type TokenType string

const (
	Issuer = "task-management"

	AccessToken  TokenType = "access-token"
	RefreshToken TokenType = "refresh-token"
)

type CustomClaims struct {
	Username             string    `json:"username"`
	Role                 db.Role   `json:"role"`
	TokenType            TokenType `json:"token_type"`
	Version              int       `json:"version"`
	jwt.RegisteredClaims           // Embed the JWT Registered claims
}

func NewJWTService(config *util.Config) *JWTService {
	return &JWTService{
		config: config,
	}
}

func (service *JWTService) CreateToken(username string, role db.Role, tokenType TokenType, version int) (string, error) {
	// Check token type and decide expiration time based on type
	var expiration time.Duration
	switch tokenType {
	case AccessToken:
		expiration = service.config.TokenExpiration
	case RefreshToken:
		expiration = service.config.RefreshTokenExpiration
	default:
		return "", fmt.Errorf("invalid token type")
	}

	// Create custom JWT claim
	claims := CustomClaims{
		Username:  string(username),
		Role:      role,
		TokenType: tokenType,
		Version:   version,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,                                         // Who issue this token
			Subject:   username,                                       // Whom the token is about
			IssuedAt:  jwt.NewNumericDate(time.Now()),                 // When the token is created
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiration)), // When the token is expired
		},
	}

	// Generate token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token
	tokenStr, err := token.SignedString(service.config.SecretKey)
	if err != nil {
		return "", err
	}

	return tokenStr, nil
}

func (service *JWTService) VerifyToken(signedToken string) (*CustomClaims, error) {
	// Use custom parser with deley to 30 secs
	parser := jwt.NewParser(jwt.WithLeeway(30 * time.Second))

	// Parse token
	parsedToken, err := parser.ParseWithClaims(signedToken, &CustomClaims{}, func(token *jwt.Token) (any, error) {
		// Check for signing method to avoid [alg: none] trick
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return service.config.SecretKey, nil
	})

	// Check if token parsing success
	if err != nil {
		return nil, err
	}

	// Extract claims from token
	claims, ok := parsedToken.Claims.(*CustomClaims)
	if !(ok && parsedToken.Valid) {
		return nil, jwt.ErrTokenInvalidClaims
	}

	// Check if this is the correct issuer
	if claims.Issuer != Issuer {
		return nil, fmt.Errorf("invalid issuer")
	}

	// Check if this is the correct role
	if claims.Role != db.Admin && claims.Role != db.User {
		return nil, fmt.Errorf("invalid role")
	}

	// Check if the token type is correct
	if claims.TokenType != AccessToken && claims.TokenType != RefreshToken {
		return nil, fmt.Errorf("invalid token type")
	}

	return claims, nil
}
