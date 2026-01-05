package job

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/linskybing/platform-go/internal/domain/job"
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/internal/domain/user"
)

// Service handles job-related business logic
type Service struct {
	jobRepo     job.Repository
	userRepo    UserFinder
	projectRepo ProjectFinder
}

// UserFinder provides minimal user lookup needed for jobs.
type UserFinder interface {
	GetUserRawByID(id uint) (user.User, error)
}

// ProjectFinder provides minimal project lookup needed for jobs.
type ProjectFinder interface {
	GetProjectByID(id uint) (project.Project, error)
}

// NewService creates a new job service
func NewService(
	jobRepo job.Repository,
	userRepo UserFinder,
	projectRepo ProjectFinder,
) *Service {
	return &Service{
		jobRepo:     jobRepo,
		userRepo:    userRepo,
		projectRepo: projectRepo,
	}
}

// CreateJobRequest represents a job creation request
type CreateJobRequest struct {
	Name               string            `json:"name" binding:"required"`
	Namespace          string            `json:"namespace" binding:"required"`
	JobType            string            `json:"job_type"`
	Image              string            `json:"image" binding:"required"`
	Command            []string          `json:"command"`
	Args               []string          `json:"args"`
	WorkingDir         string            `json:"working_dir"`
	EnvVars            map[string]string `json:"env_vars"`
	GPUCount           int               `json:"gpu_count"`
	GPUType            string            `json:"gpu_type"`
	CPURequest         string            `json:"cpu_request"`
	MemoryRequest      string            `json:"memory_request"`
	MPIProcesses       int               `json:"mpi_processes"`
	OutputPath         string            `json:"output_path"`
	CheckpointPath     string            `json:"checkpoint_path"`
	EnableCheckpoint   bool              `json:"enable_checkpoint"`
	CheckpointInterval int               `json:"checkpoint_interval"`
	Volumes            []job.VolumeMount `json:"volumes"`
}

// CreateJob creates a new job in pending state
func (s *Service) CreateJob(ctx context.Context, userID uint, req CreateJobRequest) (*job.Job, error) {
	_, err := s.userRepo.GetUserRawByID(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	var projectID *uint
	if strings.Contains(req.Namespace, "-") {
		parts := strings.Split(req.Namespace, "-")
		if len(parts) >= 1 {
			var pid uint
			if _, err := fmt.Sscanf(parts[0], "%d", &pid); err == nil {
				projectID = &pid
			}
		}
	}

	if req.GPUCount > 0 && projectID != nil {
		proj, err := s.projectRepo.GetProjectByID(*projectID)
		if err != nil {
			return nil, fmt.Errorf("project not found: %w", err)
		}

		if !s.isGPUAccessAllowed(&proj, req.GPUType) {
			return nil, fmt.Errorf("GPU access type '%s' not allowed for this project", req.GPUType)
		}

		currentUsage, err := s.calculateProjectGPUUsage(ctx, *projectID)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate GPU usage: %w", err)
		}

		requestedUnits := req.GPUCount
		if req.GPUType == job.GPUTypeDedicated {
			requestedUnits = req.GPUCount * 10
		}

		if currentUsage+requestedUnits > proj.GPUQuota {
			return nil, fmt.Errorf("GPU quota exceeded: current=%d, requested=%d, quota=%d",
				currentUsage, requestedUnits, proj.GPUQuota)
		}
	}

	if req.JobType == "" {
		req.JobType = string(job.JobTypeNormal)
	}

	if req.EnableCheckpoint && req.CheckpointInterval == 0 {
		req.CheckpointInterval = 300
	}

	commandJSON, _ := json.Marshal(req.Command)
	argsJSON, _ := json.Marshal(req.Args)
	envVarsJSON, _ := json.Marshal(req.EnvVars)
	volumesJSON, _ := json.Marshal(req.Volumes)

	newJob := &job.Job{
		UserID:             userID,
		ProjectID:          projectID,
		Name:               req.Name,
		Namespace:          req.Namespace,
		JobType:            job.JobType(req.JobType),
		Image:              req.Image,
		Command:            string(commandJSON),
		Args:               string(argsJSON),
		WorkingDir:         req.WorkingDir,
		EnvVars:            string(envVarsJSON),
		GPUCount:           req.GPUCount,
		GPUType:            req.GPUType,
		CPURequest:         req.CPURequest,
		MemoryRequest:      req.MemoryRequest,
		MPIProcesses:       req.MPIProcesses,
		OutputPath:         req.OutputPath,
		CheckpointPath:     req.CheckpointPath,
		K8sJobName:         req.Name,
		Priority:           job.PriorityLow,
		Status:             string(job.JobStatusQueued),
		EnableCheckpoint:   req.EnableCheckpoint,
		CheckpointInterval: req.CheckpointInterval,
		Volumes:            string(volumesJSON),
	}

	if err := s.jobRepo.Create(newJob); err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	newJob.OutputPath = s.generateOutputPath(newJob.ID, req.OutputPath)
	newJob.CheckpointPath = s.generateCheckpointPath(newJob.ID, req.CheckpointPath)
	newJob.LogPath = s.generateLogPath(newJob.ID)

	if err := s.jobRepo.Update(newJob); err != nil {
		log.Printf("Warning: Failed to update job paths: %v", err)
	}

	log.Printf("Created job %d (%s) in pending state", newJob.ID, newJob.Name)
	return newJob, nil
}

