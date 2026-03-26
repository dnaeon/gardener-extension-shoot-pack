// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kustomize/api/types"
)

// ResourceReference references a resource in the Garden cluster.
type ResourceReference struct {
	// Name specifies the name of the referenced resource from the shoot spec.
	//
	// +k8s:required
	Name string `json:"name,omitzero"`
}

// PatchSpec describes a patch for pack resources.
type PatchSpec struct {
	// ResourceRef points to a referenced resource name, which is a secret
	// providing patches for pack resources.
	//
	// +k8s:required
	ResourceRef ResourceReference `json:"resourceRef,omitzero"`

	// Target points to the resources that the patch is applied to.
	//
	// +k8s:optional
	Target *types.Selector `json:"target,omitzero"`
}

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

	// Patches specifies a list of optional patches.
	//
	// +k8s:optional
	Patches []PatchSpec `json:"patches,omitzero"`
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
