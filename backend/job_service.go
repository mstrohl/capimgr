package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// JobDetail représente le statut d'un Job K8s d'installation ou désinstallation
type JobDetail struct {
	Name           string     `json:"name"`
	Status         string     `json:"status"` // Pending, Running, Succeeded, Failed
	StartTime      *time.Time `json:"startTime,omitempty"`
	CompletionTime *time.Time `json:"completionTime,omitempty"`
	PodName        string     `json:"podName,omitempty"`
	Message        string     `json:"message,omitempty"`
}

const defaultJobNamespace = "capi-dashboard-system"

// getJobStatus inspecte l'état d'un Job Kubernetes
func getJobStatus(ctx context.Context, jobName string) (*JobDetail, error) {
	job := &batchv1.Job{}
	key := types.NamespacedName{
		Namespace: defaultJobNamespace,
		Name:      jobName,
	}

	if err := k8sClient.Get(ctx, key, job); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("job %s non trouvé dans le namespace %s", jobName, defaultJobNamespace)
		}
		return nil, fmt.Errorf("erreur lors de la récupération du job %s: %w", jobName, err)
	}

	detail := &JobDetail{
		Name:   jobName,
		Status: "Pending",
	}

	if job.Status.StartTime != nil {
		t := job.Status.StartTime.Time
		detail.StartTime = &t
	}
	if job.Status.CompletionTime != nil {
		t := job.Status.CompletionTime.Time
		detail.CompletionTime = &t
	}

	// Évaluation de l'état
	switch {
	case job.Status.Succeeded > 0:
		detail.Status = "Succeeded"
		detail.Message = "Opération terminée avec succès."
	case job.Status.Failed > 0:
		detail.Status = "Failed"
		detail.Message = "L'exécution du Job a échoué."
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
				if cond.Message != "" {
					detail.Message = cond.Message
				}
			}
		}
	case job.Status.Active > 0:
		detail.Status = "Running"
		detail.Message = "Job en cours d'exécution..."
	default:
		detail.Status = "Pending"
		detail.Message = "En attente de démarrage du Pod..."
	}

	// Recherche du Pod associé
	podName, _ := findJobPodName(ctx, jobName, defaultJobNamespace)
	detail.PodName = podName

	return detail, nil
}

// getJobLogs extrait les logs du Pod exécuté par le Job
func getJobLogs(ctx context.Context, jobName string) (string, error) {
	podName, err := findJobPodName(ctx, jobName, defaultJobNamespace)
	if err != nil {
		return "", err
	}
	if podName == "" {
		return "Aucun Pod actif ou complété trouvé pour ce Job.", nil
	}

	if k8sClientset == nil {
		return "", fmt.Errorf("client K8s Core non configuré pour la lecture des logs")
	}

	req := k8sClientset.CoreV1().Pods(defaultJobNamespace).GetLogs(podName, &corev1.PodLogOptions{})
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return fmt.Sprintf("Logs non disponibles actuellement (%v)", err), nil
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, podLogs); err != nil {
		return "", fmt.Errorf("erreur lors de la lecture des logs: %w", err)
	}

	return buf.String(), nil
}

// findJobPodName recherche le nom du Pod rattaché au Job via les labels standards
func findJobPodName(ctx context.Context, jobName, namespace string) (string, error) {
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{"batch.kubernetes.io/job-name": jobName},
	}

	if err := k8sClient.List(ctx, podList, listOpts...); err != nil {
		// Fallback sur l'ancien label 'job-name'
		listOpts = []client.ListOption{
			client.InNamespace(namespace),
			client.MatchingLabels{"job-name": jobName},
		}
		if err2 := k8sClient.List(ctx, podList, listOpts...); err2 != nil {
			return "", fmt.Errorf("erreur lors de la recherche du Pod du job: %w", err)
		}
	}

	if len(podList.Items) == 0 {
		return "", nil
	}

	// Retourner le pod le plus récent
	latestPod := podList.Items[0]
	for _, p := range podList.Items[1:] {
		if p.CreationTimestamp.After(latestPod.CreationTimestamp.Time) {
			latestPod = p
		}
	}

	return latestPod.Name, nil
}
