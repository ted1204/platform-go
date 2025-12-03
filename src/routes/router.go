package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/handlers"
	"github.com/linskybing/platform-go/src/middleware"
	"github.com/linskybing/platform-go/src/repositories"
	"github.com/linskybing/platform-go/src/services"
)

func RegisterRoutes(r *gin.Engine) {

	// init
	repos_instance := repositories.New()
	services_instance := services.New(repos_instance)
	handlers_instance := handlers.New(services_instance)
	authMiddleware := middleware.NewAuth(repos_instance)

	// setup
	r.POST("/register", handlers_instance.User.Register)
	r.POST("/login", handlers_instance.User.Login)
	r.POST("/logout", handlers_instance.User.Logout)
	r.GET("/ws/exec", handlers.ExecWebSocketHandler)
	r.GET("/ws/monitoring/:namespace", handlers.WatchNamespaceHandler)
	auth := r.Group("/")
	auth.Use(middleware.JWTAuthMiddleware())
	{
		auth.GET("/ws/monitoring", handlers.WatchUserNamespaceHandler)
		userGroup := auth.Group("/user-group")
		{
			userGroup.GET("", authMiddleware.Admin(), handlers_instance.UserGroup.GetUserGroup)
			userGroup.GET("/by-group", handlers_instance.UserGroup.GetUserGroupsByGID)
			userGroup.GET("/by-user", handlers_instance.UserGroup.GetUserGroupsByUID)

			userGroup.POST("", authMiddleware.GroupAdmin(middleware.FromPayload(dto.UserGroupInputDTO{})), handlers_instance.UserGroup.CreateUserGroup)
			userGroup.PUT("", authMiddleware.GroupAdmin(middleware.FromPayload(dto.UserGroupInputDTO{})), handlers_instance.UserGroup.UpdateUserGroup)
			userGroup.DELETE("", authMiddleware.GroupAdmin(middleware.FromPayload(dto.UserGroupDeleteDTO{})), handlers_instance.UserGroup.DeleteUserGroup)
		}
		audit := auth.Group("/audit/logs")
		{
			audit.GET("", handlers_instance.Audit.GetAuditLogs)
		}
		instances := auth.Group("/instance")
		{
			instances.POST("/:id", authMiddleware.GroupMember(middleware.FromIDParam(repos_instance.View.GetGroupIDByConfigFileID)), handlers_instance.ConfigFile.CreateInstanceHandler)
			instances.DELETE("/:id", authMiddleware.GroupMember(middleware.FromIDParam(repos_instance.View.GetGroupIDByConfigFileID)), handlers_instance.ConfigFile.DestructInstanceHandler)
		}
		configFiles := auth.Group("/config-files")
		{
			configFiles.GET("", authMiddleware.Admin(), handlers_instance.ConfigFile.ListConfigFilesHandler)
			configFiles.GET("/:id", authMiddleware.GroupMember(middleware.FromIDParam(repos_instance.View.GetGroupIDByConfigFileID)), handlers_instance.ConfigFile.GetConfigFileHandler)
			configFiles.GET("/:id/resources", authMiddleware.GroupMember(middleware.FromIDParam(repos_instance.View.GetGroupIDByConfigFileID)), handlers_instance.Resource.ListResourcesByConfigFileID)
			configFiles.POST("", authMiddleware.GroupMember(middleware.FromPayload(dto.CreateConfigFileInput{})), handlers_instance.ConfigFile.CreateConfigFileHandler)
			configFiles.PUT("/:id", authMiddleware.GroupMember(middleware.FromIDParam(repos_instance.View.GetGroupIDByConfigFileID)), handlers_instance.ConfigFile.UpdateConfigFileHandler)
			configFiles.DELETE("/:id", authMiddleware.GroupMember(middleware.FromIDParam(repos_instance.View.GetGroupIDByConfigFileID)), handlers_instance.ConfigFile.DeleteConfigFileHandler)
		}
		projects := auth.Group("/projects")
		{
			projects.GET("", handlers_instance.Project.GetProjects)
			projects.GET("/by-user", handlers_instance.Project.GetProjectsByUser)
			projects.GET("/:id", handlers_instance.Project.GetProjectByID)
			projects.GET("/:id/config-files", handlers_instance.ConfigFile.ListConfigFilesByProjectIDHandler)
			projects.GET("/:id/resources", handlers_instance.Resource.ListResourcesByProjectID)
			projects.POST("", authMiddleware.GroupMember(middleware.FromPayload(dto.CreateProjectDTO{})), handlers_instance.Project.CreateProject)
			projects.PUT("/:id", authMiddleware.GroupMember(middleware.FromIDParam(repos_instance.Project.GetGroupIDByProjectID)), handlers_instance.Project.UpdateProject)
			projects.DELETE("/:id", authMiddleware.GroupMember(middleware.FromIDParam(repos_instance.Project.GetGroupIDByProjectID)), handlers_instance.Project.DeleteProject)

		}
		users := auth.Group("/users")
		{
			users.GET("", handlers_instance.User.GetUsers)
			users.GET("/paging", handlers_instance.User.ListUsersPaging)
			users.GET("/:id", handlers_instance.User.GetUserByID)
			users.PUT("/:id", authMiddleware.UserOrAdmin(), handlers_instance.User.UpdateUser)
			users.DELETE("/:id", authMiddleware.UserOrAdmin(), handlers_instance.User.DeleteUser)
		}
		groups := auth.Group("/groups")
		{
			groups.GET("", handlers_instance.Group.GetGroups)
			groups.GET("/:id", handlers_instance.Group.GetGroupByID)
			groups.POST("", authMiddleware.Admin(), handlers_instance.Group.CreateGroup)
			groups.PUT("/:id", authMiddleware.Admin(), handlers_instance.Group.UpdateGroup)
			groups.DELETE("/:id", authMiddleware.Admin(), handlers_instance.Group.DeleteGroup)
		}
		k8s := auth.Group("/k8s")
		{
			k8s.POST("/jobs", authMiddleware.Admin(), handlers_instance.K8s.CreateJob)
			
			// PVC routes moved here
			pvc := k8s.Group("/pvc")
			{
				pvc.GET("/:namespace/:name", authMiddleware.Admin(), handlers_instance.K8s.GetPVC)
				pvc.GET("/list/:namespace", authMiddleware.Admin(), handlers_instance.K8s.ListPVCs)
				pvc.GET("/by-project/:id", authMiddleware.GroupMember(middleware.FromIDParam(repos_instance.Project.GetGroupIDByProjectID)), handlers_instance.K8s.ListPVCsByProject)
				pvc.POST("", authMiddleware.Admin(), handlers_instance.K8s.CreatePVC)
				pvc.PUT("/expand", authMiddleware.Admin(), handlers_instance.K8s.ExpandPVC)
				pvc.DELETE("/:namespace/:name", authMiddleware.Admin(), handlers_instance.K8s.DeletePVC)
			}
			
			fb := k8s.Group("/filebrowser")
			{
				fb.POST("/start", authMiddleware.Admin(), handlers_instance.K8s.StartFileBrowser)
				fb.POST("/stop", authMiddleware.Admin(), handlers_instance.K8s.StopFileBrowser)
			}
		}

		tickets := auth.Group("/tickets")
		{
			tickets.POST("", handlers_instance.Ticket.CreateTicket)
			tickets.GET("/my", handlers_instance.Ticket.GetMyTickets)
			tickets.GET("", authMiddleware.Admin(), handlers_instance.Ticket.GetAllTickets)
			tickets.PUT("/:id/status", authMiddleware.Admin(), handlers_instance.Ticket.UpdateTicketStatus)
		}

	}
}
