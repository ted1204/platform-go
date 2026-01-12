package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/api/middleware"
	"github.com/linskybing/platform-go/internal/api/routes"
	"github.com/linskybing/platform-go/internal/config"
	"github.com/linskybing/platform-go/internal/config/db"
	"github.com/linskybing/platform-go/internal/cron"
	"github.com/linskybing/platform-go/internal/domain/audit"
	"github.com/linskybing/platform-go/internal/domain/configfile"
	"github.com/linskybing/platform-go/internal/domain/form"
	"github.com/linskybing/platform-go/internal/domain/group"
	"github.com/linskybing/platform-go/internal/domain/image"
	"github.com/linskybing/platform-go/internal/domain/job"
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/internal/domain/resource"
	"github.com/linskybing/platform-go/internal/domain/user"
	"github.com/linskybing/platform-go/pkg/k8s"
)

func main() {
	// Load configuration from environment variables and .env file
	config.LoadConfig()

	// Initialize JWT signing key
	middleware.Init()

	// Initialize Kubernetes scheme (register API types)
	config.InitK8sConfig()

	// Initialize database connection
	db.Init()
	k8s.Init()

	// Auto migrate database schemas
	if err := db.DB.AutoMigrate(
		&user.User{},
		&group.Group{},
		&group.UserGroup{},
		&project.Project{},
		&configfile.ConfigFile{},
		&resource.Resource{},
		&job.Job{},
		&job.JobLog{},
		&job.JobCheckpoint{},
		&form.Form{},
		&form.FormMessage{},
		&audit.AuditLog{},
		&image.ContainerRepository{},
		&image.ContainerTag{},
		&image.ImageAllowList{},
		&image.ImageRequest{},
		&image.ClusterImageStatus{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize Docker cleanup CronJob
	if err := cron.CreateDockerCleanupCronJob(); err != nil {
		log.Printf("Warning: Failed to create Docker cleanup CronJob: %v", err)
		// Don't fail startup if CronJob creation fails
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.LoggingMiddleware())

	routes.RegisterRoutes(router, db.DB)

	port := ":" + config.ServerPort
	log.Printf("Starting API server on %s", port)
	if err := router.Run(port); err != nil {
		log.Fatalf("Failed to start: %v", err)
	}
}
