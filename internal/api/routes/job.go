package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/api/handlers"
)

// JobRoutes registers job endpoints
func JobRoutes(rg *gin.RouterGroup, h *handlers.JobHandler) {
	jobs := rg.Group("/jobs")
	{
		jobs.POST("", h.CreateJob)
		jobs.GET("", h.ListJobs)
		jobs.GET("/:id", h.GetJob)
		jobs.DELETE("/:id", h.CancelJob)
		jobs.POST("/:id/restart", h.RestartJob)
		jobs.GET("/:id/logs", h.GetJobLogs)
		jobs.GET("/:id/checkpoints", h.GetJobCheckpoints)
	}
}
