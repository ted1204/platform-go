package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	appjob "github.com/linskybing/platform-go/internal/application/job"
	"github.com/linskybing/platform-go/internal/repository"
	"github.com/linskybing/platform-go/pkg/response"
	"github.com/linskybing/platform-go/pkg/utils"
)

// JobHandler handles job-related HTTP endpoints.
type JobHandler struct {
	svc   *appjob.Service
	repos *repository.Repos
}

// NewJobHandler creates a new job handler.
func NewJobHandler(svc *appjob.Service, repos *repository.Repos) *JobHandler {
	return &JobHandler{svc: svc, repos: repos}
}

// CreateJob handles job creation.
func (h *JobHandler) CreateJob(c *gin.Context) {
	var req appjob.CreateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid payload: " + err.Error()})
		return
	}

	uid, err := utils.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "unauthorized"})
		return
	}

	job, err := h.svc.CreateJob(c.Request.Context(), uid, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response.SuccessResponse{Code: 0, Message: "created", Data: job})
}

// ListJobs lists jobs for the user; super admin can view all.
func (h *JobHandler) ListJobs(c *gin.Context) {
	uid, err := utils.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "unauthorized"})
		return
	}

	isAdmin, err := utils.IsSuperAdmin(uid, h.repos.View)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "failed to check role"})
		return
	}

	jobs, err := h.svc.ListJobs(c.Request.Context(), uid, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{Code: 0, Message: "success", Data: jobs})
}

// GetJob returns a single job by ID.
func (h *JobHandler) GetJob(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid id"})
		return
	}

	job, err := h.svc.GetJob(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{Code: 0, Message: "success", Data: job})
}

// CancelJob terminates a job (super admin only).
func (h *JobHandler) CancelJob(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid id"})
		return
	}

	uid, err := utils.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "unauthorized"})
		return
	}

	isAdmin, err := utils.IsSuperAdmin(uid, h.repos.View)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "failed to check role"})
		return
	}
	if !isAdmin {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "super admin only"})
		return
	}

	if err := h.svc.CancelJob(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{Code: 0, Message: "cancelled"})
}

// RestartJob restarts a job, optionally from a checkpoint.
func (h *JobHandler) RestartJob(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid id"})
		return
	}

	var checkpointID *uint
	if raw := c.Query("checkpoint_id"); raw != "" {
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid checkpoint_id"})
			return
		}
		val := uint(v)
		checkpointID = &val
	}

	if err := h.svc.RestartJob(c.Request.Context(), id, checkpointID); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{Code: 0, Message: "restarted"})
}

// GetJobLogs returns job logs.
func (h *JobHandler) GetJobLogs(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid id"})
		return
	}

	limit := 100
	offset := 0
	if raw := c.Query("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if raw := c.Query("offset"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}

	logs, err := h.svc.GetJobLogs(c.Request.Context(), id, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{Code: 0, Message: "success", Data: logs})
}

// GetJobCheckpoints returns job checkpoints.
func (h *JobHandler) GetJobCheckpoints(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid id"})
		return
	}

	checkpoints, err := h.svc.GetJobCheckpoints(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{Code: 0, Message: "success", Data: checkpoints})
}

// StreamJobs streams job updates over WebSocket, scoped by user unless super admin.
func (h *JobHandler) StreamJobs(c *gin.Context) {
	uid, err := utils.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "unauthorized"})
		return
	}

	isAdmin, err := utils.IsSuperAdmin(uid, h.repos.View)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "failed to check role"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "websocket upgrade failed: " + err.Error()})
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	// Heartbeat handling
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	ctx := c.Request.Context()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Reader to consume control frames and detect close
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			jobs, err := h.svc.ListJobs(c.Request.Context(), uid, isAdmin)
			if err != nil {
				_ = conn.WriteMessage(websocket.TextMessage, []byte("{}"))
				continue
			}

			payload, _ := json.Marshal(jobs)
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		}
	}
}

// StreamJobLogs streams logs for a specific job over WebSocket.
func (h *JobHandler) StreamJobLogs(c *gin.Context) {
	jobID, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid id"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "websocket upgrade failed: " + err.Error()})
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	ctx := c.Request.Context()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	lastCount := 0

	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			logs, err := h.svc.GetJobLogs(c.Request.Context(), jobID, 0, 0)
			if err != nil {
				continue
			}
			if len(logs) == 0 || len(logs) == lastCount {
				continue
			}

			newLogs := logs[lastCount:]
			lines := make([]string, 0, len(newLogs))
			for _, l := range newLogs {
				lines = append(lines, l.Content)
			}
			lastCount = len(logs)

			payload, _ := json.Marshal(lines)
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		}
	}
}
