// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/resid"

	"github.com/gardener/gardener-extension-shoot-pack/pkg/apis/config"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/apis/config/validation"
)

var _ = Describe("Validation Tests", Ordered, func() {
	It("should fail with an empty config", func() {
		cfg := config.PackConfig{}
		Expect(validation.Validate(cfg)).ShouldNot(Succeed())
	})

	It("should fail with an empty pack name", func() {
		cfg := config.PackConfig{
			Spec: config.PackConfigSpec{
				Packs: []config.Pack{
					{
						// Pack name is missing here
						Version: "42.0",
					},
				},
			},
		}
		Expect(validation.Validate(cfg)).ShouldNot(Succeed())
	})

	It("should fail with an empty pack version", func() {
		cfg := config.PackConfig{
			Spec: config.PackConfigSpec{
				Packs: []config.Pack{
					{
						Name: "foobar",
						// Pack version is missing here
					},
				},
			},
		}
		Expect(validation.Validate(cfg)).ShouldNot(Succeed())
	})

	It("should fail with duplicate packs", func() {
		cfg := config.PackConfig{
			Spec: config.PackConfigSpec{
				Packs: []config.Pack{
					{
						Name:    "foo",
						Version: "v1.2.3",
					},
					{
						Name:    "bar",
						Version: "v1.2.3",
					},
					{
						// A different version of foo is already specified
						Name:    "foo",
						Version: "v2.0.0",
					},
				},
			},
		}
		Expect(validation.Validate(cfg)).ShouldNot(Succeed())
	})

	It("should fail with empty resource refs", func() {
		cfg := config.PackConfig{
			Spec: config.PackConfigSpec{
				Packs: []config.Pack{
					{
						Name:    "foo",
						Version: "v1.2.3",
						Patches: []config.PatchSpec{
							{
								// Empty resource ref
								ResourceRef: "",
							},
						},
					},
				},
			},
		}
		Expect(validation.Validate(cfg)).ShouldNot(Succeed())
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
						Patches: []config.PatchSpec{
							// A dummy patch
							{
								ResourceRef: "my-resource-ref",
								Target: &types.Selector{
									ResId: resid.ResId{
										Gvk: resid.Gvk{
											Group:   "apps",
											Version: "v1",
											Kind:    "Deployment",
										},
										Name: "my-deployment-name",
									},
								},
							},
						},
					},
				},
			},
		}

		err := validation.Validate(cfg)
		Expect(err).NotTo(HaveOccurred())
	})
})
