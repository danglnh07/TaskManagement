package api

import (
	"log/slog"

	"github.com/danglnh07/TaskManagement/db"
	"github.com/danglnh07/TaskManagement/service/security"
	"github.com/danglnh07/TaskManagement/util"
	"github.com/gin-gonic/gin"
)

type Server struct {
	router     *gin.Engine
	queries    *db.Queries
	jwtService *security.JWTService
	logger     *slog.Logger
	config     *util.Config
}

func NewServer(queries *db.Queries, logger *slog.Logger, config *util.Config) *Server {
	return &Server{
		router:     gin.Default(),
		queries:    queries,
		jwtService: security.NewJWTService(config),
		logger:     logger,
		config:     config,
	}
}

func (server *Server) RegisterHandler() {
	server.router.GET("/api/auth", server.HandleAuth)
	server.router.GET("/oauth2/callback", server.HandleCallback)
}

func (server *Server) Start() error {
	server.RegisterHandler()
	return server.router.Run(":8080")
}

type ErrorResponse struct {
	Message string `json:"error"`
}
