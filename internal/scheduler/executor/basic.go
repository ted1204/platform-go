package executor

import (
	"context"

	"github.com/linskybing/platform-go/internal/domain/job"
)

// BasicExecutor executes jobs by updating their status; placeholder for real k8s execution.
type BasicExecutor struct {
	jobRepo job.Repository
}

// NewBasicExecutor creates a basic executor.
func NewBasicExecutor(jobRepo job.Repository) *BasicExecutor {
	return &BasicExecutor{jobRepo: jobRepo}
}

func (e *BasicExecutor) Execute(ctx context.Context, j *job.Job) error {
	// Placeholder: in real impl, create k8s Job and watch.
	if e.jobRepo != nil {
		j.Status = string(job.JobStatusRunning)
		_ = e.jobRepo.Update(j)
	}
	return nil
}

func (e *BasicExecutor) Cancel(ctx context.Context, jobID uint) error {
	if e.jobRepo != nil {
		if obj, err := e.jobRepo.FindByID(jobID); err == nil {
			obj.Status = string(job.StatusCancelled)
			return e.jobRepo.Update(obj)
		}
	}
	return nil
}

func (e *BasicExecutor) GetStatus(ctx context.Context, jobID uint) (job.JobStatus, error) {
	if e.jobRepo == nil {
		return job.StatusPending, nil
	}
	obj, err := e.jobRepo.FindByID(jobID)
	if err != nil {
		return job.StatusPending, err
	}
	return job.JobStatus(obj.Status), nil
}

func (e *BasicExecutor) GetLogs(ctx context.Context, jobID uint) (string, error) {
	// Not implemented: return empty
	return "", nil
}

func (e *BasicExecutor) SupportsType(jobType job.JobType) bool {
	return true
}
