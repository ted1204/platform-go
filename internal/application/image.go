package application

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	cfg "github.com/linskybing/platform-go/internal/config"
	"github.com/linskybing/platform-go/internal/domain/image"
	"github.com/linskybing/platform-go/internal/repository"
	"github.com/linskybing/platform-go/pkg/k8s"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// boolPtr returns a pointer to a boolean value
func boolPtr(b bool) *bool {
	return &b
}

// PullJobStatus represents the status of an image pull job
type PullJobStatus struct {
	JobID     string    `json:"job_id"`
	ImageName string    `json:"image_name"`
	ImageTag  string    `json:"image_tag"`
	Status    string    `json:"status"`   // pending, pulling, completed, failed
	Progress  int       `json:"progress"` // 0-100
	Message   string    `json:"message"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PullJobTracker holds active pull jobs and their status
type PullJobTracker struct {
	mu         sync.RWMutex
	jobs       map[string]*PullJobStatus
	chans      map[string][]chan *PullJobStatus
	failedJobs []*PullJobStatus // Keep history of failed jobs
	maxHistory int
}

var pullTracker = &PullJobTracker{
	jobs:       make(map[string]*PullJobStatus),
	chans:      make(map[string][]chan *PullJobStatus),
	failedJobs: make([]*PullJobStatus, 0),
	maxHistory: 50,
}

func (pt *PullJobTracker) AddJob(jobID, imageName, imageTag string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.jobs[jobID] = &PullJobStatus{
		JobID:     jobID,
		ImageName: imageName,
		ImageTag:  imageTag,
		Status:    "pending",
		Progress:  0,
		UpdatedAt: time.Now(),
	}
}

func (pt *PullJobTracker) GetJob(jobID string) *PullJobStatus {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.jobs[jobID]
}

func (pt *PullJobTracker) UpdateJob(jobID string, status string, progress int, message string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if job, ok := pt.jobs[jobID]; ok {
		job.Status = status
		job.Progress = progress
		job.Message = message
		job.UpdatedAt = time.Now()

		// Notify all subscribers
		if chans, ok := pt.chans[jobID]; ok {
			for _, ch := range chans {
				select {
				case ch <- job:
				default:
				}
			}
		}
	}
}

func (pt *PullJobTracker) Subscribe(jobID string) <-chan *PullJobStatus {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	ch := make(chan *PullJobStatus, 10)
	pt.chans[jobID] = append(pt.chans[jobID], ch)
	return ch
}

func (pt *PullJobTracker) RemoveJob(jobID string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Save failed job to history before removing
	if job, ok := pt.jobs[jobID]; ok && job.Status == "failed" {
		pt.failedJobs = append(pt.failedJobs, job)
		// Keep only recent failed jobs
		if len(pt.failedJobs) > pt.maxHistory {
			pt.failedJobs = pt.failedJobs[len(pt.failedJobs)-pt.maxHistory:]
		}
	}

	delete(pt.jobs, jobID)
}

func (pt *PullJobTracker) GetFailedJobs(limit int) []*PullJobStatus {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if limit <= 0 || limit > len(pt.failedJobs) {
		limit = len(pt.failedJobs)
	}

	// Return most recent first
	result := make([]*PullJobStatus, 0, limit)
	for i := len(pt.failedJobs) - 1; i >= len(pt.failedJobs)-limit && i >= 0; i-- {
		result = append(result, pt.failedJobs[i])
	}
	return result
}

func (pt *PullJobTracker) GetActiveJobs() []*PullJobStatus {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	result := make([]*PullJobStatus, 0, len(pt.jobs))
	for _, job := range pt.jobs {
		result = append(result, job)
	}
	return result
}

type ImageService struct {
	repo repository.ImageRepo
}

func NewImageService(repo repository.ImageRepo) *ImageService {
	return &ImageService{repo: repo}
}

func (s *ImageService) SubmitRequest(userID uint, name, tag string, projectID *uint) (*image.ImageRequest, error) {
	req := &image.ImageRequest{
		UserID:    userID,
		Name:      name,
		Tag:       tag,
		ProjectID: projectID,
		Status:    "pending",
	}
	if warn := s.validateNameAndTag(name, tag); warn != "" {
		log.Printf("[image-validate] warning: %s", warn)
		req.Note = warn
	}
	return req, s.repo.CreateRequest(req)
}

func (s *ImageService) ListRequests(status string) ([]image.ImageRequest, error) {
	return s.repo.ListRequests(status)
}

func (s *ImageService) ApproveRequest(id uint, note string, isGlobal bool) (*image.ImageRequest, error) {
	req, err := s.repo.FindRequestByID(id)
	if err != nil {
		return nil, err
	}
	if warn := s.validateNameAndTag(req.Name, req.Tag); warn != "" && req.Note == "" {
		log.Printf("[image-validate] warning on approve: %s", warn)
		req.Note = warn
	}
	req.Status = "approved"
	req.Note = note
	if err := s.repo.UpdateRequest(req); err != nil {
		return nil, err
	}

	// Add to allowed images
	allowedImg := &image.AllowedImage{
		Name:      req.Name,
		Tag:       req.Tag,
		ProjectID: req.ProjectID,
		IsGlobal:  isGlobal,
		CreatedBy: req.UserID,
	}
	_ = s.repo.CreateAllowed(allowedImg)
	return req, nil
}

func (s *ImageService) RejectRequest(id uint, note string) (*image.ImageRequest, error) {
	req, err := s.repo.FindRequestByID(id)
	if err != nil {
		return nil, err
	}
	req.Status = "rejected"
	req.Note = note
	if err := s.repo.UpdateRequest(req); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *ImageService) ListAllowed() ([]image.AllowedImage, error) {
	return s.repo.ListAllowed()
}

// ListAllowedForProject returns global + project-specific images
func (s *ImageService) ListAllowedForProject(projectID uint) ([]image.AllowedImage, error) {
	return s.repo.FindAllowedImagesForProject(projectID)
}

// AddProjectImage directly adds an image to a project (for project managers)
func (s *ImageService) AddProjectImage(userID, projectID uint, name, tag string) (*image.AllowedImage, error) {
	if warn := s.validateNameAndTag(name, tag); warn != "" {
		return nil, fmt.Errorf("invalid image format: %s", warn)
	}

	img := &image.AllowedImage{
		Name:      name,
		Tag:       tag,
		ProjectID: &projectID,
		IsGlobal:  false,
		CreatedBy: userID,
	}

	if err := s.repo.CreateAllowed(img); err != nil {
		return nil, err
	}
	return img, nil
}

// ValidateImageForProject checks if image is allowed for a project
func (s *ImageService) ValidateImageForProject(name, tag string, projectID uint) (bool, error) {
	return s.repo.ValidateImageForProject(name, tag, projectID)
}

// GetAllowedImage retrieves an allowed image by name, tag, and project
func (s *ImageService) GetAllowedImage(name, tag string, projectID uint) (*image.AllowedImage, error) {
	return s.repo.FindAllowedImage(name, tag, projectID)
}

func (s *ImageService) PullImage(name, tag string) error {
	if warn := s.validateNameAndTag(name, tag); warn != "" {
		log.Printf("[image-validate] warning on pull: %s", warn)
	}

	fullImage := fmt.Sprintf("%s:%s", name, tag)
	ttl := int32(300) // Clean up job 5 minutes after completion

	// Create a Job to pull the image
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "image-puller-",
			Namespace:    "default", // Using default namespace for admin tasks
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:            "puller",
							Image:           fullImage,
							ImagePullPolicy: corev1.PullAlways, // Force pull
							// We try to run a harmless command.
							// If the image lacks sh, it will fail, but the image will be pulled.
							Command: []string{"/bin/sh", "-c", "echo Image pulled successfully"},
						},
					},
				},
			},
		},
	}

	_, err := k8s.Clientset.BatchV1().Jobs("default").Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create image pull job: %v", err)
		return err
	}

	log.Printf("Created job to pull image: %s", fullImage)
	return nil
}

// PullImageAsync creates a Kubernetes Job to pull an image, push to Harbor, and returns the job ID
// The actual pull happens asynchronously, with status updates sent via WebSocket
func (s *ImageService) PullImageAsync(name, tag string) (string, error) {
	if warn := s.validateNameAndTag(name, tag); warn != "" {
		log.Printf("[image-validate] warning on pull: %s", warn)
	}

	fullImage := fmt.Sprintf("%s:%s", name, tag)
	harborImage := fmt.Sprintf("%s%s:%s", cfg.HarborPrivatePrefix, name, tag)
	ttl := int32(300) // Clean up job 5 minutes after completion

	// Create a Job to pull the image, tag it, and push to Harbor
	k8sJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "image-puller-",
			Namespace:    "default",
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:            "puller",
							Image:           "docker:24-dind",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"/bin/sh", "-c"},
							Args: []string{
								fmt.Sprintf(`
									set -e
									echo "Pulling image %s..."
									docker pull %s
									echo "Tagging image for Harbor..."
									docker tag %s %s
									echo "Pushing to Harbor..."
									docker push %s
									echo "Successfully pushed to Harbor"
								`, fullImage, fullImage, fullImage, harborImage, harborImage),
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: boolPtr(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "docker-sock",
									MountPath: "/var/run/docker.sock",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "docker-sock",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/run/docker.sock",
								},
							},
						},
					},
				},
			},
		},
	}

	createdJob, err := k8s.Clientset.BatchV1().Jobs("default").Create(context.TODO(), k8sJob, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create image pull job: %v", err)
		return "", err
	}

	jobID := createdJob.Name
	pullTracker.AddJob(jobID, name, tag)
	pullTracker.UpdateJob(jobID, "pulling", 10, "Starting image pull...")

	// Start background monitoring goroutine
	go s.monitorPullJob(jobID, name, tag)

	log.Printf("Created pull job %s for image: %s", jobID, fullImage)
	return jobID, nil
}

// monitorPullJob watches a pull job and updates status when complete
func (s *ImageService) monitorPullJob(jobID, imageName, imageTag string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	maxRetries := 600 // 20 minutes max wait time
	retries := 0

	for range ticker.C {
		retries++
		if retries > maxRetries {
			pullTracker.UpdateJob(jobID, "failed", 0, "Job timeout")
			pullTracker.RemoveJob(jobID)
			return
		}

		k8sJob, err := k8s.Clientset.BatchV1().Jobs("default").Get(context.TODO(), jobID, metav1.GetOptions{})
		if err != nil {
			log.Printf("Error getting job %s: %v", jobID, err)
			pullTracker.UpdateJob(jobID, "pulling", 50, "Monitoring...")
			continue
		}

		// Check if job succeeded
		if k8sJob.Status.Succeeded > 0 {
			pullTracker.UpdateJob(jobID, "completed", 100, "Image pushed to Harbor successfully")

			// Update database: mark image as pulled
			if err := s.repo.UpdateImagePulledStatus(imageName, imageTag, true); err != nil {
				log.Printf("Failed to update image pulled status: %v", err)
			} else {
				log.Printf("Image %s:%s pushed to Harbor and marked as pulled", imageName, imageTag)
			}

			pullTracker.RemoveJob(jobID)
			return
		}

		// Check if job failed
		if k8sJob.Status.Failed > 0 {
			pullTracker.UpdateJob(jobID, "failed", 0, "Job failed to pull image")
			pullTracker.RemoveJob(jobID)
			return
		}

		// Update progress
		progress := 20 + (retries * 70 / maxRetries)
		if progress > 90 {
			progress = 90
		}
		pullTracker.UpdateJob(jobID, "pulling", progress, fmt.Sprintf("Pulling... (%d%%)", progress))
	}
}

// GetPullJobStatus returns the current status of a pull job
func (s *ImageService) GetPullJobStatus(jobID string) *PullJobStatus {
	return pullTracker.GetJob(jobID)
}

// SubscribeToPullJob returns a channel that receives status updates for a pull job
func (s *ImageService) SubscribeToPullJob(jobID string) <-chan *PullJobStatus {
	return pullTracker.Subscribe(jobID)
}

// GetFailedPullJobs returns recent failed pull jobs
func (s *ImageService) GetFailedPullJobs(limit int) []*PullJobStatus {
	return pullTracker.GetFailedJobs(limit)
}

// GetActivePullJobs returns currently active pull jobs
func (s *ImageService) GetActivePullJobs() []*PullJobStatus {
	return pullTracker.GetActiveJobs()
}

// validateNameAndTag performs lightweight format checks; returns warning string but does not block.
func (s *ImageService) validateNameAndTag(name, tag string) string {
	name = strings.TrimSpace(name)
	tag = strings.TrimSpace(tag)
	if name == "" || tag == "" {
		return "image name/tag should not be empty"
	}

	nameRe := regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*$`)
	if !nameRe.MatchString(name) {
		return "image name format looks invalid"
	}

	tagRe := regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)
	if !tagRe.MatchString(tag) {
		return "image tag format looks invalid"
	}

	return ""
}

func (s *ImageService) RemoveProjectImage(projectID, imageID uint) error {
	img, err := s.repo.FindAllowedByID(imageID)
	if err != nil {
		return err
	}
	if img.ProjectID == nil || *img.ProjectID != projectID {
		return fmt.Errorf("image does not belong to this project")
	}
	return s.repo.DeleteAllowedImage(imageID)
}

func (s *ImageService) DeleteAllowedImage(id uint) error {
	return s.repo.DeleteAllowedImage(id)
}
