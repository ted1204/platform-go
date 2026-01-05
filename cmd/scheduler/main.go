package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/linskybing/platform-go/internal/application"
	"github.com/linskybing/platform-go/internal/application/scheduler"
	"github.com/linskybing/platform-go/internal/config"
	"github.com/linskybing/platform-go/internal/config/db"
	"github.com/linskybing/platform-go/internal/domain/audit"
	"github.com/linskybing/platform-go/internal/domain/configfile"
	"github.com/linskybing/platform-go/internal/domain/form"
	"github.com/linskybing/platform-go/internal/domain/gpu"
	"github.com/linskybing/platform-go/internal/domain/group"
	"github.com/linskybing/platform-go/internal/domain/job"
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/internal/domain/resource"
	"github.com/linskybing/platform-go/internal/domain/user"
	"github.com/linskybing/platform-go/internal/repository"
	"github.com/linskybing/platform-go/internal/scheduler/executor"
	"github.com/linskybing/platform-go/pkg/k8s"
)

func main() {
	// Load configuration from environment variables and .env file
	config.LoadConfig()

	// Initialize database connection
	db.Init()

	// Initialize Kubernetes client
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
		&audit.AuditLog{},
		&gpu.GPURequest{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	repos := repository.New()
	imageService := application.NewImageService(repos.Image)
	registry := executor.NewExecutorRegistry()
	registry.Register(job.JobTypeNormal, executor.NewK8sExecutor(repos.Job, imageService))
	sched := scheduler.NewScheduler(registry, repos.Job)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
		<-sigChan
		log.Println("Shutdown signal")
		cancel()
	}()

	log.Printf("Starting scheduler (queue: %d)", sched.GetQueueSize())
	if err := sched.Start(ctx); err != nil {
		log.Printf("Scheduler error: %v", err)
	}
}
