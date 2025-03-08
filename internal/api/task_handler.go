package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/yourusername/taskscheduler/internal/model"
	"github.com/yourusername/taskscheduler/internal/repository"
)

// TaskHandler handles HTTP requests for task operations
type TaskHandler struct {
	repo repository.TaskRepository
}

// NewTaskHandler creates a new task handler
func NewTaskHandler(repo repository.TaskRepository) *TaskHandler {
	return &TaskHandler{
		repo: repo,
	}
}

// RegisterRoutes registers the routes for the task API
func (h *TaskHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/tasks", func(r chi.Router) {
		r.Post("/", h.CreateTask)
		r.Get("/", h.ListTasks)
		r.Get("/{id}", h.GetTask)
		r.Put("/{id}", h.UpdateTask)
		r.Delete("/{id}", h.DeleteTask)
	})
}

// TaskRequest represents a request to create or update a task
type TaskRequest struct {
	Name         string   `json:"name"`
	Command      string   `json:"command"`
	Schedule     string   `json:"schedule"`
	Retries      int      `json:"retries"`
	Timeout      int      `json:"timeout"`
	Dependencies []string `json:"dependencies"`
}

// CreateTask handles POST /api/tasks
func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Name == "" || req.Command == "" {
		http.Error(w, "Name and command are required", http.StatusBadRequest)
		return
	}

	// Create task
	task := model.NewTask(req.Name, req.Command, req.Schedule)
	if req.Retries > 0 {
		task.Retries = req.Retries
	}
	if req.Timeout > 0 {
		task.Timeout = req.Timeout
	}
	task.Dependencies = req.Dependencies

	// Save to repository
	if err := h.repo.Create(r.Context(), task); err != nil {
		http.Error(w, "Failed to create task: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return created task
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

// ListTasks handles GET /api/tasks
func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	// Parse pagination parameters
	limit := 100
	offset := 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Get tasks from repository
	tasks, err := h.repo.List(r.Context(), limit, offset)
	if err != nil {
		http.Error(w, "Failed to list tasks: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return task list
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

// GetTask handles GET /api/tasks/{id}
func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	// Get task from repository
	task, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if err == repository.ErrTaskNotFound {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to get task: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return task
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// UpdateTask handles PUT /api/tasks/{id}
func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	var req TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get existing task
	task, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if err == repository.ErrTaskNotFound {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to get task: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update task fields
	if req.Name != "" {
		task.Name = req.Name
	}
	if req.Command != "" {
		task.Command = req.Command
	}
	if req.Schedule != "" {
		task.Schedule = req.Schedule
	}
	if req.Retries > 0 {
		task.Retries = req.Retries
	}
	if req.Timeout > 0 {
		task.Timeout = req.Timeout
	}
	if req.Dependencies != nil {
		task.Dependencies = req.Dependencies
	}

	// Save updated task
	if err := h.repo.Update(r.Context(), task); err != nil {
		http.Error(w, "Failed to update task: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated task
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// DeleteTask handles DELETE /api/tasks/{id}
func (h *TaskHandler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	// Delete task from repository
	if err := h.repo.Delete(r.Context(), id); err != nil {
		if err == repository.ErrTaskNotFound {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to delete task: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success
	w.WriteHeader(http.StatusNoContent)
} 