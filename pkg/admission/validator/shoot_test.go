// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	packactuator "github.com/gardener/gardener-extension-shoot-pack/pkg/actuator/pack"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/admission/validator"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/apis/config"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/assets"
)

var _ = Describe("Shoot Validator", Ordered, func() {
	var (
		testCollection         *assets.Collection
		testFilesystem         = os.DirFS("testdata/collection")
		ctx                    = context.TODO()
		goodProviderConfigData []byte
		decoder                = serializer.NewCodecFactory(scheme.Scheme, serializer.EnableStrict).UniversalDecoder()
		shootValidator         extensionswebhook.Validator
		shoot                  *gardencore.Shoot
		projectNamespace       = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "garden-local",
			},
		}
		goodProviderConfig = config.PackConfig{
			Spec: config.PackConfigSpec{
				Packs: []config.Pack{
					{
						Name:    "foo",
						Version: "v1.0.0",
					},
					{
						Name:    "bar",
						Version: "v1.0.0",
					},
				},
			},
		}
	)

	BeforeAll(func() {
		var err error
		goodProviderConfigData, err = json.Marshal(goodProviderConfig)
		Expect(err).NotTo(HaveOccurred())

		testCollection, err = assets.New(testFilesystem)
		Expect(err).NotTo(HaveOccurred())
		Expect(testCollection).NotTo(BeNil())
	})

	BeforeEach(func() {
		var err error
		shootValidator, err = validator.NewShootValidator(decoder, validator.WithPackCollection(testCollection))
		Expect(err).NotTo(HaveOccurred())
		shoot = &gardencore.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "local",
				Namespace: projectNamespace.Name,
			},
			Spec: gardencore.ShootSpec{
				SeedName: new("local"),
				Provider: gardencore.Provider{
					Type: "local",
				},
				Region: "local",
				Resources: []gardencore.NamedResourceReference{
					{
						Name: "resource-ref-foo",
						ResourceRef: autoscalingv1.CrossVersionObjectReference{
							Name:       "my-project-secret-foo",
							Kind:       "Secret",
							APIVersion: corev1.SchemeGroupVersion.String(),
						},
					},
					{
						Name: "resource-ref-bar",
						ResourceRef: autoscalingv1.CrossVersionObjectReference{
							Name:       "my-project-secret-bar",
							Kind:       "Secret",
							APIVersion: corev1.SchemeGroupVersion.String(),
						},
					},
				},
			},
		}
	})

	It("IgnoreNotFound should ignore ErrExtensionNotFound errors", func() {
		Expect(validator.IgnoreExtensionNotFound(validator.ErrExtensionNotFound)).To(Succeed())
		Expect(validator.IgnoreExtensionNotFound(errors.New("an error"))).To(MatchError(ContainSubstring("an error")))
	})

	It("should successfully validate good provider config", func() {
		// Ensure we have the extension enabled with proper provider config
		shoot.Spec.Extensions = []gardencore.Extension{
			{
				Type: packactuator.ExtensionType,
				ProviderConfig: &runtime.RawExtension{
					Raw: goodProviderConfigData,
				},
			},
		}
		Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
	})

	It("should successfully validate if shoot is being deleted", func() {
		shoot.DeletionTimestamp = new(metav1.NewTime(time.Now()))
		shoot.Spec.Extensions = []gardencore.Extension{
			{
				Type: packactuator.ExtensionType,
				// ProviderConfig is empty on purpose here
			},
		}
		Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
	})

	It("should successfully validate when extension is not defined", func() {
		shoot.Spec.Extensions = []gardencore.Extension{}
		Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
	})

	It("should successfully validate when extension is disabled", func() {
		shoot.Spec.Extensions = []gardencore.Extension{
			{
				// Extension is explicitly disabled
				Disabled: new(true),
				Type:     packactuator.ExtensionType,
				ProviderConfig: &runtime.RawExtension{
					Raw: goodProviderConfigData,
				},
			},
		}
		Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
	})

	It("should successfully validate patches in referenced resources", func() {
		providerConfig := config.PackConfig{
			Spec: config.PackConfigSpec{
				Packs: []config.Pack{
					{
						Name:    "foo",
						Version: "v1.0.0",
						Patches: []config.PatchSpec{
							{
								ResourceRef: "resource-ref-foo",
							},
							{
								ResourceRef: "resource-ref-bar",
							},
						},
					},
				},
			},
		}
		data, err := json.Marshal(providerConfig)
		Expect(err).NotTo(HaveOccurred())

		shoot.Spec.Extensions = []gardencore.Extension{
			{
				Type: packactuator.ExtensionType,
				ProviderConfig: &runtime.RawExtension{
					Raw: data,
				},
			},
		}
		Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
	})

	It("should fail to validate missing referenced resources in patches", func() {
		providerConfig := config.PackConfig{
			Spec: config.PackConfigSpec{
				Packs: []config.Pack{
					{
						Name:    "foo",
						Version: "v1.0.0",
						Patches: []config.PatchSpec{
							{
								ResourceRef: "no-such-resource-ref",
							},
						},
					},
				},
			},
		}
		data, err := json.Marshal(providerConfig)
		Expect(err).NotTo(HaveOccurred())

		shoot.Spec.Extensions = []gardencore.Extension{
			{
				Type: packactuator.ExtensionType,
				ProviderConfig: &runtime.RawExtension{
					Raw: data,
				},
			},
		}
		Expect(shootValidator.Validate(ctx, shoot, nil)).NotTo(Succeed())
	})

	It("should fail to validate example providerConfig with builtin collection", func() {
		// We are not using validator.WithPackCollection here, which
		// will cause the validator to fallback to the builtin pack
		// collection, which does not provide the test `foo' and `bar'
		// packs.
		shootValidator, err := validator.NewShootValidator(decoder)
		Expect(err).NotTo(HaveOccurred())

		shoot.Spec.Extensions = []gardencore.Extension{
			{
				Type: packactuator.ExtensionType,
				ProviderConfig: &runtime.RawExtension{
					Raw: goodProviderConfigData,
				},
			},
		}

		Expect(shootValidator.Validate(ctx, shoot, nil)).NotTo(Succeed())
	})

	It("should fail to create shoot validator with invalid decoder", func() {
		_, err := validator.NewShootValidator(nil)
		Expect(err).To(MatchError(ContainSubstring("invalid decoder specified")))
	})

	It("should fail to validate when extension provider config is not defined", func() {
		shoot.Spec.Extensions = []gardencore.Extension{
			{
				Type: packactuator.ExtensionType,
				// ProviderConfig is not specified
			},
		}
		err := shootValidator.Validate(ctx, shoot, nil)
		Expect(err).To(MatchError(ContainSubstring("no provider config specified")))
	})

	It("should fail to validate because of unknown pack", func() {
		providerConfig := config.PackConfig{
			Spec: config.PackConfigSpec{
				Packs: []config.Pack{
					{
						Name:    "no-such-pack-name",
						Version: "v1.0.0",
					},
					{
						Name:    "bar",
						Version: "v1.0.0",
					},
				},
			},
		}
		data, err := json.Marshal(providerConfig)
		Expect(err).NotTo(HaveOccurred())

		shoot.Spec.Extensions = []gardencore.Extension{
			{
				Type: packactuator.ExtensionType,
				ProviderConfig: &runtime.RawExtension{
					Raw: data,
				},
			},
		}

		err = shootValidator.Validate(ctx, shoot, nil)
		Expect(err).To(MatchError(ContainSubstring("pack no-such-pack-name@v1.0.0 does not exist")))
	})

	It("should fail to validate with nil shoot", func() {
		err := shootValidator.Validate(ctx, nil, nil)
		Expect(err).To(MatchError(ContainSubstring("invalid object type")))
	})

	It("should fail to validate with empty packs", func() {
		providerConfig := config.PackConfig{
			Spec: config.PackConfigSpec{
				// No packs have been configured
				Packs: []config.Pack{},
			},
		}
		data, err := json.Marshal(providerConfig)
		Expect(err).NotTo(HaveOccurred())

		shoot.Spec.Extensions = []gardencore.Extension{
			{
				Type: packactuator.ExtensionType,
				ProviderConfig: &runtime.RawExtension{
					Raw: data,
				},
			},
		}

		err = shootValidator.Validate(ctx, shoot, nil)
		Expect(err).To(MatchError(ContainSubstring("invalid extension configuration")))
	})

	It("should fail to validate with bad provider config data", func() {
		shoot.Spec.Extensions = []gardencore.Extension{
			{
				Type: packactuator.ExtensionType,
				ProviderConfig: &runtime.RawExtension{
					Raw: []byte("foo\nbar"),
				},
			},
		}

		err := shootValidator.Validate(ctx, shoot, nil)
		Expect(err).To(MatchError(ContainSubstring("invalid provider spec configuration")))
	})
})
