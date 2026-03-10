// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Pack describes a pack.
type Pack struct {
	// Name specifies the name of the pack.
	//
	// +k8s:required
	Name string `json:"name,omitzero"`

	// Version specifies the version of the pack.
	//
	// +k8s:required
	Version string `json:"version,omitzero"`
}

// PackConfigSpec defines the desired state of [PackConfig]
type PackConfigSpec struct {
	// Packs specifies the list of packs to be installed.
	//
	// +k8s:required
	Packs []Pack `json:"packs,omitzero"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PackConfig is the schema for the API
type PackConfig struct {
	metav1.TypeMeta `json:",inline"`

	// Spec provides the extension configuration spec.
	//
	// +k8s:required
	Spec PackConfigSpec `json:"spec,omitzero"`
}
