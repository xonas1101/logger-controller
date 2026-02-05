/*
Copyright 2026.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LoggerSpec defines the desired state of Logger
type LoggerSpec struct {
	Scope     ScopeSpec   `json:"scope"`
	Resources []string    `json:"resources"`
	Trigger   TriggerSpec `json:"trigger"`
}

type ScopeSpec struct {
	Type      string `json:"type"`
	Namespace string `json:"namespace,omitempty"`
}

type TriggerSpec struct {
	Interval    string `json:"interval,omitempty"`
	WatchEvents bool   `json:"watchevents,omitempty"`
}

// LoggerStatus defines the observed state of Logger.
type LoggerStatus struct {
	Conditions        []metav1.Condition `json:"conditions,omitempty"`
	LastRunTime       *metav1.Time       `json:"lastRunTime,omitempty"`
	ObservedResources int32              `json:"observedResources,omitempty"`
	LastError         string             `json:"lastError,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Logger is the Schema for the loggers API
type Logger struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Logger
	// +required
	Spec LoggerSpec `json:"spec"`

	// status defines the observed state of Logger
	// +optional
	Status LoggerStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// LoggerList contains a list of Logger
type LoggerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Logger `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Logger{}, &LoggerList{})
}
