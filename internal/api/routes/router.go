package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/api/handlers"
	"github.com/linskybing/platform-go/internal/api/middleware"
	"github.com/linskybing/platform-go/internal/application"
	"github.com/linskybing/platform-go/internal/cron"
	"github.com/linskybing/platform-go/internal/domain/configfile"
	"github.com/linskybing/platform-go/internal/domain/group"
	"github.com/linskybing/platform-go/internal/repository"
)

func RegisterRoutes(r *gin.Engine) {
	// --- JWT-protected routes ---
	// Token status check endpoint (no group, but with JWT middleware)
	r.GET("/auth/status", middleware.JWTAuthMiddleware(), handlers.AuthStatusHandler)
	// init
	repos_instance := repository.New()
	services_instance := application.New(repos_instance)
	handlers_instance := handlers.New(services_instance, repos_instance, r)
	authMiddleware := middleware.NewAuth(repos_instance)

	// Start background tasks
	cron.StartCleanupTask(services_instance.Audit)

	// setup
	r.POST("/register", handlers_instance.User.Register)
	r.POST("/login", handlers_instance.User.Login)
	r.POST("/logout", handlers_instance.User.Logout)
	r.GET("/ws/exec", handlers.ExecWebSocketHandler)
	auth := r.Group("/")
	auth.Use(middleware.JWTAuthMiddleware())
	{
		auth.GET("/ws/monitoring/:namespace", handlers.WatchNamespaceHandler)
		auth.GET("/ws/jobs", handlers_instance.Job.StreamJobs)
		auth.GET("/ws/jobs/:id/logs", handlers_instance.Job.StreamJobLogs)
		// Image pull monitoring WebSocket
		auth.GET("/ws/image-pull/:job_id", func(c *gin.Context) {
			handlers.WatchImagePullHandler(c, services_instance.Image)
		})
		auth.GET("/ws/image-pull-all", func(c *gin.Context) {
			handlers.WatchMultiplePullJobsHandler(c, services_instance.Image)
		})

		imageReq := auth.Group("/image-requests")
		{
			imageReq.POST("", handlers_instance.Image.SubmitRequest)
			imageReq.GET("", authMiddleware.Admin(), handlers_instance.Image.ListRequests)
			imageReq.PUT("/:id/approve", authMiddleware.Admin(), handlers_instance.Image.ApproveRequest)
			imageReq.PUT("/:id/reject", authMiddleware.Admin(), handlers_instance.Image.RejectRequest)
		}

		images := auth.Group("/images")
		{
			images.GET("/allowed", handlers_instance.Image.ListAllowed)
			images.POST("/pull", authMiddleware.Admin(), handlers_instance.Image.PullImage)
			images.DELETE("/allowed/:id", authMiddleware.Admin(), handlers_instance.Image.DeleteAllowedImage)
		}

		userGroup := auth.Group("/user-group")
		{
			userGroup.GET("", authMiddleware.Admin(), handlers_instance.UserGroup.GetUserGroup)
			userGroup.GET("/by-group", handlers_instance.UserGroup.GetUserGroupsByGID)
			userGroup.GET("/by-user", handlers_instance.UserGroup.GetUserGroupsByUID)

			userGroup.POST("", authMiddleware.GroupAdmin(middleware.FromPayload(group.UserGroupInputDTO{})), handlers_instance.UserGroup.CreateUserGroup)
			userGroup.PUT("", authMiddleware.GroupAdmin(middleware.FromPayload(group.UserGroupInputDTO{})), handlers_instance.UserGroup.UpdateUserGroup)
			userGroup.DELETE("", authMiddleware.GroupAdmin(middleware.FromPayload(group.UserGroupDeleteDTO{})), handlers_instance.UserGroup.DeleteUserGroup)
		}

		projects := auth.Group("/projects")
		{
			projects.GET("", handlers_instance.Project.GetProjects)
			projects.GET("/by-user", handlers_instance.Project.GetProjectsByUser)
			projects.GET("/:id", handlers_instance.Project.GetProjectByID)
			projects.GET("/:id/config-files", handlers_instance.ConfigFile.ListConfigFilesByProjectIDHandler)
			projects.GET("/:id/resources", handlers_instance.Resource.ListResourcesByProjectID)
			projects.POST("", authMiddleware.Admin(), handlers_instance.Project.CreateProject)
			projects.PUT("/:id", authMiddleware.GroupManager(middleware.FromIDParam(repos_instance.Project.GetGroupIDByProjectID)), handlers_instance.Project.UpdateProject)
			projects.DELETE("/:id", authMiddleware.GroupAdmin(middleware.FromIDParam(repos_instance.Project.GetGroupIDByProjectID)), handlers_instance.Project.DeleteProject)
			projects.POST("/:id/gpu-requests", handlers_instance.GPURequest.CreateRequest)
			projects.GET("/:id/gpu-requests", handlers_instance.GPURequest.ListRequestsByProject)

			// Project-level image management (for project managers)
			projects.GET("/:id/images", authMiddleware.GroupMember(middleware.FromIDParam(repos_instance.Project.GetGroupIDByProjectID)), handlers_instance.Image.ListAllowed)
			projects.POST("/:id/images", authMiddleware.GroupManager(middleware.FromIDParam(repos_instance.Project.GetGroupIDByProjectID)), handlers_instance.Image.AddProjectImage)
			projects.DELETE("/:id/images/:image_id", authMiddleware.GroupManager(middleware.FromIDParam(repos_instance.Project.GetGroupIDByProjectID)), handlers_instance.Image.RemoveProjectImage)
		}

		audit := auth.Group("/audit/logs")
		{
			audit.GET("", handlers_instance.Audit.GetAuditLogs)
		}

		// Job management
		JobRoutes(auth, handlers_instance.Job)
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
			configFiles.POST("", authMiddleware.GroupManager(middleware.FromProjectIDInPayload(configfile.CreateConfigFileInput{})), handlers_instance.ConfigFile.CreateConfigFileHandler)
			configFiles.PUT("/:id", authMiddleware.GroupManager(middleware.FromIDParam(repos_instance.View.GetGroupIDByConfigFileID)), handlers_instance.ConfigFile.UpdateConfigFileHandler)
			configFiles.DELETE("/:id", authMiddleware.GroupManager(middleware.FromIDParam(repos_instance.View.GetGroupIDByConfigFileID)), handlers_instance.ConfigFile.DeleteConfigFileHandler)
		}
		admin := auth.Group("/admin")
		{
			admin.GET("/gpu-requests", authMiddleware.Admin(), handlers_instance.GPURequest.ListPendingRequests)
			admin.PUT("/gpu-requests/:id/status", authMiddleware.Admin(), handlers_instance.GPURequest.ProcessRequest)
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
			k8s.GET("/jobs", handlers_instance.K8s.ListJobs)
			k8s.GET("/jobs/:id", handlers_instance.K8s.GetJob)

			// PVC routes moved here
			pvc := k8s.Group("/pvc")
			{
				pvc.GET("/:namespace/:name", authMiddleware.Admin(), handlers_instance.K8s.GetPVC)
				pvc.GET("/list/:namespace", authMiddleware.Admin(), handlers_instance.K8s.ListPVCs)
				pvc.GET("/by-project/:id", authMiddleware.GroupMember(middleware.FromIDParam(repos_instance.Project.GetGroupIDByProjectID)), handlers_instance.K8s.ListPVCsByProject)
				pvc.POST("", authMiddleware.Admin(), handlers_instance.K8s.CreatePVC)
				pvc.POST("/project/:id", authMiddleware.GroupMember(middleware.FromIDParam(repos_instance.Project.GetGroupIDByProjectID)), handlers_instance.Project.CreateProjectPVC)
				pvc.PUT("/expand", authMiddleware.Admin(), handlers_instance.K8s.ExpandPVC)
				pvc.DELETE("/:namespace/:name", authMiddleware.Admin(), handlers_instance.K8s.DeletePVC)
			}

			// [NEW] Project Storage Management & Proxy
			// Base URL: /k8s/storage/projects
			projectStorage := k8s.Group("/storage/projects")
			{
				// 1. Admin Management (List & Create)
				// GET /k8s/storage/projects -> List all project storages
				projectStorage.GET("", authMiddleware.Admin(), handlers_instance.K8s.ListProjectStorages)
				projectStorage.GET("/my-storages", handlers_instance.K8s.GetUserProjectStorages)
				// POST /k8s/storage/projects -> Create new project storage (with labels)
				projectStorage.POST("", authMiddleware.Admin(), handlers_instance.K8s.CreateProjectStorage)
				projectStorage.POST("/:id/start",
					authMiddleware.GroupMember(middleware.FromIDParam(repos_instance.Project.GetGroupIDByProjectID)),
					handlers_instance.K8s.StartProjectFileBrowser)

				projectStorage.DELETE("/:id", authMiddleware.Admin(), handlers_instance.K8s.DeleteProjectStorage)

				projectStorage.DELETE("/:id/stop",
					authMiddleware.GroupMember(middleware.FromIDParam(repos_instance.Project.GetGroupIDByProjectID)),
					handlers_instance.K8s.StopProjectFileBrowser)
				// 2. FileBrowser Proxy (Access)
				// Use "id" (projectID) to verify membership via middleware
				// URL: /k8s/storage/projects/:id/proxy/*path
				projectStorage.Any("/:id/proxy/*path",
					authMiddleware.GroupMember(middleware.FromIDParam(repos_instance.Project.GetGroupIDByProjectID)),
					handlers_instance.K8s.ProjectStorageProxy,
				)
			}
			userStorageGroup := k8s.Group("/users")
			{
				userStorageGroup.GET("/:username/storage/status", authMiddleware.Admin(), handlers_instance.K8s.GetUserStorageStatus)
				userStorageGroup.POST("/:username/storage/init", authMiddleware.Admin(), handlers_instance.K8s.InitializeUserStorage)
				userStorageGroup.PUT("/:username/storage/expand", authMiddleware.Admin(), handlers_instance.K8s.ExpandUserStorage)
				userStorageGroup.DELETE("/:username/storage", authMiddleware.Admin(), handlers_instance.K8s.DeleteUserStorage)
				userStorageGroup.POST("/browse", handlers_instance.K8s.OpenMyDrive)
				userStorageGroup.DELETE("/browse", handlers_instance.K8s.StopMyDrive)
				userStorageGroup.Any("/proxy/*path", handlers_instance.K8s.UserStorageProxy)
			}
		}

		forms := auth.Group("/forms")
		{
			forms.POST("", handlers_instance.Form.CreateForm)
			forms.GET("/my", handlers_instance.Form.GetMyForms)
			forms.GET("", authMiddleware.Admin(), handlers_instance.Form.GetAllForms)
			forms.PUT("/:id/status", authMiddleware.Admin(), handlers_instance.Form.UpdateFormStatus)
			forms.GET("/:id/messages", handlers_instance.Form.ListMessages)
			forms.POST("/:id/messages", handlers_instance.Form.CreateMessage)
		}
	}
}
