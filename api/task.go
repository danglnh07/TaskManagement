package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/danglnh07/TaskManagement/db"
	"github.com/danglnh07/TaskManagement/service/security"
	"github.com/gin-gonic/gin"
	_ "github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
)

type CreateTaskRequest struct {
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description" binding:"required"`
	Category    string    `json:"category" binding:"required"`
	Deadline    time.Time `json:"deadline" binding:"required" time_format:"2006-01-02 15:04:05" time_utc:"true"` // This value is fixed in Go, cannot change to other
}

type TaskResponse struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Deadline    time.Time `json:"deadline"`
	Status      db.Status `json:"status"`
}

func (server *Server) CreateTask(ctx *gin.Context) {
	// Get the request
	var req CreateTaskRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		server.logger.Error("", "error", err)
		ctx.JSON(http.StatusBadRequest, ErrorResponse{"Invalid request body"})
		return
	}

	// Check if the deadline is actually in the future
	if req.Deadline.Add(time.Minute * 15).Before(time.Now()) {
		server.logger.Error("", "error", fmt.Errorf("deadline invalid"))
		ctx.JSON(http.StatusBadRequest, ErrorResponse{"The deadline should be at least 15 minutes from the current time"})
		return
	}

	// Get the ID from claims
	claims, _ := ctx.Get(claimsKey)
	id := claims.(*security.CustomClaims).ID

	// Create task
	var task = db.Task{
		Model:       gorm.Model{},
		IssuerID:    id,
		TaskName:    req.Name,
		Description: req.Description,
		Category:    req.Category,
		Deadline:    req.Deadline,
		Status:      db.Incomplete,
	}
	result := server.queries.DB.Create(&task)

	if result.Error != nil {
		server.logger.Error("POST /api/tasks: failed to create task", "error", result.Error)
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{"Internal server error"})
		return
	}

	// Return data back to client
	ctx.JSON(http.StatusCreated, TaskResponse{
		ID:          task.ID,
		Name:        task.TaskName,
		Description: task.Description,
		Category:    task.Category,
		Deadline:    task.Deadline,
		Status:      task.Status,
	})

	// Set up schedule jobs
}

type EditTaskRequest struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Deadline    time.Time `json:"deadline" time_format:"2006-01-02 15:04:05" time_utc:"true"` // This value is fixed in Go, cannot change to other
}

func (server *Server) EditTask(ctx *gin.Context) {
	// Get the ID from path parameter
	id := ctx.Param("id")

	// Get the old value using ID
	var task db.Task
	result := server.queries.DB.First(&task, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{"Task not found"})
		return
	}

	// Check if the requester is the owner of this tasks
	claims, _ := ctx.Get(claimsKey)
	userID := claims.(*security.CustomClaims).ID
	if userID != task.IssuerID {
		ctx.JSON(http.StatusForbidden, ErrorResponse{"You are not the owner of this task"})
		return
	}

	// Get the request
	var req EditTaskRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		server.logger.Error("", "error", err)
		ctx.JSON(http.StatusBadRequest, ErrorResponse{"Invalid request body"})
		return
	}

	// Update the task with new value
	if req.Name != "" {
		task.TaskName = req.Name
	}

	if req.Description != "" {
		task.Description = req.Description
	}

	if req.Category != "" {
		task.Category = req.Category
	}

	if !req.Deadline.IsZero() {
		// Check if the deadline is actually in the future
		if req.Deadline.Add(time.Minute * 15).Before(time.Now()) {
			server.logger.Error("", "error", fmt.Errorf("deadline invalid"))
			ctx.JSON(http.StatusBadRequest, ErrorResponse{"The deadline should be at least 15 minutes from the current time"})
			return
		}
		task.Deadline = req.Deadline
	}

	result = server.queries.DB.Save(task)
	if result.Error != nil {
		server.logger.Error("PUT /api/tasks/:id: failed to edit task", "error", result.Error)
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{"Internal server error"})
		return
	}

	// Return data back to client
	ctx.JSON(http.StatusCreated, TaskResponse{
		ID:          task.ID,
		Name:        task.TaskName,
		Description: task.Description,
		Category:    task.Category,
		Deadline:    task.Deadline,
		Status:      task.Status,
	})

	// Set up schedule jobs
}

func (server *Server) GetTask(ctx *gin.Context) {
	// Get the ID in path parameter
	id := ctx.Param("id")

	// Get task from database
	var task db.Task
	result := server.queries.DB.First(&task, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, ErrorResponse{"Cannot find any task with this ID"})
			return
		}

		server.logger.Error("GET /api/tasks/:id: failed to fetch task data", "error", result.Error)
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{"Internal server error"})
		return
	}

	// Return data back to client
	ctx.JSON(http.StatusOK, TaskResponse{
		ID:          task.ID,
		Name:        task.TaskName,
		Description: task.Description,
		Category:    task.Category,
		Deadline:    task.Deadline,
		Status:      task.Status,
	})
}

func (server *Server) DeleteTask(ctx *gin.Context) {
	// Get the ID in path parameter
	id := ctx.Param("id")

	// Get task from database
	var task db.Task
	result := server.queries.DB.First(&task, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusBadRequest, ErrorResponse{"No such task with this ID for deleting"})
			return
		}

		server.logger.Error("DELETE /api/tasks/:id: failed to fetch task before deleting", "error", result.Error)
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{"Internal server error"})
		return
	}

	// Check if the requester is the owner of the task
	claims, _ := ctx.Get(claimsKey)
	userID := claims.(*security.CustomClaims).ID
	if userID != task.IssuerID {
		ctx.JSON(http.StatusForbidden, ErrorResponse{"You are not the owner of this task"})
		return
	}

	// Delete task
	result = server.queries.DB.Delete(&db.Task{}, id)
	if result.Error != nil {
		server.logger.Error("DELETE /api/tasks/:id: failed to delete task", "error", result.Error)
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{"Internal server error"})
		return
	}

	ctx.JSON(http.StatusOK, "Task delete successfully")
}

type TaskStatusRequest struct {
	Status db.Status `json:"status" binding:"required"`
}

func (server *Server) SetTaskStatus(ctx *gin.Context) {
	// Get ID from path parameter
	id := ctx.Param("id")

	// Get task from database
	var task db.Task
	result := server.queries.DB.First(&task, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusBadRequest, ErrorResponse{"No such task with this ID for deleting"})
			return
		}

		server.logger.Error("DELETE /api/tasks/:id: failed to fetch task before deleting", "error", result.Error)
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{"Internal server error"})
		return
	}

	// Check if the requester is the owner of the task
	claims, _ := ctx.Get(claimsKey)
	userID := claims.(*security.CustomClaims).ID
	if userID != task.IssuerID {
		ctx.JSON(http.StatusForbidden, ErrorResponse{"You are not the owner of this task"})
		return
	}

	// Parse request
	var req TaskStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{"Invalid JSON data"})
		return
	}

	// Check if status value is correct
	if req.Status != db.Incomplete && req.Status != db.Cancel && req.Status != db.Complete {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{"Invalid status value"})
		return
	}

	// Set task's status
	task.Status = db.Status(req.Status)
	result = server.queries.DB.Save(&task)
	if result.Error != nil {
		server.logger.Error("PUT /api/tasks/:id/status: failed to set tasks status", "error", result.Error)
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{"Internal server error"})
		return
	}

	// Return data back to client
	ctx.JSON(http.StatusOK, TaskResponse{
		ID:          task.ID,
		Name:        task.TaskName,
		Description: task.Description,
		Category:    task.Category,
		Deadline:    task.Deadline,
		Status:      task.Status,
	})
}
