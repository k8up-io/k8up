/*


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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PreBackupPodSpec defines the desired state of PreBackupPod
type PreBackupPodSpec struct {
	// BackupCommand will be added to the backupcommand annotation on the pod.
	BackupCommand string `json:"backupCommand,omitempty"`
	FileExtension string `json:"fileExtension,omitempty"`
	// +kubebuilder:validation:Required
	Pod *Pod `json:"pod,omitempty"`
}

// Pod is a dummy struct to fix some code generation issues.
type Pod struct {
	corev1.PodTemplateSpec `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PreBackupPod is the Schema for the prebackuppods API
type PreBackupPod struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PreBackupPodSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// PreBackupPodList contains a list of PreBackupPod
type PreBackupPodList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PreBackupPod `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PreBackupPod{}, &PreBackupPodList{})
}
