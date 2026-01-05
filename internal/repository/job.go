package repository

import (
	"github.com/linskybing/platform-go/internal/config/db"
	"github.com/linskybing/platform-go/internal/domain/job"
)

// JobRepo matches the domain job repository contract.
type JobRepo interface {
	job.Repository
}

type DBJobRepo struct{}

func (r *DBJobRepo) Create(j *job.Job) error {
	return db.DB.Create(j).Error
}

func (r *DBJobRepo) FindAll() ([]job.Job, error) {
	var jobs []job.Job
	err := db.DB.Find(&jobs).Error
	return jobs, err
}

func (r *DBJobRepo) FindByID(id uint) (*job.Job, error) {
	var j job.Job
	err := db.DB.First(&j, id).Error
	return &j, err
}

func (r *DBJobRepo) GetByID(id uint) (*job.Job, error) {
	return r.FindByID(id)
}

func (r *DBJobRepo) FindByUserID(userID uint) ([]job.Job, error) {
	var jobs []job.Job
	err := db.DB.Where("user_id = ?", userID).Find(&jobs).Error
	return jobs, err
}

func (r *DBJobRepo) GetByUserID(userID uint) ([]job.Job, error) {
	return r.FindByUserID(userID)
}

func (r *DBJobRepo) FindByProjectID(projectID uint) ([]job.Job, error) {
	var jobs []job.Job
	err := db.DB.Where("project_id = ?", projectID).Find(&jobs).Error
	return jobs, err
}

func (r *DBJobRepo) GetByProjectID(projectID uint) ([]job.Job, error) {
	return r.FindByProjectID(projectID)
}

func (r *DBJobRepo) FindByNamespace(namespace string) ([]job.Job, error) {
	var jobs []job.Job
	err := db.DB.Where("namespace = ?", namespace).Find(&jobs).Error
	return jobs, err
}

func (r *DBJobRepo) GetByStatus(status string) ([]job.Job, error) {
	var jobs []job.Job
	err := db.DB.Where("status = ?", status).Find(&jobs).Error
	return jobs, err
}

func (r *DBJobRepo) GetQueuedJobs() ([]job.Job, error) {
	var jobs []job.Job
	err := db.DB.Where("status IN ?", []string{string(job.StatusPending), string(job.JobStatusQueued)}).
		Find(&jobs).Error
	return jobs, err
}

func (r *DBJobRepo) FindLogs(jobID uint) ([]job.JobLog, error) {
	var logs []job.JobLog
	err := db.DB.Where("job_id = ?", jobID).Order("id ASC").Find(&logs).Error
	return logs, err
}

func (r *DBJobRepo) SaveLog(entry *job.JobLog) error {
	return db.DB.Create(entry).Error
}

func (r *DBJobRepo) FindCheckpoints(jobID uint) ([]job.JobCheckpoint, error) {
	var checkpoints []job.JobCheckpoint
	err := db.DB.Where("job_id = ?", jobID).Order("checkpoint_num ASC").Find(&checkpoints).Error
	return checkpoints, err
}

func (r *DBJobRepo) Update(j *job.Job) error {
	return db.DB.Save(j).Error
}

func (r *DBJobRepo) Delete(id uint) error {
	return db.DB.Delete(&job.Job{}, id).Error
}

func (r *DBJobRepo) UpdateStatus(id uint, status string) error {
	return db.DB.Model(&job.Job{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": status}).Error
}

func (r *DBJobRepo) GetPreemptibleJobs() ([]job.Job, error) {
	var jobs []job.Job
	err := db.DB.Where("priority <> ?", job.PriorityHigh).Find(&jobs).Error
	return jobs, err
}
