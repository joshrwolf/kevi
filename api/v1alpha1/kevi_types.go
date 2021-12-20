/*
Copyright 2021.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	KeviPackageManifestType   = "manifest"
	KeviPackageChartType      = "chart"
	KeviPackageUnknowntype    = "unknown"
	KeviPackageLayerMediaType = "application/vnd.kevi.cattle.io.package.layer"
	ManifestLayerMediaType    = "application/vnd.kevi.cattle.io.kustomize.layer.tar+gzip"
)

// KeviSpec defines the desired state of Kevi
type KeviSpec struct {
	Packages []KeviSpecPackage `json:"packages,omitempty"`
}

type KeviSpecPackage struct {
	Name     string                  `json:"name,omitempty"`
	Manifest KeviSpecPackageManifest `json:"manifest,omitempty"`
	Chart    KeviSpecPackageChart    `json:"chart,omitempty"`
	Images   []string                `json:"images,omitempty"`
}

func (in *KeviSpecPackage) Identify() string {
	if in.Manifest.Path != "" {
		return KeviPackageManifestType

	} else if in.Chart.Path != "" || in.Chart.RepoUrl != "" {
		return KeviPackageChartType
	}
	return KeviPackageUnknowntype
}

type KeviSpecPackageManifest struct {
	Path string `json:"path"`
}

type KeviSpecPackageChart struct {
	Path    string `json:"path,omitempty"`
	Name    string `json:"name,omitempty"`
	RepoUrl string `json:"repoUrl,omitempty"`
	Version string `json:"version,omitempty"`
}

// KeviStatus defines the observed state of Kevi
type KeviStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Kevi is the Schema for the kevis API
type Kevi struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KeviSpec   `json:"spec,omitempty"`
	Status KeviStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KeviList contains a list of Kevi
type KeviList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Kevi `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Kevi{}, &KeviList{})
}
