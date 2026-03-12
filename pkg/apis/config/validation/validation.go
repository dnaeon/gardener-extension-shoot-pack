// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener-extension-shoot-pack/pkg/apis/config"
)

// seenPack is used to track packs from the API spec, in order to ensure no
// duplicates have been configured.
type seenPack struct {
	name    string
	version string
	idx     int
}

// Validate validates the given [config.PackConfig]
func Validate(cfg config.PackConfig) error {
	allErrs := make(field.ErrorList, 0)

	if len(cfg.Spec.Packs) == 0 {
		allErrs = append(
			allErrs,
			field.Required(field.NewPath("spec.packs"), "no packs configured"),
		)
	}

	// Validate pack fields
	seenPacks := make([]seenPack, 0, len(cfg.Spec.Packs))
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

		// Ensure that no duplicates exist
		seenIdx := slices.IndexFunc(seenPacks, func(item seenPack) bool {
			return item.name == pack.Name && item.version == pack.Version
		})

		if seenIdx >= 0 {
			allErrs = append(
				allErrs,
				field.Invalid(
					field.NewPath("spec.packs").Index(idx),
					cfg.Spec.Packs[idx],
					fmt.Sprintf("pack already defined at spec.packs[%d]", seenIdx),
				),
			)
		}

		seen := seenPack{
			name:    pack.Name,
			version: pack.Version,
			idx:     idx,
		}
		seenPacks = append(seenPacks, seen)
	}

	return allErrs.ToAggregate()
}
