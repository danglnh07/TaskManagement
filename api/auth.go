package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/danglnh07/TaskManagement/db"
	"github.com/danglnh07/TaskManagement/service/security"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

type UserData struct {
	ID       string
	Username string
	Email    string
}

type OAuthProvider interface {
	Name() string
	ExchangeToken(code string) (*TokenResponse, error)
	FetchUser(token string) (*UserData, error)
}

func (server *Server) HandleAuth(ctx *gin.Context) {
	// Get which OAuth provider
	provider := db.OAuthProvider(ctx.Query("provider"))

	// Based on which provider, construct the URL corresponding
	switch provider {
	case db.Google:
		// Construct the URL
		url := &url.URL{
			Scheme: "https",
			Host:   "accounts.google.com",
			Path:   "/o/oauth2/v2/auth",
		}

		// Add query parameters
		query := url.Query()
		query.Set("client_id", server.config.GoogleClientID)
		query.Set("redirect_uri", fmt.Sprintf("%s/oauth2/callback", server.config.BaseURL))
		query.Set("response_type", "code")
		query.Set("scope", "openid email profile")
		query.Set("state", string(db.Google))
		url.RawQuery = query.Encode()

		// Redirect the client to the real Google OAuth
		ctx.Redirect(http.StatusFound, url.String())
	case db.GitHub:
		// Construct the URL
		url := &url.URL{
			Scheme: "https",
			Host:   "github.com",
			Path:   "/login/oauth/authorize",
		}

		// Add query parameters
		query := url.Query()
		query.Set("client_id", server.config.GithubClientID)
		query.Set("redirect_uri", fmt.Sprintf("%s/oauth2/callback", server.config.BaseURL))
		query.Set("scope", "read:user+user:email")
		query.Set("state", string(db.GitHub))
		url.RawQuery = query.Encode()

		// Redirect the client to the real GitHub OAuth
		ctx.Redirect(http.StatusFound, url.String())
	default:
		ctx.JSON(http.StatusBadRequest, ErrorResponse{fmt.Sprintf("Unsupported OAuth2 provider: %s", provider)})
	}
}

type AuthResponse struct {
	ID           uint   `json:"id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (server *Server) HandleCallback(ctx *gin.Context) {
	// Get the state
	state := db.OAuthProvider(ctx.Query("state"))
	var provider OAuthProvider
	switch state {
	case db.Google:
		provider = &GoogleProvider{
			ClientID:     server.config.GoogleClientID,
			ClientSecret: server.config.GoogleClientSecret,
			BaseURL:      server.config.BaseURL,
		}
	case db.GitHub:
		provider = &GitHubProvider{
			ClientID:     server.config.GithubClientID,
			ClientSecret: server.config.GithubClientSecret,
		}
	default:
		ctx.JSON(http.StatusBadRequest, ErrorResponse{"Invalid state"})
		return
	}

	// Get the code return by OAuth provider
	code := ctx.Query("code")
	if code = strings.TrimSpace(code); code == "" {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{"Invalid code"})
		return
	}

	// Exchange code token
	tokenResp, err := provider.ExchangeToken(code)
	if err != nil {
		server.logger.Error("GET /oauth2/callback: failed to exchange code for token", "error", err)
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{"Internal server error"})
		return
	}

	// Fetch user data using token
	data, err := provider.FetchUser(tokenResp.AccessToken)
	if err != nil {
		server.logger.Error("GET /oauth2/callback: failed to fetch user data from OAuth provider", "error", err)
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{"Internal server error"})
		return
	}

	// Check if user is not registered
	// Here, we don't use username as the condition, since username in OAuth provider can be the same
	// or the client just login into using different social account
	var acc db.Account
	result := server.queries.DB.
		Where("oauth_provider = ? AND oauth_provider_id = ?", provider.Name(), data.ID).
		First(&acc)

	// Here, if no record found, then we register new account into the system
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		acc.Username = data.Username
		acc.Email = data.Email
		acc.Role = db.User
		acc.OauthProvider = db.OAuthProvider(provider.Name())
		acc.OauthProviderID = data.ID
		acc.TokenVersion = 1

		result := server.queries.DB.Create(&acc)

		if result.Error != nil {
			server.logger.Error("GET /oauth2/callback: failed to create account", "error", err)
			ctx.JSON(http.StatusInternalServerError, ErrorResponse{"Internal server error"})
			return
		}
	}

	// Create access token and refresh token
	accessToken, err := server.jwtService.CreateToken(acc.Username, acc.Role, security.AccessToken, acc.TokenVersion)
	if err != nil {
		server.logger.Error("GET /oauth2/callback: failed to create access token", "error", err)
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{"Internal server error"})
		return
	}

	refreshToken, err := server.jwtService.CreateToken(acc.Username, acc.Role, security.RefreshToken, acc.TokenVersion)
	if err != nil {
		server.logger.Error("GET /oauth2/callback: failed to create refresh token", "error", err)
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{"Internal server error"})
		return
	}

	// Return the data back to client
	ctx.JSON(http.StatusOK, AuthResponse{
		ID:           acc.ID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}
