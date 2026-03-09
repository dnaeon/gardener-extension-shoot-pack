// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener-extension-shoot-pack/pkg/apis/config"
)

// Validate validates the given [config.PackConfig]
func Validate(cfg config.PackConfig) error {
	allErrs := make(field.ErrorList, 0)

	if cfg.Spec.Foo == "" {
		allErrs = append(
			allErrs,
			field.Required(field.NewPath("spec.foo"), "empty value specified"),
		)
	}

	// TODO(user): validate any other config setting

	return allErrs.ToAggregate()
}
