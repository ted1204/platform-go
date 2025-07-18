package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/handlers"
	"github.com/linskybing/platform-go/middleware"
	"github.com/linskybing/platform-go/repositories"
)

func RegisterRoutes(r *gin.Engine) {
	r.POST("/register", handlers.Register)
	r.POST("/login", handlers.Login)

	auth := r.Group("/")
	auth.Use(middleware.JWTAuthMiddleware())
	{
		audit := auth.Group("/audit/logs")
		{
			audit.GET("", handlers.GetAuditLogs)
		}

		projects := auth.Group("/projects")
		{
			projects.GET("", handlers.GetProjects)
			projects.GET("/:id", handlers.GetProjectByID)
			projects.POST("", middleware.CheckPermissionPayload("create_project", dto.CreateProjectDTO{}), handlers.CreateProject)
			projects.PUT("/:id", middleware.CheckPermissionByParam(repositories.GetGroupIDByProjectID), handlers.UpdateProject)
			projects.DELETE("/:id", middleware.CheckPermissionByParam(repositories.GetGroupIDByProjectID), handlers.DeleteProject)
		}
		users := auth.Group("/users")
		{
			users.GET("", handlers.GetUsers)
			users.GET(":id", handlers.GetUserByID)
			users.PUT(":id", middleware.AuthorizeUserOrAdmin(), handlers.UpdateUser)
			users.DELETE(":id", middleware.AuthorizeUserOrAdmin(), handlers.DeleteUser)
		}
		groups := auth.Group("/groups")
		{
			groups.GET("", handlers.GetGroups)
			groups.GET(":id", handlers.GetGroupByID)
			groups.POST("", middleware.AuthorizeAdmin(), handlers.CreateGroup)
			groups.PUT(":id", middleware.AuthorizeAdmin(), handlers.UpdateGroup)
			groups.DELETE(":id", middleware.AuthorizeAdmin(), handlers.DeleteGroup)
		}
	}
}
