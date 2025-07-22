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
	r.GET("/ws/exec", handlers.ExecWebSocketHandler)

	auth := r.Group("/")
	auth.Use(middleware.JWTAuthMiddleware())
	{
		userGroup := auth.Group("/user-group")
		{
			userGroup.GET("", handlers.GetUserGroup)
			userGroup.GET("/by-group", handlers.GetUserGroupsByGID)
			userGroup.GET("/by-user", handlers.GetUserGroupsByUID)

			userGroup.POST("", handlers.CreateUserGroup)
			userGroup.PUT("", handlers.UpdateUserGroup)
			userGroup.DELETE("", handlers.DeleteUserGroup)
		}
		audit := auth.Group("/audit/logs")
		{
			audit.GET("", handlers.GetAuditLogs)
		}
		instances := auth.Group("/instance")
		{
			instances.POST("/:id", handlers.CreateInstanceHandler)
			instances.DELETE("/:id", handlers.DestructInstanceHandler)
		}
		configFiles := auth.Group("/config-files")
		{
			configFiles.GET("", handlers.ListConfigFilesHandler)
			configFiles.GET("/:id", handlers.GetConfigFileHandler)
			configFiles.GET("/:id/resources", middleware.CheckPermissionByParam(repositories.GetGroupIDByConfigFileID), handlers.ListResourcesByConfigFileID)
			configFiles.POST("", middleware.CheckPermissionPayload("create_config_file", dto.CreateConfigFileInput{}), handlers.CreateConfigFileHandler)
			configFiles.PUT("/:id", middleware.CheckPermissionByParam(repositories.GetGroupIDByConfigFileID), handlers.UpdateConfigFileHandler)
			configFiles.DELETE("/:id", middleware.CheckPermissionByParam(repositories.GetGroupIDByConfigFileID), handlers.DeleteConfigFileHandler)
		}
		projects := auth.Group("/projects")
		{
			projects.GET("", handlers.GetProjects)
			projects.GET("/:id", handlers.GetProjectByID)
			projects.GET("/:id/config-files", handlers.ListConfigFilesByProjectIDHandler)
			projects.GET("/:id/resources", handlers.ListResourcesByProjectID)
			projects.POST("", middleware.CheckPermissionPayload("create_project", dto.CreateProjectDTO{}), handlers.CreateProject)
			projects.PUT("/:id", middleware.CheckPermissionByParam(repositories.GetGroupIDByProjectID), handlers.UpdateProject)
			projects.DELETE("/:id", middleware.CheckPermissionByParam(repositories.GetGroupIDByProjectID), handlers.DeleteProject)

		}
		users := auth.Group("/users")
		{
			users.GET("", handlers.GetUsers)
			users.GET("/:id", handlers.GetUserByID)
			users.PUT("/:id", middleware.AuthorizeUserOrAdmin(), handlers.UpdateUser)
			users.DELETE("/:id", middleware.AuthorizeUserOrAdmin(), handlers.DeleteUser)
		}
		groups := auth.Group("/groups")
		{
			groups.GET("", handlers.GetGroups)
			groups.GET("/:id", handlers.GetGroupByID)
			groups.POST("", middleware.AuthorizeAdmin(), handlers.CreateGroup)
			groups.PUT("/:id", middleware.AuthorizeAdmin(), handlers.UpdateGroup)
			groups.DELETE("/:id", middleware.AuthorizeAdmin(), handlers.DeleteGroup)
		}
	}
}
