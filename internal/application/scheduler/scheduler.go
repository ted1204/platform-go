package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/linskybing/platform-go/internal/domain/job"
	"github.com/linskybing/platform-go/internal/scheduler/executor"
	"github.com/linskybing/platform-go/internal/scheduler/queue"
)

// Scheduler manages job execution with priority queue
type Scheduler struct {
	jobQueue *queue.JobQueue
	registry *executor.ExecutorRegistry
	running  bool
	jobRepo  job.Repository
	enqueued map[uint]bool
}

// NewScheduler creates a new scheduler
func NewScheduler(registry *executor.ExecutorRegistry, jobRepo job.Repository) *Scheduler {
	return &Scheduler{
		jobQueue: queue.NewJobQueue(),
		registry: registry,
		running:  false,
		jobRepo:  jobRepo,
		enqueued: make(map[uint]bool),
	}
}

// Start begins scheduling
func (s *Scheduler) Start(ctx context.Context) error {
	s.running = true
	log.Println("Scheduler started")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.running = false
			log.Println("Scheduler stopped")
			return ctx.Err()
		case <-ticker.C:
			s.syncQueued()
			s.processQueue(ctx)
		}
	}
}

// EnqueueJob adds a job to queue
func (s *Scheduler) EnqueueJob(j *job.Job) {
	s.jobQueue.Push(j)
	if j != nil {
		s.enqueued[j.ID] = true
	}
}

// processQueue processes pending jobs
func (s *Scheduler) processQueue(ctx context.Context) {
	j := s.jobQueue.Pop()
	if j == nil {
		return
	}

	if s.jobRepo != nil {
		j.Status = string(job.JobStatusScheduling)
		_ = s.jobRepo.Update(j)
	}

	err := s.registry.Execute(ctx, j)
	if err == executor.ErrExecutorNotFound {
		// Don't change status for unregistered job types
		log.Printf("Job executor not found for type: %s", j.JobType)
		return
	}
	if err != nil {
		log.Printf("Job error: %v", err)
		j.Status = string(job.JobStatusFailed)
		if s.jobRepo != nil {
			_ = s.jobRepo.Update(j)
		}
	} else {
		j.Status = string(job.JobStatusRunning)
		if s.jobRepo != nil {
			_ = s.jobRepo.Update(j)
		}
	}
}

// IsRunning returns if active
func (s *Scheduler) IsRunning() bool {
	return s.running
}

// GetQueueSize returns pending count
func (s *Scheduler) GetQueueSize() int {
	return s.jobQueue.Len()
}

// syncQueued fetches queued jobs from the repository and enqueues them.
func (s *Scheduler) syncQueued() {
	if s.jobRepo == nil {
		return
	}
	jbs, err := s.jobRepo.GetQueuedJobs()
	if err != nil {
		log.Printf("syncQueued error: %v", err)
		return
	}
	for i := range jbs {
		if s.enqueued[jbs[i].ID] {
			continue
		}
		s.EnqueueJob(&jbs[i])
	}
}
