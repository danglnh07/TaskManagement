package api

import (
	"log/slog"

	"github.com/danglnh07/TaskManagement/db"
	_ "github.com/danglnh07/TaskManagement/docs"
	"github.com/danglnh07/TaskManagement/service/event"
	"github.com/danglnh07/TaskManagement/service/security"
	"github.com/danglnh07/TaskManagement/util"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Server struct {
	router     *gin.Engine
	queries    *db.Queries
	jwtService *security.JWTService
	calendar   event.EventScheduler
	logger     *slog.Logger
	config     *util.Config
}

func NewServer(queries *db.Queries, logger *slog.Logger, config *util.Config) *Server {
	return &Server{
		router:     gin.Default(),
		queries:    queries,
		jwtService: security.NewJWTService(config),
		calendar:   event.NewGoogleCalendarManager(queries, config),
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
		tasks := api.Group("/tasks", server.AuthMiddleware(), server.UserMiddleware())
		{
			tasks.POST("", server.CreateTask)
			tasks.GET("/:id", server.GetTask)
			tasks.PUT("/:id", server.EditTask)
			tasks.DELETE("/:id", server.DeleteTask)
			tasks.PUT("/:id/status", server.SetTaskStatus)
			tasks.GET("", server.ListTasks)
		}
	}

	server.router.GET("/oauth2/callback", server.HandleCallback)

	// Swagger docs
	server.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

func (server *Server) Start() error {
	server.RegisterHandler()
	return server.router.Run(":8080")
}

type ErrorResponse struct {
	Message string `json:"error"`
}
