package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/linskybing/platform-go/internal/domain/job"
	"github.com/linskybing/platform-go/internal/scheduler/executor"
)

// MockJobExecutor for testing
type MockJobExecutor struct {
	executeErr bool
}

func (m *MockJobExecutor) Execute(ctx context.Context, j *job.Job) error {
	if m.executeErr {
		return errors.New("mock execute error")
	}
	return nil
}

func (m *MockJobExecutor) Cancel(ctx context.Context, jobID uint) error {
	return nil
}

func (m *MockJobExecutor) GetStatus(ctx context.Context, jobID uint) (job.JobStatus, error) {
	return job.StatusRunning, nil
}

func (m *MockJobExecutor) GetLogs(ctx context.Context, jobID uint) (string, error) {
	return "", nil
}

func (m *MockJobExecutor) SupportsType(jobType job.JobType) bool {
	return true
}

func TestNewScheduler(t *testing.T) {
	registry := executor.NewExecutorRegistry()
	sched := NewScheduler(registry, nil)

	if sched == nil {
		t.Fatal("expected non-nil scheduler")
	}
	if sched.jobQueue == nil {
		t.Fatal("expected jobQueue to be initialized")
	}
	if sched.running {
		t.Fatal("expected scheduler not running initially")
	}
}

func TestEnqueueJob(t *testing.T) {
	registry := executor.NewExecutorRegistry()
	sched := NewScheduler(registry, nil)

	j1 := &job.Job{ID: 1, Priority: "low"}
	j2 := &job.Job{ID: 2, Priority: "high"}

	sched.EnqueueJob(j1)
	sched.EnqueueJob(j2)

	if sched.GetQueueSize() != 2 {
		t.Fatalf("expected 2 jobs, got %d", sched.GetQueueSize())
	}
}

func TestGetQueueSize(t *testing.T) {
	registry := executor.NewExecutorRegistry()
	sched := NewScheduler(registry, nil)

	if sched.GetQueueSize() != 0 {
		t.Fatal("expected empty queue initially")
	}

	sched.EnqueueJob(&job.Job{ID: 1, Priority: "low"})
	if sched.GetQueueSize() != 1 {
		t.Fatal("expected queue size 1")
	}

	sched.EnqueueJob(&job.Job{ID: 2, Priority: "high"})
	if sched.GetQueueSize() != 2 {
		t.Fatal("expected queue size 2")
	}
}

func TestIsRunning(t *testing.T) {
	registry := executor.NewExecutorRegistry()
	sched := NewScheduler(registry, nil)

	if sched.IsRunning() {
		t.Fatal("expected scheduler not running initially")
	}
}

func TestStartAndStop(t *testing.T) {
	registry := executor.NewExecutorRegistry()
	sched := NewScheduler(registry, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := sched.Start(ctx)
	if err != context.DeadlineExceeded {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
	if sched.IsRunning() {
		t.Fatal("expected scheduler stopped")
	}
}

func TestProcessQueueWithEmptyQueue(t *testing.T) {
	registry := executor.NewExecutorRegistry()
	sched := NewScheduler(registry, nil)

	ctx := context.Background()
	sched.processQueue(ctx)

	if sched.GetQueueSize() != 0 {
		t.Fatal("expected queue to remain empty")
	}
}

func TestProcessQueueWithJob(t *testing.T) {
	registry := executor.NewExecutorRegistry()
	mockExec := &MockJobExecutor{executeErr: false}
	registry.Register("test", mockExec)

	sched := NewScheduler(registry, nil)

	j := &job.Job{ID: 1, JobType: "test", Priority: "medium"}
	sched.EnqueueJob(j)

	ctx := context.Background()
	sched.processQueue(ctx)

	if sched.GetQueueSize() != 0 {
		t.Fatal("expected queue to be empty after processing")
	}
	if j.Status != string(job.StatusRunning) {
		t.Fatalf("expected job status %s, got %s", job.StatusRunning, j.Status)
	}
}

func TestProcessQueueWithExecutorError(t *testing.T) {
	registry := executor.NewExecutorRegistry()
	mockExec := &MockJobExecutor{executeErr: true}
	registry.Register("test", mockExec)

	sched := NewScheduler(registry, nil)

	j := &job.Job{ID: 1, JobType: "test", Priority: "low"}
	sched.EnqueueJob(j)

	ctx := context.Background()
	sched.processQueue(ctx)

	if sched.GetQueueSize() != 0 {
		t.Fatal("expected queue to be empty after processing")
	}
	if j.Status != string(job.StatusFailed) {
		t.Fatalf("expected job status %s, got %s", job.StatusFailed, j.Status)
	}
}

func TestProcessQueueWithUnregisteredJobType(t *testing.T) {
	registry := executor.NewExecutorRegistry()
	sched := NewScheduler(registry, nil)

	j := &job.Job{ID: 1, JobType: "unknown", Priority: "high"}
	sched.EnqueueJob(j)

	ctx := context.Background()
	sched.processQueue(ctx)

	if sched.GetQueueSize() != 0 {
		t.Fatal("expected queue to be empty after processing")
	}
	// Job should remain unchanged since no executor exists
	if j.Status != "" {
		t.Fatalf("expected job status to remain empty, got %s", j.Status)
	}
}

func TestMultipleJobProcessing(t *testing.T) {
	registry := executor.NewExecutorRegistry()
	mockExec := &MockJobExecutor{executeErr: false}
	registry.Register("test", mockExec)

	sched := NewScheduler(registry, nil)

	jobs := []*job.Job{
		{ID: 1, JobType: "test", Priority: "low"},
		{ID: 2, JobType: "test", Priority: "high"},
		{ID: 3, JobType: "test", Priority: "medium"},
	}

	for _, job := range jobs {
		sched.EnqueueJob(job)
	}

	if sched.GetQueueSize() != 3 {
		t.Fatalf("expected 3 jobs in queue, got %d", sched.GetQueueSize())
	}

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		sched.processQueue(ctx)
	}

	if sched.GetQueueSize() != 0 {
		t.Fatalf("expected empty queue, got %d", sched.GetQueueSize())
	}

	for _, j := range jobs {
		if j.Status != string(job.StatusRunning) {
			t.Fatalf("expected all jobs to have status %s", job.StatusRunning)
		}
	}
}

func TestEnqueueAndProcessPriority(t *testing.T) {
	registry := executor.NewExecutorRegistry()
	mockExec := &MockJobExecutor{executeErr: false}
	registry.Register("test", mockExec)

	sched := NewScheduler(registry, nil)

	// Enqueue in non-priority order
	jobs := []*job.Job{
		{ID: 1, JobType: "test", Priority: "low"},
		{ID: 2, JobType: "test", Priority: "high"},
		{ID: 3, JobType: "test", Priority: "medium"},
	}

	for _, j := range jobs {
		sched.EnqueueJob(j)
	}

	ctx := context.Background()

	// First pop should be high priority
	sched.processQueue(ctx)
	if jobs[1].Status != string(job.StatusRunning) {
		t.Fatal("expected high priority job to be processed first")
	}

	// Second pop should be medium priority
	sched.processQueue(ctx)
	if jobs[2].Status != string(job.StatusRunning) {
		t.Fatal("expected medium priority job to be processed second")
	}

	// Third pop should be low priority
	sched.processQueue(ctx)
	if jobs[0].Status != string(job.StatusRunning) {
		t.Fatal("expected low priority job to be processed last")
	}
}

func TestSchedulerContextCancellation(t *testing.T) {
	registry := executor.NewExecutorRegistry()
	sched := NewScheduler(registry, nil)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- sched.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for scheduler to stop")
	}
}
