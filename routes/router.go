package routes

import (
    "github.com/gin-gonic/gin"
    "platform-go/handlers"
)

func RegisterProjectRoutes(r *gin.Engine) {
    projects := r.Group("/projects")
    {
        projects.GET("", handlers.GetProjects)
        projects.GET("/:id", handlers.GetProjectByID)
        projects.POST("", handlers.CreateProject)
        projects.PUT("/:id", handlers.UpdateProject)
        projects.DELETE("/:id", handlers.DeleteProject)
    }
}
