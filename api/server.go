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
	// Group everything under the /api endpoint
	api := server.router.Group("/api")
	{
		api.GET("/auth", server.HandleAuth)

		// Task group
		tasks := api.Group("/tasks", server.AuthMiddleware())
		{
			tasks.POST("", server.CreateTask)
			tasks.GET("/:id", server.GetTask)
			tasks.PUT("/:id", server.EditTask)
			tasks.DELETE("/:id", server.DeleteTask)
			tasks.PUT("/:id/status", server.SetTaskStatus)
		}
	}

	server.router.GET("/oauth2/callback", server.HandleCallback)
}

func (server *Server) Start() error {
	server.RegisterHandler()
	return server.router.Run(":8080")
}

type ErrorResponse struct {
	Message string `json:"error"`
}
