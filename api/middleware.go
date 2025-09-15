package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/danglnh07/TaskManagement/db"
	"github.com/danglnh07/TaskManagement/service/security"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const claimsKey = "claims-key"

func (server *Server) AuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Get the token from request header
		token := strings.TrimSpace(strings.TrimPrefix(ctx.Request.Header.Get("Authorization"), "Bearer"))
		if token == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{"Missing Bearer token"})
			return
		}

		// Verify token
		claims, err := server.jwtService.VerifyToken(token)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{"Invalid token: " + err.Error()})
			return
		}

		// Check if the token version is match with database
		var account db.Account
		result := server.queries.DB.First(&account, claims.ID)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{"Invalid token: ID not exists"})
			return
		}

		if claims.Version != account.JWTTokenVersion {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{"Invalid token: token version not match"})
			return
		}

		// If match, put the claims into the context and forward to the next handler
		ctx.Set(claimsKey, claims)
		ctx.Next()
	}
}

func (server *Server) UserMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Get the claims from context
		claims, _ := ctx.Get(claimsKey)

		// Check if the role is user
		if claims.(*security.CustomClaims).Role != db.User {
			ctx.AbortWithStatusJSON(http.StatusForbidden, ErrorResponse{"Only user can access these routes"})
			return
		}

		ctx.Next()
	}
}
