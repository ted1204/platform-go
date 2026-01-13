package application

import (
	"context"
	"fmt"
	"io"
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

func ptrTime(t time.Time) *time.Time { return &t }

type PullJobStatus struct {
	JobID     string    `json:"job_id"`
	ImageName string    `json:"image_name"`
	ImageTag  string    `json:"image_tag"`
	Status    string    `json:"status"`
	Progress  int       `json:"progress"`
	Message   string    `json:"message"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PullJobTracker struct {
	mu         sync.RWMutex
	jobs       map[string]*PullJobStatus
	chans      map[string][]chan *PullJobStatus
	failedJobs []*PullJobStatus
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

	if job, ok := pt.jobs[jobID]; ok && job.Status == "failed" {
		pt.failedJobs = append(pt.failedJobs, job)
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

func (s *ImageService) SubmitRequest(userID uint, registry, name, tag string, projectID *uint) (*image.ImageRequest, error) {
	if registry == "" {
		registry = "docker.io"
	}

	req := &image.ImageRequest{
		UserID:         userID,
		ProjectID:      projectID,
		InputRegistry:  registry,
		InputImageName: name,
		InputTag:       tag,
		Status:         "pending",
	}

	if warn := s.validateNameAndTag(name, tag); warn != "" {
		log.Printf("[image-validate] warning: %s", warn)
	}
	return req, s.repo.CreateRequest(req)
}

func (s *ImageService) ListRequests(projectID *uint, status string) ([]image.ImageRequest, error) {
	return s.repo.ListRequests(projectID, status)
}

func (s *ImageService) ApproveRequest(id uint, note string, isGlobal bool, approverID uint) error {
	req, err := s.repo.FindRequestByID(id)
	if err != nil {
		return err
	}

	// If approver marked this as global, clear ProjectID so the created
	// allow-list rule will be global (ProjectID == nil)
	if isGlobal {
		req.ProjectID = nil
	}

	req.Status = "approved"
	req.ReviewerNote = note
	req.ReviewerID = &approverID
	req.ReviewedAt = ptrTime(time.Now())

	if err := s.repo.UpdateRequest(req); err != nil {
		return err
	}

	return s.createCoreAndPolicyFromRequest(req, approverID)
}

func (s *ImageService) createCoreAndPolicyFromRequest(req *image.ImageRequest, adminID uint) error {
	fullName := req.InputImageName
	if req.InputRegistry != "" && req.InputRegistry != "docker.io" {
		fullName = fmt.Sprintf("%s/%s", req.InputRegistry, req.InputImageName)
	}

	parts := strings.Split(req.InputImageName, "/")
	var namespace, name string
	if len(parts) >= 2 {
		namespace = parts[0]
		name = strings.Join(parts[1:], "/")
	} else {
		namespace = "library"
		name = req.InputImageName
	}

	repoEntity := &image.ContainerRepository{
		Registry:  req.InputRegistry,
		Namespace: namespace,
		Name:      name,
		FullName:  fullName,
	}
	if err := s.repo.FindOrCreateRepository(repoEntity); err != nil {
		return err
	}

	tagEntity := &image.ContainerTag{
		RepositoryID: repoEntity.ID,
		Name:         req.InputTag,
	}
	if err := s.repo.FindOrCreateTag(tagEntity); err != nil {
		return err
	}

	rule := &image.ImageAllowList{
		ProjectID:    req.ProjectID,
		RepositoryID: repoEntity.ID,
		TagID:        &tagEntity.ID,
		RequestID:    &req.ID,
		CreatedBy:    adminID,
		IsEnabled:    true,
	}

	return s.repo.CreateAllowListRule(rule)
}

func (s *ImageService) RejectRequest(id uint, note string, approverID uint) (*image.ImageRequest, error) {
	req, err := s.repo.FindRequestByID(id)
	if err != nil {
		return nil, err
	}
	req.Status = "rejected"
	req.ReviewerNote = note
	req.ReviewerID = &approverID
	req.ReviewedAt = ptrTime(time.Now())

	if err := s.repo.UpdateRequest(req); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *ImageService) GetAllowedImage(name, tag string, projectID uint) (*image.AllowedImageDTO, error) {
	rule, err := s.repo.FindAllowListRule(&projectID, name, tag)
	if err != nil {
		return nil, err
	}

	status, _ := s.repo.GetClusterStatus(rule.Tag.ID)
	isPulled := false
	if status != nil {
		isPulled = status.IsPulled
	}

	return &image.AllowedImageDTO{
		ID:        rule.ID,
		Registry:  rule.Repository.Registry,
		ImageName: rule.Repository.Name,
		Tag:       rule.Tag.Name,
		Digest:    rule.Tag.Digest,
		ProjectID: rule.ProjectID,
		IsGlobal:  rule.ProjectID == nil,
		IsPulled:  isPulled,
	}, nil
}

func (s *ImageService) ListAllowedImages(projectID *uint) ([]image.AllowedImageDTO, error) {
	rules, err := s.repo.ListAllowedImages(projectID)
	if err != nil {
		return nil, err
	}

	var dtos []image.AllowedImageDTO
	for _, rule := range rules {
		isGlobal := rule.ProjectID == nil

		status, _ := s.repo.GetClusterStatus(rule.Tag.ID)
		isPulled := false
		if status != nil {
			isPulled = status.IsPulled
		}

		displayImageName := rule.Repository.FullName
		if displayImageName == "" {
			if rule.Repository.Namespace != "" {
				displayImageName = fmt.Sprintf("%s/%s", rule.Repository.Namespace, rule.Repository.Name)
			} else {
				displayImageName = rule.Repository.Name
			}
		}

		dtos = append(dtos, image.AllowedImageDTO{
			ID:        rule.ID,
			Registry:  rule.Repository.Registry,
			ImageName: displayImageName,
			Tag:       rule.Tag.Name,
			Digest:    rule.Tag.Digest,
			ProjectID: rule.ProjectID,
			IsGlobal:  isGlobal,
			IsPulled:  isPulled,
		})
	}
	return dtos, nil
}

func (s *ImageService) AddProjectImage(userID uint, projectID uint, name, tag string) error {
	if warn := s.validateNameAndTag(name, tag); warn != "" {
		return fmt.Errorf("invalid image format: %s", warn)
	}

	req := &image.ImageRequest{
		UserID:         userID,
		ProjectID:      &projectID,
		InputImageName: name,
		InputTag:       tag,
		Status:         "pending", // Needs approval
	}

	return s.repo.CreateRequest(req)
}

func (s *ImageService) ValidateImageForProject(name, tag string, projectID *uint) (bool, error) {
	fullName := name
	if !strings.Contains(name, "/") {
		// handle simple names like "nginx" implying "docker.io/library/nginx" logic handling or partial match
		// For simplicity, we assume repo.FullName is stored fully.
		// A robust implementation needs to normalize input name to match DB FullName.
	}
	return s.repo.CheckImageAllowed(projectID, fullName, tag)
}

func (s *ImageService) PullImageAsync(name, tag string) (string, error) {
	if warn := s.validateNameAndTag(name, tag); warn != "" {
		log.Printf("[image-validate] warning on pull: %s", warn)
	}

	// --- [修正] 映像檔名稱正規化邏輯 ---
	// 解決 codercom/code-server 變成 code-server:latest 或找不到 Registry 的問題
	normalizedName := name
	parts := strings.Split(name, "/")

	// 檢查第一部分是否包含 Domain 特徵 (. 或 :) 或為 localhost
	hasDomain := strings.Contains(parts[0], ".") ||
		strings.Contains(parts[0], ":") ||
		parts[0] == "localhost"

	if !hasDomain {
		if len(parts) == 1 {
			// Case: "nginx" -> "docker.io/library/nginx"
			normalizedName = "docker.io/library/" + name
		} else {
			// Case: "codercom/code-server" -> "docker.io/codercom/code-server"
			normalizedName = "docker.io/" + name
		}
	}

	// 這是用於 K8s InitContainer 拉取以及 crane copy 來源的完整位址
	fullImage := fmt.Sprintf("%s:%s", normalizedName, tag)

	// 這是推送到內部 Harbor 的位址 (維持原始路徑結構)
	harborImage := fmt.Sprintf("%s%s:%s", cfg.HarborPrivatePrefix, name, tag)
	// --------------------------------

	ttl := int32(300)

	k8sJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "image-puller-",
			Namespace:    "default",
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					ServiceAccountName: "default",
					InitContainers: []corev1.Container{
						{
							Name:            "pull-source",
							Image:           fullImage,
							ImagePullPolicy: corev1.PullAlways,
							Command:         []string{"/bin/sh", "-c", "echo 'Image pulled successfully'"},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "push-to-harbor",
							Image:           "gcr.io/go-containerregistry/crane:latest",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"crane", "copy", fullImage, harborImage, "--insecure"},
							Env: []corev1.EnvVar{
								{
									Name:  "DOCKER_CONFIG",
									Value: "/kaniko/.docker",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "docker-config",
									MountPath: "/kaniko/.docker",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "docker-config",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "harbor-regcred",
									Items: []corev1.KeyToPath{
										{
											Key:  ".dockerconfigjson",
											Path: "config.json",
										},
									},
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

	go s.monitorPullJob(jobID, name, tag)

	log.Printf("Created pull job %s for image: %s", jobID, fullImage)
	return jobID, nil
}

func (s *ImageService) monitorPullJob(jobID, imageName, imageTag string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	maxRetries := 600
	retries := 0

	for range ticker.C {
		retries++
		if retries > maxRetries {
			logs := s.getPodLogsForJob(jobID)
			errMsg := fmt.Sprintf("Job timeout. Logs: %s", logs)
			pullTracker.UpdateJob(jobID, "failed", 0, errMsg)
			pullTracker.RemoveJob(jobID)
			return
		}

		k8sJob, err := k8s.Clientset.BatchV1().Jobs("default").Get(context.TODO(), jobID, metav1.GetOptions{})
		if err != nil {
			log.Printf("Error getting job %s: %v", jobID, err)
			continue
		}

		labelSelector := fmt.Sprintf("job-name=%s", jobID)
		pods, err := k8s.Clientset.CoreV1().Pods("default").List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})

		var statusMsg string
		var progress int

		if err == nil && len(pods.Items) > 0 {
			pod := &pods.Items[0]

			if len(pod.Status.InitContainerStatuses) > 0 {
				initStatus := pod.Status.InitContainerStatuses[0]
				if initStatus.State.Running != nil {
					statusMsg = "Pulling source image..."
					progress = 30
				} else if initStatus.State.Terminated != nil {
					if initStatus.State.Terminated.ExitCode == 0 {
						statusMsg = "Source image pulled, pushing to Harbor..."
						progress = 60
					} else {
						logs := s.getPodLogsForJob(jobID)
						errMsg := fmt.Sprintf("Failed to pull source image. Logs: %s", logs)
						pullTracker.UpdateJob(jobID, "failed", 0, errMsg)
						pullTracker.RemoveJob(jobID)
						return
					}
				} else if initStatus.State.Waiting != nil {
					statusMsg = fmt.Sprintf("Waiting: %s", initStatus.State.Waiting.Reason)
					progress = 10
				}
			}

			if len(pod.Status.ContainerStatuses) > 0 {
				containerStatus := pod.Status.ContainerStatuses[0]
				if containerStatus.State.Running != nil {
					statusMsg = "Pushing to Harbor..."
					progress = 80
				} else if containerStatus.State.Waiting != nil {
					if statusMsg == "" {
						statusMsg = fmt.Sprintf("Waiting: %s", containerStatus.State.Waiting.Reason)
						progress = 50
					}
				}
			}

			switch pod.Status.Phase {
			case corev1.PodPending:
				if statusMsg == "" {
					statusMsg = "Pod pending..."
					progress = 5
				}
			case corev1.PodRunning:
				if statusMsg == "" {
					statusMsg = "Processing..."
					progress = 50
				}
			case corev1.PodFailed:
				logs := s.getPodLogsForJob(jobID)
				errMsg := fmt.Sprintf("Pod failed. Logs: %s", logs)
				pullTracker.UpdateJob(jobID, "failed", 0, errMsg)
				pullTracker.RemoveJob(jobID)
				return
			}
		} else {
			statusMsg = "Initializing..."
			progress = 5
		}

		if k8sJob.Status.Succeeded > 0 {
			pullTracker.UpdateJob(jobID, "completed", 100, "Image pushed to Harbor successfully")

			s.markImageAsPulled(imageName, imageTag)

			pullTracker.RemoveJob(jobID)
			return
		}

		if k8sJob.Status.Failed > 0 {
			logs := s.getPodLogsForJob(jobID)
			errMsg := fmt.Sprintf("Job failed. Logs: %s", logs)
			pullTracker.UpdateJob(jobID, "failed", 0, errMsg)
			pullTracker.RemoveJob(jobID)
			return
		}

		pullTracker.UpdateJob(jobID, "pulling", progress, statusMsg)
	}
}

func (s *ImageService) markImageAsPulled(name, tag string) {
	parts := strings.Split(name, "/")
	var namespace, repoName string
	if len(parts) >= 2 {
		namespace = parts[0]
		repoName = strings.Join(parts[1:], "/")
	} else {
		namespace = "library"
		repoName = name
	}

	repo := &image.ContainerRepository{
		Namespace: namespace,
		Name:      repoName,
		FullName:  name,
	}
	if err := s.repo.FindOrCreateRepository(repo); err != nil {
		log.Printf("Failed to find repo for status update: %v", err)
		return
	}

	tagEntity := &image.ContainerTag{
		RepositoryID: repo.ID,
		Name:         tag,
	}
	if err := s.repo.FindOrCreateTag(tagEntity); err != nil {
		log.Printf("Failed to find tag for status update: %v", err)
		return
	}

	status := &image.ClusterImageStatus{
		TagID:        tagEntity.ID,
		IsPulled:     true,
		LastPulledAt: ptrTime(time.Now()),
	}

	if err := s.repo.UpdateClusterStatus(status); err != nil {
		log.Printf("Failed to update cluster status: %v", err)
	} else {
		log.Printf("Cluster status updated for %s:%s", name, tag)
	}
}

func (s *ImageService) getPodLogsForJob(jobName string) string {
	ctx := context.TODO()
	labelSelector := fmt.Sprintf("job-name=%s", jobName)

	pods, err := k8s.Clientset.CoreV1().Pods("default").List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Sprintf("Error listing pods: %v", err)
	}

	if len(pods.Items) == 0 {
		return "No pods found for this job"
	}

	var logBuilder strings.Builder
	for i := range pods.Items {
		pod := &pods.Items[i]
		logBuilder.WriteString(fmt.Sprintf("=== Pod: %s ===\n", pod.Name))

		req := k8s.Clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
			TailLines: func() *int64 { i := int64(100); return &i }(),
		})

		stream, err := req.Stream(ctx)
		if err != nil {
			logBuilder.WriteString(fmt.Sprintf("Error getting logs: %v\n", err))
			continue
		}

		data, err := io.ReadAll(stream)
		_ = stream.Close()
		if err != nil {
			logBuilder.WriteString(fmt.Sprintf("Error reading logs: %v\n", err))
			continue
		}

		logBuilder.Write(data)
		logBuilder.WriteString("\n")
	}

	return logBuilder.String()
}

func (s *ImageService) GetPullJobStatus(jobID string) *PullJobStatus {
	return pullTracker.GetJob(jobID)
}

func (s *ImageService) SubscribeToPullJob(jobID string) <-chan *PullJobStatus {
	return pullTracker.Subscribe(jobID)
}

func (s *ImageService) GetFailedPullJobs(limit int) []*PullJobStatus {
	return pullTracker.GetFailedJobs(limit)
}

func (s *ImageService) GetActivePullJobs() []*PullJobStatus {
	return pullTracker.GetActiveJobs()
}

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

func (s *ImageService) DisableAllowListRule(id uint) error {
	return s.repo.DisableAllowListRule(id)
}
