// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener-extension-shoot-pack/pkg/apis/config"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/apis/config/validation"
)

var _ = Describe("Validation Tests", Ordered, func() {
	It("should fail on empty config", func() {
		cfg := config.PackConfig{}
		err := validation.Validate(cfg)
		Expect(err).Should(HaveOccurred())
	})

	It("should fail with an empty pack name", func() {
		cfg := config.PackConfig{
			Spec: config.PackConfigSpec{
				Packs: []config.Pack{
					{
						Version: "42.0",
					},
				},
			},
		}
		err := validation.Validate(cfg)
		Expect(err).Should(HaveOccurred())
	})

	It("should fail with an empty pack version", func() {
		cfg := config.PackConfig{
			Spec: config.PackConfigSpec{
				Packs: []config.Pack{
					{
						Name: "foobar",
					},
				},
			},
		}
		err := validation.Validate(cfg)
		Expect(err).Should(HaveOccurred())
	})

	It("should successfully validate extension config", func() {
		cfg := config.PackConfig{
			Spec: config.PackConfigSpec{
				Packs: []config.Pack{
					{
						Name:    "foo",
						Version: "1.2.3",
					},
					{
						Name:    "bar",
						Version: "42.0",
					},
				},
			},
		}

		err := validation.Validate(cfg)
		Expect(err).NotTo(HaveOccurred())
	})
})
