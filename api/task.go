package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/danglnh07/TaskManagement/db"
	"github.com/danglnh07/TaskManagement/service/security"
	"github.com/gin-gonic/gin"
	_ "github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
)

// Request struct for create task action
type CreateTaskRequest struct {
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description" binding:"required"`
	Category    string    `json:"category" binding:"required"`
	Deadline    time.Time `json:"deadline" binding:"required" time_format:"2006-01-02 15:04:05" time_utc:"true"` // This value is fixed in Go, cannot change to other
}

// Response struct for task (used across all actions)
type TaskResponse struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	// Deadline should always be in UTC (ISO 8601 format, with "Z" suffix)
	// Example: 2025-09-13T16:00:00Z
	Deadline time.Time `json:"deadline"`
	Status   db.Status `json:"status"`
}

// CreateTask godoc
// @Summary      Create a new task
// @Description  Creates a new task owned by the authenticated user. The deadline must be at least 15 minutes in the future.
// @Tags         tasks
// @Accept       json
// @Produce      json
// @Param        request  body      CreateTaskRequest  true  "Task details"
// @Success      201      {object}  TaskResponse       "Task created successfully"
// @Failure      400      {object}  ErrorResponse      "Invalid request body or invalid deadline"
// @Failure      500      {object}  ErrorResponse      "Internal server error"
// @Security     BearerAuth
// @Router       /api/tasks [post]
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
	err := server.calendar.CreateEvent(&task)
	if err != nil {
		server.logger.Error("POST /api/tasks: failed to create Google Calendar event", "error", err)
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{"Internal server error"})
		return
	}
	server.logger.Info("POST /api/tasks: create calendar event successfully")
}

// Request struct for edit task action
type EditTaskRequest struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Deadline    time.Time `json:"deadline" time_format:"2006-01-02 15:04:05" time_utc:"true"` // This value is fixed in Go, cannot change to other
}

// EditTask godoc
// @Summary      Edit an existing task
// @Description  Updates the fields of an existing task. Only the task owner can edit it. Any provided deadline must be at least 15 minutes in the future.
// @Tags         tasks
// @Accept       json
// @Produce      json
// @Param        id       path      int               true  "Task ID"
// @Param        request  body      EditTaskRequest   true  "Updated task details"
// @Success      201      {object}  TaskResponse      "Task updated successfully"
// @Failure      400      {object}  ErrorResponse     "Invalid request body or task not found"
// @Failure      403      {object}  ErrorResponse     "User is not the owner of the task"
// @Failure      500      {object}  ErrorResponse     "Internal server error"
// @Security     BearerAuth
// @Router       /api/tasks/{id} [put]
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

	// Update event in Google Calendar
	err := server.calendar.UpdateEvent(&task)
	if err != nil {
		server.logger.Error("PUT /api/tasks/:id: failed to update calendar event", "error", err)
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{"Internal server error"})
		return
	}
	server.logger.Info("PUT /api/tasks/:id: update calendar successfully")
}

// GetTask godoc
// @Summary      Get a task by ID
// @Description  Retrieves details of a specific task by its ID.
// @Tags         tasks
// @Accept       json
// @Produce      json
// @Param        id   path      int           true  "Task ID"
// @Success      200  {object}  TaskResponse  "Task retrieved successfully"
// @Failure      404  {object}  ErrorResponse "Task not found"
// @Failure      500  {object}  ErrorResponse "Internal server error"
// @Security     BearerAuth
// @Router       /api/tasks/{id} [get]
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

// DeleteTask godoc
// @Summary      Delete a task
// @Description  Deletes a specific task by its ID. Only the owner of the task can perform this action.
// @Tags         tasks
// @Accept       json
// @Produce      json
// @Param        id   path      int           true  "Task ID"
// @Success      200  {string}  string        "Task delete successfully"
// @Failure      400  {object}  ErrorResponse "No such task with this ID for deleting"
// @Failure      403  {object}  ErrorResponse "You are not the owner of this task"
// @Failure      500  {object}  ErrorResponse "Internal server error"
// @Security     BearerAuth
// @Router       /api/tasks/{id} [delete]
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

