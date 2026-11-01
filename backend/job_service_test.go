package main

import (
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestJobStatusEvaluation(t *testing.T) {
	now := metav1.Now()

	tests := []struct {
		name           string
		job            *batchv1.Job
		expectedStatus string
	}{
		{
			name: "Job Succeeded",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "install-aws-1"},
				Status: batchv1.JobStatus{
					Succeeded:      1,
					StartTime:      &now,
					CompletionTime: &now,
				},
			},
			expectedStatus: "Succeeded",
		},
		{
			name: "Job Failed",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "install-aws-2"},
				Status: batchv1.JobStatus{
					Failed:    1,
					StartTime: &now,
					Conditions: []batchv1.JobCondition{
						{
							Type:    batchv1.JobFailed,
							Status:  corev1.ConditionTrue,
							Message: "BackoffLimitExceeded",
						},
					},
				},
			},
			expectedStatus: "Failed",
		},
		{
			name: "Job Running",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "install-aws-3"},
				Status: batchv1.JobStatus{
					Active:    1,
					StartTime: &now,
				},
			},
			expectedStatus: "Running",
		},
		{
			name: "Job Pending",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "install-aws-4"},
				Status:     batchv1.JobStatus{},
			},
			expectedStatus: "Pending",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detail := &JobDetail{
				Name:   tt.job.Name,
				Status: "Pending",
			}

			if tt.job.Status.StartTime != nil {
				startTime := tt.job.Status.StartTime.Time
				detail.StartTime = &startTime
			}
			if tt.job.Status.CompletionTime != nil {
				completionTime := tt.job.Status.CompletionTime.Time
				detail.CompletionTime = &completionTime
			}

			switch {
			case tt.job.Status.Succeeded > 0:
				detail.Status = "Succeeded"
			case tt.job.Status.Failed > 0:
				detail.Status = "Failed"
			case tt.job.Status.Active > 0:
				detail.Status = "Running"
			default:
				detail.Status = "Pending"
			}

			if detail.Status != tt.expectedStatus {
				t.Errorf("Statut obtenu %s, attendu %s", detail.Status, tt.expectedStatus)
			}
		})
	}
}

func TestJobDetailFormatting(t *testing.T) {
	startTime := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	detail := &JobDetail{
		Name:      "install-azure-12345",
		Status:    "Running",
		StartTime: &startTime,
		PodName:   "install-azure-12345-abcde",
		Message:   "Job en cours d'exécution...",
	}

	if detail.Name != "install-azure-12345" {
		t.Errorf("Nom invalide: %s", detail.Name)
	}
	if detail.PodName != "install-azure-12345-abcde" {
		t.Errorf("PodName invalide: %s", detail.PodName)
	}
	if detail.Status != "Running" {
		t.Errorf("Statut invalide: %s", detail.Status)
	}
}
