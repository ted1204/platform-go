package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/handlers"
	"github.com/linskybing/platform-go/middleware"
	"github.com/linskybing/platform-go/repositories"
	"github.com/linskybing/platform-go/services"
)

func RegisterRoutes(r *gin.Engine) {

	// init
	repos_instance := repositories.New()
	services_instance := services.New(repos_instance)
	handlers_instance := handlers.New(services_instance)

	// setup
	r.POST("/register", handlers_instance.User.Register)
	r.POST("/login", handlers_instance.User.Login)
	r.GET("/ws/exec", handlers.ExecWebSocketHandler)
	r.GET("/ws/monitoring/:namespace", handlers.WatchNamespaceHandler)
	auth := r.Group("/")
	auth.Use(middleware.JWTAuthMiddleware())
	{
		userGroup := auth.Group("/user-group")
		{
			userGroup.GET("", handlers_instance.UserGroup.GetUserGroup)
			userGroup.GET("/by-group", handlers_instance.UserGroup.GetUserGroupsByGID)
			userGroup.GET("/by-user", handlers_instance.UserGroup.GetUserGroupsByUID)

			userGroup.POST("", handlers_instance.UserGroup.CreateUserGroup)
			userGroup.PUT("", handlers_instance.UserGroup.UpdateUserGroup)
			userGroup.DELETE("", handlers_instance.UserGroup.DeleteUserGroup)
		}
		audit := auth.Group("/audit/logs")
		{
			audit.GET("", handlers_instance.Audit.GetAuditLogs)
		}
		instances := auth.Group("/instance")
		{
			instances.POST("/:id", handlers_instance.ConfigFile.CreateInstanceHandler)
			instances.DELETE("/:id", handlers_instance.ConfigFile.DestructInstanceHandler)
		}
		configFiles := auth.Group("/config-files")
		{
			configFiles.GET("", handlers_instance.ConfigFile.ListConfigFilesHandler)
			configFiles.GET("/:id", handlers_instance.ConfigFile.GetConfigFileHandler)
			configFiles.GET("/:id/resources", middleware.CheckPermissionByParam(repos_instance.View.GetGroupIDByConfigFileID, repos_instance.View), handlers_instance.Resource.ListResourcesByConfigFileID)
			configFiles.POST("", middleware.CheckPermissionPayloadByRepo("create_config_file", dto.CreateConfigFileInput{}, repos_instance), handlers_instance.ConfigFile.CreateConfigFileHandler)
			configFiles.PUT("/:id", middleware.CheckPermissionByParam(repos_instance.View.GetGroupIDByConfigFileID, repos_instance.View), handlers_instance.ConfigFile.UpdateConfigFileHandler)
			configFiles.DELETE("/:id", middleware.CheckPermissionByParam(repos_instance.View.GetGroupIDByConfigFileID, repos_instance.View), handlers_instance.ConfigFile.DeleteConfigFileHandler)
		}
		projects := auth.Group("/projects")
		{
			projects.GET("", handlers_instance.Project.GetProjects)
			projects.GET("/by-user", handlers_instance.Project.GetProjectsByUser)
			projects.GET("/:id", handlers_instance.Project.GetProjectByID)
			projects.GET("/:id/config-files", handlers_instance.ConfigFile.ListConfigFilesByProjectIDHandler)
			projects.GET("/:id/resources", handlers_instance.Resource.ListResourcesByProjectID)
			projects.POST("", middleware.CheckPermissionPayload("create_project", dto.CreateProjectDTO{}, repos_instance.View), handlers_instance.Project.CreateProject)
			projects.PUT("/:id", middleware.CheckPermissionByParam(repos_instance.Project.GetGroupIDByProjectID, repos_instance.View), handlers_instance.Project.UpdateProject)
			projects.DELETE("/:id", middleware.CheckPermissionByParam(repos_instance.Project.GetGroupIDByProjectID, repos_instance.View), handlers_instance.Project.DeleteProject)

		}
		users := auth.Group("/users")
		{
			users.GET("", handlers_instance.User.GetUsers)
			users.GET("/paging", handlers_instance.User.ListUsersPaging)
			users.GET("/:id", handlers_instance.User.GetUserByID)
			users.PUT("/:id", middleware.AuthorizeUserOrAdmin(repos_instance.View), handlers_instance.User.UpdateUser)
			users.DELETE("/:id", middleware.AuthorizeUserOrAdmin(repos_instance.View), handlers_instance.User.DeleteUser)
		}
		groups := auth.Group("/groups")
		{
			groups.GET("", handlers_instance.Group.GetGroups)
			groups.GET("/:id", handlers_instance.Group.GetGroupByID)
			groups.POST("", middleware.AuthorizeAdmin(repos_instance.View), handlers_instance.Group.CreateGroup)
			groups.PUT("/:id", middleware.AuthorizeAdmin(repos_instance.View), handlers_instance.Group.UpdateGroup)
			groups.DELETE("/:id", middleware.AuthorizeAdmin(repos_instance.View), handlers_instance.Group.DeleteGroup)
		}
		pvc := auth.Group("/pvc")
		{
			pvc.GET("/:namespace/:name", handlers.GetPVCHandler)
			pvc.GET("/list/:namespace", handlers.ListPVCsHandler)
			pvc.POST("", handlers.CreatePVCHandler)
			pvc.PUT("/expand", handlers.ExpandPVCHandler)
			pvc.DELETE("/:namespace/:name", handlers.DeletePVCHandler)
		}

	}
}
