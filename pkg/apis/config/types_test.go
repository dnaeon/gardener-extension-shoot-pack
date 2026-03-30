// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener-extension-shoot-pack/pkg/apis/config"
)

var _ = Describe("API Tests", Ordered, func() {
	It("should return proper fmt.Stringer value", func() {
		pack := config.Pack{
			Name:    "foo",
			Version: "v1.2.3",
		}
		Expect(pack.String()).To(Equal("foo@v1.2.3"))
	})
})
