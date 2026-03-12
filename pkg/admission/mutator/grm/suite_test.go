// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package grm_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGRMMutator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GRM Mutator Webhook Suite")
}
