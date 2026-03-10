// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Pack describes a pack.
type Pack struct {
	// Name specifies the name of the pack.
	Name string

	// Version specifies the version of the pack.
	Version string
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
