/*
Copyright 2020 The Kruise Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// BroadcastCronJobSpec defines the desired state of BroadcastCronJob
type BroadcastCronJobSpec struct {
	Schedule string `json:"schedule" protobuf:"bytes,1,opt,name=schedule"`
	// Parallelism specifies the maximum desired number of pods the job should
	// run at any given time. The actual number of pods running in steady state will
	// be less than this number when the work left to do is less than max parallelism.
	// Not setting this value means no limit.
	// +optional
	Parallelism *intstr.IntOrString `json:"parallelism,omitempty" protobuf:"varint,2,opt,name=parallelism"`

	// Template describes the pod that will be created when executing a job.
	Template v1.PodTemplateSpec `json:"template" protobuf:"bytes,3,opt,name=template"`

	// CompletionPolicy indicates the completion policy of the job.
	// Default is Always CompletionPolicyType
	// +optional
	CompletionPolicy CompletionPolicy `json:"completionPolicy" protobuf:"bytes,4,opt,name=completionPolicy"`

	// Specifies how to treat concurrent executions of a Job.
	// Valid values are:
	// - "Allow" (default): allows CronJobs to run concurrently;
	// - "Forbid": forbids concurrent runs, skipping next run if previous run hasn't finished yet;
	// - "Replace": cancels currently running job and replaces it with a new one
	// +optional
	ConcurrencyPolicy ConcurrencyPolicy `json:"concurrencyPolicy,omitempty"`

	// Paused will pause the job.
	// +optional
	Paused bool `json:"paused,omitempty" protobuf:"bytes,5,opt,name=paused"`

	// FailurePolicy indicates the behavior of the job, when failed pod is found.
	// +optional
	FailurePolicy FailurePolicy `json:"failurePolicy,omitempty" protobuf:"bytes,6,opt,name=failurePolicy"`

	// +kubebuilder:validation:Minimum=0

	// The number of failed finished jobs to retain.
	// This is a pointer to distinguish between explicit zero and not specified.
	// +optional
	FailedJobsHistoryLimit *int32 `json:"failedJobsHistoryLimit,omitempty"`

	// +kubebuilder:validation:Minimum=0

	// The number of successful finished jobs to retain.
	// This is a pointer to distinguish between explicit zero and not specified.
	// +optional
	SuccessfulJobsHistoryLimit *int32 `json:"successfulJobsHistoryLimit,omitempty"`

	// Optional deadline in seconds for starting the job if it misses scheduled
	// time for any reason.  Missed jobs executions will be counted as failed ones.
	// +optional
	StartingDeadlineSeconds *int64 `json:"startingDeadlineSeconds,omitempty"`
}

// BroadcastCronJobStatus defines the observed state of BroadcastCronJob
type BroadcastCronJobStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// A list of pointers to currently running jobs.
	// +optional
	Active []corev1.ObjectReference `json:"active,omitempty"`

	// Information when was the last time the job was successfully scheduled.
	// +optional
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`
}

type BroadcastCronJobPhase string

const (

	// PhaseRunning means the job is running.
	CronPhaseRunning BroadcastCronJobPhase = "running"

	// PhasePaused means the job is paused.
	CronPhasePaused BroadcastCronJobPhase = "scheduled"

	// PhaseFailed means the job is failed.
	CronPhaseFailed BroadcastCronJobPhase = "failed"
)

// ConcurrencyPolicy describes how the job will be handled.
// Only one of the following concurrent policies may be specified.
// If none of the following policies is specified, the default one
// is AllowConcurrent.
// +kubebuilder:validation:Enum=Allow;Forbid;Replace
type ConcurrencyPolicy string

const (
	// AllowConcurrent allows CronJobs to run concurrently.
	AllowConcurrent ConcurrencyPolicy = "Allow"

	// ForbidConcurrent forbids concurrent runs, skipping next run if previous
	// hasn't finished yet.
	ForbidConcurrent ConcurrencyPolicy = "Forbid"

	// ReplaceConcurrent cancels currently running job and replaces it with a new one.
	ReplaceConcurrent ConcurrencyPolicy = "Replace"
)

// +kubebuilder:object:root=true

// BroadcastCronJob is the Schema for the broadcastcronjobs API
type BroadcastCronJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BroadcastCronJobSpec   `json:"spec,omitempty"`
	Status BroadcastCronJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BroadcastCronJobList contains a list of BroadcastCronJob
type BroadcastCronJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BroadcastCronJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BroadcastCronJob{}, &BroadcastCronJobList{})
}
