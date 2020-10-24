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

	// Paused will pause the job.
	// +optional
	Paused bool `json:"paused,omitempty" protobuf:"bytes,5,opt,name=paused"`

	// FailurePolicy indicates the behavior of the job, when failed pod is found.
	// +optional
	FailurePolicy FailurePolicy `json:"failurePolicy,omitempty" protobuf:"bytes,6,opt,name=failurePolicy"`
}

// BroadcastCronJobStatus defines the observed state of BroadcastCronJob
type BroadcastCronJobStatus struct {
	// The latest available observations of an object's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []JobCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`

	// Represents time when the job was acknowledged by the job controller.
	// It is not guaranteed to be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty" protobuf:"bytes,2,opt,name=startTime"`

	// Represents time when the job was completed. It is not guaranteed to
	// be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty" protobuf:"bytes,3,opt,name=completionTime"`

	// The number of actively running pods.
	// +optional
	Active int32 `json:"active" protobuf:"varint,4,opt,name=active"`

	// The number of pods which reached phase Succeeded.
	// +optional
	Succeeded int32 `json:"succeeded" protobuf:"varint,5,opt,name=succeeded"`

	// The number of pods which reached phase Failed.
	// +optional
	Failed int32 `json:"failed" protobuf:"varint,6,opt,name=failed"`

	// The desired number of pods, this is typically equal to the number of nodes satisfied to run pods.
	// +optional
	Desired int32 `json:"desired" protobuf:"varint,7,opt,name=desired"`

	// The phase of the job.
	// +optional
	Phase BroadcastCronJobPhase `json:"phase" protobuf:"varint,8,opt,name=phase"`
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
