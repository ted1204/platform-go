package job

// Repository defines data access interface for jobs
type Repository interface {
	Create(job *Job) error
	GetByID(id uint) (*Job, error)
	FindByID(id uint) (*Job, error) // Alias for GetByID
	GetByUserID(userID uint) ([]Job, error)
	FindByUserID(userID uint) ([]Job, error) // Alias
	GetByProjectID(projectID uint) ([]Job, error)
	FindByProjectID(projectID uint) ([]Job, error) // Alias
	GetByStatus(status string) ([]Job, error)
	GetQueuedJobs() ([]Job, error)
	FindAll() ([]Job, error)                             // Find all jobs
	FindLogs(jobID uint) ([]JobLog, error)               // Find logs for a job
	SaveLog(entry *JobLog) error                         // Append a log entry
	FindCheckpoints(jobID uint) ([]JobCheckpoint, error) // Find checkpoints for a job
	Update(job *Job) error
	Delete(id uint) error
	UpdateStatus(id uint, status string) error
	GetPreemptibleJobs() ([]Job, error)
}
