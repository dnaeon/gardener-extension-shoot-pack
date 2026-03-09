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
	It("should detect invalid config", func() {
		cfg := config.PackConfig{}
		err := validation.Validate(cfg)
		Expect(err).Should(HaveOccurred())
	})

	It("should successfully validate correct config", func() {
		cfg := config.PackConfig{
			Spec: config.PackConfigSpec{
				Foo: "bar",
			},
		}
		err := validation.Validate(cfg)
		Expect(err).NotTo(HaveOccurred())
	})

	// TODO(user): additional tests
})
