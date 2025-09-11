package api

import (
	"log/slog"
	"net/http"

	"github.com/danglnh07/TaskManagement/util"
	"github.com/gin-gonic/gin"
)

type Server struct {
	router *gin.Engine
	logger *slog.Logger
	config *util.Config
}

func NewServer(logger *slog.Logger, config *util.Config) *Server {
	return &Server{
		router: gin.Default(),
		logger: logger,
		config: config,
	}
}

func (server *Server) RegisterHandler() {
	server.router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
}

func (server *Server) Start() error {
	server.RegisterHandler()
	return server.router.Run(":8080")
}
