package executor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/linskybing/platform-go/internal/domain/job"
	"github.com/linskybing/platform-go/pkg/k8s"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// K8sExecutor runs jobs on Kubernetes.
type K8sExecutor struct {
	jobRepo job.Repository
}

// NewK8sExecutor constructs a Kubernetes-backed executor.
func NewK8sExecutor(jobRepo job.Repository) *K8sExecutor {
	return &K8sExecutor{jobRepo: jobRepo}
}

func (e *K8sExecutor) Execute(ctx context.Context, j *job.Job) error {
	var cmd []string
	if j.Command != "" {
		_ = json.Unmarshal([]byte(j.Command), &cmd)
	}
	var args []string
	if j.Args != "" {
		_ = json.Unmarshal([]byte(j.Args), &args)
	}

	spec := k8s.JobSpec{
		Name:              j.K8sJobName,
		Namespace:         j.Namespace,
		Image:             j.Image,
		Command:           append(cmd, args...),
		PriorityClassName: "low-priority",
		Parallelism:       1,
		Completions:       1,
		GPUCount:          j.GPUCount,
		GPUType:           j.GPUType,
		EnvVars:           map[string]string{},
		Annotations:       map[string]string{},
	}

	if err := k8s.CreateJob(ctx, spec); err != nil {
		return err
	}

	if e.jobRepo != nil {
		j.Status = string(job.JobStatusRunning)
		if err := e.jobRepo.Update(j); err != nil {
			log.Printf("update job status failed: %v", err)
		}
	}

	// Watch job completion and collect logs asynchronously
	go e.watchJob(ctx, j)
	go e.followLogs(ctx, j)
	return nil
}

func (e *K8sExecutor) Cancel(ctx context.Context, jobID uint) error {
	if e.jobRepo == nil {
		return nil
	}
	obj, err := e.jobRepo.FindByID(jobID)
	if err != nil {
		return err
	}
	if err := k8s.DeleteJob(ctx, obj.Namespace, obj.K8sJobName); err != nil {
		return err
	}
	obj.Status = string(job.StatusCancelled)
	return e.jobRepo.Update(obj)
}

func (e *K8sExecutor) GetStatus(ctx context.Context, jobID uint) (job.JobStatus, error) {
	if e.jobRepo == nil {
		return job.StatusPending, nil
	}
	obj, err := e.jobRepo.FindByID(jobID)
	if err != nil {
		return job.StatusPending, err
	}
	return job.JobStatus(obj.Status), nil
}

func (e *K8sExecutor) GetLogs(ctx context.Context, jobID uint) (string, error) {
	// Placeholder: log streaming to be added; return empty for now.
	return "", nil
}

func (e *K8sExecutor) SupportsType(jobType job.JobType) bool {
	return jobType == job.JobTypeNormal || jobType == job.JobTypeGPU || jobType == ""
}

// followLogs streams pod logs and persists to repository while the job runs.
func (e *K8sExecutor) followLogs(ctx context.Context, j *job.Job) {
	if e.jobRepo == nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}

		podName := e.pickPodForJob(ctx, j.Namespace, j.K8sJobName)
		if podName == "" {
			continue
		}

		req := k8s.Clientset.CoreV1().Pods(j.Namespace).GetLogs(podName, &corev1.PodLogOptions{Follow: true})
		stream, err := req.Stream(ctx)
		if err != nil {
			log.Printf("log follow stream err: %v", err)
			continue
		}

		scanner := bufio.NewScanner(stream)
		for scanner.Scan() {
			line := scanner.Text()
			_ = e.jobRepo.SaveLog(&job.JobLog{JobID: j.ID, Content: line})
		}
		stream.Close()
		return
	}
}

func (e *K8sExecutor) pickPodForJob(ctx context.Context, ns, jobName string) string {
	labelSelector := fmt.Sprintf("job-name=%s", jobName)
	pods, err := k8s.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return ""
	}
	for i := range pods.Items {
		p := pods.Items[i]
		if p.Status.Phase == corev1.PodRunning || p.Status.Phase == corev1.PodPending || p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
			return p.Name
		}
	}
	return ""
}

func (e *K8sExecutor) watchJob(ctx context.Context, j *job.Job) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}

		jobObj, err := k8s.Clientset.BatchV1().Jobs(j.Namespace).Get(ctx, j.K8sJobName, metav1.GetOptions{})
		if err != nil {
			log.Printf("watch job get err: %v", err)
			continue
		}

		status, done := evaluateJobStatus(jobObj)
		if !done {
			continue
		}

		logs := e.collectLogs(ctx, j.Namespace, j.K8sJobName)
		if e.jobRepo != nil {
			j.Status = string(status)
			now := time.Now()
			j.CompletedAt = &now
			if err := e.jobRepo.Update(j); err != nil {
				log.Printf("update job final status failed: %v", err)
			}
			if logs != "" {
				_ = e.jobRepo.SaveLog(&job.JobLog{JobID: j.ID, Content: logs})
			}
		}
		return
	}
}

func evaluateJobStatus(obj *batchv1.Job) (job.JobStatus, bool) {
	if obj.Status.Succeeded > 0 {
		return job.StatusCompleted, true
	}
	if obj.Status.Failed > 0 {
		return job.StatusFailed, true
	}
	return job.JobStatusRunning, false
}

func (e *K8sExecutor) collectLogs(ctx context.Context, namespace, jobName string) string {
	labelSelector := fmt.Sprintf("job-name=%s", jobName)
	pods, err := k8s.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		log.Printf("list pods for logs err: %v", err)
		return ""
	}

	var combined string
	for i := range pods.Items {
		combined += e.readPodLog(ctx, &pods.Items[i])
	}
	return combined
}

func (e *K8sExecutor) readPodLog(ctx context.Context, pod *corev1.Pod) string {
	req := k8s.Clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
	stream, err := req.Stream(ctx)
	if err != nil {
		log.Printf("pod log stream err: %v", err)
		return ""
	}
	defer stream.Close()
	data, err := io.ReadAll(stream)
	if err != nil {
		log.Printf("read pod log err: %v", err)
		return ""
	}
	return string(data)
}
