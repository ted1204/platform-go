package job

import "time"

// JobType defines the type of job execution
type JobType string

const (
	JobTypeNormal JobType = "normal" // Standard containerized job
	JobTypeMPI    JobType = "mpi"    // MPI distributed computing job
	JobTypeGPU    JobType = "gpu"    // GPU-accelerated job
)

// JobStatus represents the current state of a job
type JobStatus string

const (
	JobStatusQueued     JobStatus = "queued"     // Waiting in queue
	JobStatusScheduling JobStatus = "scheduling" // Being scheduled
	JobStatusRunning    JobStatus = "running"    // Currently executing
	JobStatusCompleted  JobStatus = "completed"  // Finished successfully
	JobStatusFailed     JobStatus = "failed"     // Execution failed
	JobStatusPreempted  JobStatus = "preempted"  // Terminated by higher priority
)

// Status aliases for backward compatibility
const (
	StatusPending    JobStatus = "pending"
	StatusRunning    JobStatus = "running"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
	StatusCancelled  JobStatus = "cancelled"
	StatusQueued     JobStatus = JobStatusQueued
	StatusScheduling JobStatus = JobStatusScheduling
	StatusPreempted  JobStatus = JobStatusPreempted
)

// Priority constants
const (
	PriorityHigh   = "high"
	PriorityLow    = "low"
	PriorityMedium = "medium"
)

// GPU type constants
const (
	GPUTypeDedicated = "dedicated"
	GPUTypeShared    = "shared"
)

// VolumeMount represents a volume mount configuration
type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
	ReadOnly  bool   `json:"read_only"`
}

// JobLog represents a job's log entry
type JobLog struct {
	ID      uint   `gorm:"primaryKey;column:id"`
	JobID   uint   `gorm:"not null;column:job_id"`
	Content string `gorm:"type:text"`
}

// JobCheckpoint represents a job's checkpoint data
type JobCheckpoint struct {
	ID            uint      `gorm:"primaryKey;column:id"`
	JobID         uint      `gorm:"not null;column:job_id"`
	CheckpointNum int       `gorm:"default:0"`
	Path          string    `gorm:"type:text"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
}

// Job represents a batch job execution request
type Job struct {
	ID                 uint       `gorm:"primaryKey;column:id"`
	UserID             uint       `gorm:"not null;column:user_id"`
	ProjectID          *uint      `gorm:"column:project_id"`
	Name               string     `gorm:"size:100;not null"`
	Namespace          string     `gorm:"size:100;not null"`
	Image              string     `gorm:"size:255;not null"`
	Status             string     `gorm:"size:50;default:'pending'"`
	JobType            JobType    `gorm:"size:20;default:'normal'"`
	Priority           string     `gorm:"size:20;default:'low'"`
	K8sJobName         string     `gorm:"size:100;not null"`
	Command            string     `gorm:"type:text"`
	Args               string     `gorm:"type:text"`
	WorkingDir         string     `gorm:"size:255"`
	EnvVars            string     `gorm:"type:text"`
	GPUCount           int        `gorm:"default:0"`
	GPUType            string     `gorm:"size:50"`
	CPURequest         string     `gorm:"size:50"`
	MemoryRequest      string     `gorm:"size:50"`
	MPIProcesses       int        `gorm:"default:0"`
	OutputPath         string     `gorm:"type:text"`
	CheckpointPath     string     `gorm:"type:text"`
	LogPath            string     `gorm:"type:text"`
	EnableCheckpoint   bool       `gorm:"default:false"`
	CheckpointInterval int        `gorm:"default:0"`
	Volumes            string     `gorm:"type:text"`
	RestartCount       int        `gorm:"default:0"`
	ExitCode           *int       `gorm:"column:exit_code"`
	ErrorMessage       string     `gorm:"type:text"`
	CreatedAt          time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;autoUpdateTime"`
	StartedAt          *time.Time `gorm:"column:started_at"`
	CompletedAt        *time.Time `gorm:"column:completed_at"`
}

// TableName specifies the database table name
func (Job) TableName() string {
	return "jobs"
}

// IsMPI checks if this is an MPI job
func (j *Job) IsMPI() bool {
	return j.JobType == JobTypeMPI
}

// RequiresGPU checks if this job requires GPU resources
func (j *Job) RequiresGPU() bool {
	return j.JobType == JobTypeGPU || j.GPUCount > 0
}

// UsesMPS checks if this job uses MPS GPU sharing
func (j *Job) UsesMPS() bool {
	return j.GPUType == "shared"
}

// IsPreemptible checks if this job can be preempted
func (j *Job) IsPreemptible() bool {
	return j.Priority != "high"
}