// Request struct for set task status action
type TaskStatusRequest struct {
	Status db.Status `json:"status" binding:"required"`
}

// SetTaskStatus godoc
// @Summary      Update task status
// @Description  Updates the status of a specific task by its ID. Only the owner of the task can perform this action.
// @Tags         tasks
// @Accept       json
// @Produce      json
// @Param        id    path      int                true  "Task ID"
// @Param        body  body      TaskStatusRequest  true  "Task status payload"
// @Success      200   {object}  TaskResponse       "Task status updated successfully"
// @Failure      400   {object}  ErrorResponse      "Invalid JSON data or invalid status value"
// @Failure      403   {object}  ErrorResponse      "You are not the owner of this task"
// @Failure      500   {object}  ErrorResponse      "Internal server error"
// @Security     BearerAuth
// @Router       /api/tasks/{id}/status [put]
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

// Response struct for list tasks action
type ListTasksResponse struct {
	Tasks []TaskResponse `json:"tasks"`

	// Here, total is the total of the tasks in database for that account, it's not equal to the len of tasks
	Total int64 `json:"total"`
}

// ListTasks godoc
// @Summary      List tasks
// @Description  Retrieves a paginated list of tasks for the authenticated user.
// @Tags         tasks
// @Accept       json
// @Produce      json
// @Param        page_size   query     int     true   "Number of tasks per page (1-100)"
// @Param        page_index  query     int     true   "Page index (0-based)"
// @Param        filter      query     string  false  "Filter tasks by status. If omitted, return all tasks" Enums(incomplete,complete,cancel)
// @Success      200         {object}  ListTasksResponse
// @Failure      400         {object}  ErrorResponse "Invalid request parameters"
// @Failure      500         {object}  ErrorResponse "Internal server error"
// @Security     BearerAuth
// @Router       /api/tasks [get]
func (server *Server) ListTasks(ctx *gin.Context) {
	// Get the account ID from claims
	claims, _ := ctx.Get(claimsKey)
	accountID := claims.(*security.CustomClaims).ID

	// First, get the query parameter: page_size, page_index (for pagination)
	pageSize, err := strconv.Atoi(ctx.Query("page_size"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{"Invalid value for page_size"})
		return
	}
	if pageSize > 100 || pageSize < 1 {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{"page_size should be an integer range [1, 100]"})
		return
	}

	pageIndex, err := strconv.Atoi(ctx.Query("page_index"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{"Invalid value for page_index"})
		return
	}
	if pageIndex < 0 {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{"page_size should be a non negative integer"})
		return
	}

	// Building the base query
	tasks := make([]TaskResponse, 0)
	query := server.queries.DB.
		Table("tasks").
		Where("issuer_id = ?", accountID).
		Limit(pageSize).
		Offset(pageIndex * pageSize).
		Select("id, task_name as name, description, category, deadline, status")

	// Get the filter. If no value provided, it will get all tasks regarded of status
	filter := ctx.Query("filter")
	if filter != "" {
		if filter != string(db.Incomplete) && filter != string(db.Complete) && filter != string(db.Cancel) {
			ctx.JSON(http.StatusBadRequest, ErrorResponse{"Invalid for filter."})
			return
		}
		query.Where("status = ?", filter)
	}

	result := query.Scan(&tasks)
	if result.Error != nil {
		server.logger.Error("GET /api/tasks: failed to get list of tasks", "error", err)
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{"Internal server error"})
		return
	}

	// Get the total of tasks this account has (for pagination)
	var total int64
	result = server.queries.DB.Table("tasks").Where("issuer_id = ?", accountID).Count(&total)
	if result.Error != nil {
		server.logger.Error("GET /api/tasks: failed to get total tasks", "error", err)
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{"Internal server error"})
		return
	}

	// Return result back to client
	ctx.JSON(http.StatusOK, ListTasksResponse{
		Tasks: tasks,
		Total: total,
	})
}
