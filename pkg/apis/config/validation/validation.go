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

	if len(cfg.Spec.Packs) == 0 {
		allErrs = append(
			allErrs,
			field.Required(field.NewPath("spec.packs"), "no packs specified"),
		)
	}

	for idx, pack := range cfg.Spec.Packs {
		if pack.Name == "" {
			allErrs = append(
				allErrs,
				field.Required(field.NewPath("spec.packs").Index(idx).Child("name"), "empty pack name"),
			)
		}
		if pack.Version == "" {
			allErrs = append(
				allErrs,
				field.Required(field.NewPath("spec.packs").Index(idx).Child("version"), "empty pack version"),
			)
		}
	}

	return allErrs.ToAggregate()
}
