package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SkillPackSpec defines the desired state of SkillPack.
// Skills are Markdown-based instruction bundles mounted into agent pods.
type SkillPackSpec struct {
	// Skills is the list of skills in this pack.
	Skills []Skill `json:"skills"`

	// RuntimeRequirements defines container image requirements for this skill pack.
	// +optional
	RuntimeRequirements *RuntimeRequirements `json:"runtimeRequirements,omitempty"`
}

// Skill defines a single skill entry.
type Skill struct {
	// Name is the skill identifier.
	Name string `json:"name"`

	// Description describes what this skill does.
	// +optional
	Description string `json:"description,omitempty"`

	// Requires lists runtime requirements (binaries, etc.) for this skill.
	// +optional
	Requires *SkillRequirements `json:"requires,omitempty"`

	// Content is the Markdown content of the skill.
	Content string `json:"content"`
}

// SkillRequirements defines what a skill needs at runtime.
type SkillRequirements struct {
	// Bins lists required binaries.
	Bins []string `json:"bins,omitempty"`
}

// RuntimeRequirements defines container-level requirements.
type RuntimeRequirements struct {
	// Image is the container image that satisfies these requirements.
	Image string `json:"image"`
}

// SkillPackStatus defines the observed state of SkillPack.
type SkillPackStatus struct {
	// Phase is the current phase.
	// +optional
	Phase string `json:"phase,omitempty"`

	// ConfigMapName is the name of the generated ConfigMap.
	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`

	// SkillCount is the number of skills in this pack.
	// +optional
	SkillCount int `json:"skillCount,omitempty"`

	// Conditions represent the latest available observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Skills",type="integer",JSONPath=".status.skillCount"
// +kubebuilder:printcolumn:name="ConfigMap",type="string",JSONPath=".status.configMapName"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// SkillPack is the Schema for the skillpacks API.
// It bundles portable skills as a CRD that produces ConfigMaps mounted into agent pods.
type SkillPack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SkillPackSpec   `json:"spec,omitempty"`
	Status SkillPackStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SkillPackList contains a list of SkillPack.
type SkillPackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SkillPack `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SkillPack{}, &SkillPackList{})
}