// ListJobs returns jobs for a user or all jobs for admin
func (s *Service) ListJobs(ctx context.Context, userID uint, isAdmin bool) ([]job.Job, error) {
	if isAdmin {
		return s.jobRepo.FindAll()
	}

	return s.jobRepo.FindByUserID(userID)
}

// GetJob returns a specific job
func (s *Service) GetJob(ctx context.Context, jobID uint) (*job.Job, error) {
	return s.jobRepo.FindByID(jobID)
}

// CancelJob cancels a running or pending job
func (s *Service) CancelJob(ctx context.Context, jobID uint) error {
	j, err := s.jobRepo.FindByID(jobID)
	if err != nil {
		return fmt.Errorf("job not found: %w", err)
	}

	if j.Status == string(job.StatusCompleted) || j.Status == string(job.StatusFailed) || j.Status == string(job.StatusCancelled) {
		return fmt.Errorf("cannot cancel job in status: %s", j.Status)
	}

	j.Status = string(job.StatusCancelled)
	now := time.Now()
	j.CompletedAt = &now

	return s.jobRepo.Update(j)
}

// RestartJob restarts a job from checkpoint
func (s *Service) RestartJob(ctx context.Context, jobID uint, checkpointID *uint) error {
	j, err := s.jobRepo.FindByID(jobID)
	if err != nil {
		return fmt.Errorf("job not found: %w", err)
	}

	if j.Status == string(job.StatusRunning) {
		return fmt.Errorf("cannot restart running job")
	}

	j.Status = string(job.JobStatusQueued)
	j.RestartCount++
	j.CompletedAt = nil
	j.StartedAt = nil
	j.ExitCode = nil
	j.ErrorMessage = ""

	return s.jobRepo.Update(j)
}

// GetJobLogs returns logs for a job
func (s *Service) GetJobLogs(ctx context.Context, jobID uint, limit, offset int) ([]job.JobLog, error) {
	return s.jobRepo.FindLogs(jobID)
}

// GetJobCheckpoints returns checkpoints for a job
func (s *Service) GetJobCheckpoints(ctx context.Context, jobID uint) ([]job.JobCheckpoint, error) {
	return s.jobRepo.FindCheckpoints(jobID)
}

// isGPUAccessAllowed checks if GPU type is allowed for the project
func (s *Service) isGPUAccessAllowed(proj *project.Project, gpuType string) bool {
	if proj.GPUAccess == "" {
		return false
	}

	allowedTypes := strings.Split(proj.GPUAccess, ",")
	for _, t := range allowedTypes {
		if strings.TrimSpace(t) == gpuType {
			return true
		}
	}

	return false
}

// calculateProjectGPUUsage calculates current GPU usage for a project
func (s *Service) calculateProjectGPUUsage(ctx context.Context, projectID uint) (int, error) {
	jobs, err := s.jobRepo.FindByProjectID(projectID)
	if err != nil {
		return 0, err
	}

	usage := 0
	for _, j := range jobs {
		if j.Status == string(job.StatusRunning) || j.Status == string(job.StatusPending) {
			units := j.GPUCount
			if j.GPUType == job.GPUTypeDedicated {
				units = j.GPUCount * 10
			}

			usage += units
		}
	}

	return usage, nil
}

// generateOutputPath generates output path for a job
func (s *Service) generateOutputPath(jobID uint, requestedPath string) string {
	if requestedPath != "" {
		return requestedPath
	}

	return fmt.Sprintf("/personal-drive/jobs/%d/output", jobID)
}

// generateCheckpointPath generates checkpoint path for a job
func (s *Service) generateCheckpointPath(jobID uint, requestedPath string) string {
	if requestedPath != "" {
		return requestedPath
	}

	return fmt.Sprintf("/personal-drive/jobs/%d/checkpoints", jobID)
}

// generateLogPath generates log path for a job
func (s *Service) generateLogPath(jobID uint) string {
	return fmt.Sprintf("/personal-drive/jobs/%d/logs", jobID)
}
