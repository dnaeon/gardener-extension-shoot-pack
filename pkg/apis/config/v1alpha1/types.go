// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PackConfigSpec defines the desired state of [PackConfig]
type PackConfigSpec struct {
	// Foo is foo
	Foo string `json:"foo,omitzero"`

	// TODO(user): insert additional spec fields
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PackConfig is the schema for the API
type PackConfig struct {
	metav1.TypeMeta `json:",inline"`

	// Spec provides the extension configuration spec.
	Spec PackConfigSpec `json:"spec,omitzero"`
}
