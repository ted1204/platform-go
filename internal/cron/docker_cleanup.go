package cron

import (
	"context"
	"log"

	"github.com/linskybing/platform-go/pkg/k8s"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// boolPtr returns a pointer to a boolean value
func boolPtr(b bool) *bool {
	return &b
}

// CreateDockerCleanupCronJob creates a Kubernetes CronJob to periodically clean up Docker images
// This job runs daily at 2 AM to free up disk space by removing unused images
func CreateDockerCleanupCronJob() error {
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "docker-image-cleanup",
			Namespace: "default",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 2 * * *", // Daily at 2 AM
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{
								{
									Name:    "docker-cleanup",
									Image:   "docker:24-dind",
									Command: []string{"/bin/sh", "-c"},
									Args: []string{`
										set -e
										echo "Starting Docker cleanup..."
										# Remove all unused images, containers, and networks older than 24 hours
										docker system prune -af --filter "until=24h"
										echo "Docker cleanup completed successfully"
									`},
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
			},
		},
	}

	// Check if CronJob already exists
	existing, err := k8s.Clientset.BatchV1().CronJobs("default").Get(context.TODO(), "docker-image-cleanup", metav1.GetOptions{})
	if err == nil && existing != nil {
		// CronJob already exists, update it
		_, err = k8s.Clientset.BatchV1().CronJobs("default").Update(context.TODO(), cronJob, metav1.UpdateOptions{})
		if err != nil {
			log.Printf("Failed to update Docker cleanup CronJob: %v", err)
			return err
		}
		log.Println("Updated Docker cleanup CronJob successfully")
		return nil
	}

	// Create new CronJob
	_, err = k8s.Clientset.BatchV1().CronJobs("default").Create(context.TODO(), cronJob, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create Docker cleanup CronJob: %v", err)
		return err
	}

	log.Println("Created Docker cleanup CronJob successfully")
	return nil
}

// DeleteDockerCleanupCronJob deletes the Docker cleanup CronJob
func DeleteDockerCleanupCronJob() error {
	err := k8s.Clientset.BatchV1().CronJobs("default").Delete(context.TODO(), "docker-image-cleanup", metav1.DeleteOptions{})
	if err != nil {
		log.Printf("Failed to delete Docker cleanup CronJob: %v", err)
		return err
	}

	log.Println("Deleted Docker cleanup CronJob successfully")
	return nil
}
