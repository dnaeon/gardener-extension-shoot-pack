// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kustomize/api/types"
)

// PatchSpec describes a patch for pack resources.
type PatchSpec struct {
	// ResourceRef points to a referenced resource name, which is a secret
	// providing patches for pack resources.
	ResourceRef string

	// Target points to the resources that the patch is applied to.
	Target *types.Selector
}

// Pack describes a pack.
type Pack struct {
	// Name specifies the name of the pack.
	Name string

	// Version specifies the version of the pack.
	Version string

	// Patches specifies a list of optional patches.
	Patches []PatchSpec
}

// PackConfigSpec defines the desired state of [PackConfig]
type PackConfigSpec struct {
	// Packs specifies the list of packs to be installed.
	Packs []Pack
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PackConfig is the schema for the API
type PackConfig struct {
	metav1.TypeMeta

	// Spec provides the extension configuration spec.
	Spec PackConfigSpec
}
