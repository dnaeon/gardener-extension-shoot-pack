// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pack_test

import (
	"encoding/json"
	"os"

	"github.com/gardener/gardener/extensions/pkg/controller/extension"
	corev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/component-base/featuregate"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	packactuator "github.com/gardener/gardener-extension-shoot-pack/pkg/actuator/pack"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/apis/config"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/assets"
)

var _ = Describe("Actuator", Ordered, func() {
	var (
		testActuator                                                  extension.Actuator
		testCollection                                                *assets.Collection
		testFilesystem                                                = os.DirFS("testdata/collection")
		refResourceFooName                                            = "resource-foo"
		refResourceBarName                                            = "resource-bar"
		refResourceQuxName                                            = "resource-qux"
		goodPatchSecretForPackFoo                                     *corev1.Secret
		badPatchSecretForPackBar                                      *corev1.Secret
		goodProviderConfigData, cloudProfileData, seedData, shootData []byte

		extResource *extensionsv1alpha1.Extension
		cluster     *extensionsv1alpha1.Cluster
		decoder     = serializer.NewCodecFactory(scheme.Scheme, serializer.EnableStrict).UniversalDecoder()

		featureGates       = make(map[featuregate.Feature]bool)
		actuatorOpts       []packactuator.Option
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

		projectNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "garden-local",
			},
		}
		shootNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "shoot--local--local",
			},
		}
		cloudProfile = &corev1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: "local",
			},
			Spec: corev1beta1.CloudProfileSpec{
				Type: "local",
			},
		}
		seed = &corev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: "local",
			},
			Spec: corev1beta1.SeedSpec{
				Ingress: &corev1beta1.Ingress{
					Domain: "ingress.local.seed.local.gardener.cloud",
				},
				Provider: corev1beta1.SeedProvider{
					Type:   "local",
					Region: "local",
					Zones:  []string{"0"},
				},
			},
		}
		shoot = &corev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "local",
				Namespace: projectNamespace.Name,
			},
			Spec: corev1beta1.ShootSpec{
				SeedName: new("local"),
				Provider: corev1beta1.Provider{
					Type: "local",
				},
				Region: "local",
				Resources: []corev1beta1.NamedResourceReference{
					{
						Name: refResourceFooName,
						ResourceRef: autoscalingv1.CrossVersionObjectReference{
							Name:       "my-project-secret-foo",
							Kind:       "Secret",
							APIVersion: corev1.SchemeGroupVersion.String(),
						},
					},
					{
						Name: refResourceBarName,
						ResourceRef: autoscalingv1.CrossVersionObjectReference{
							Name:       "my-project-secret-bar",
							Kind:       "Secret",
							APIVersion: corev1.SchemeGroupVersion.String(),
						},
					},
					{
						Name: refResourceQuxName,
						ResourceRef: autoscalingv1.CrossVersionObjectReference{
							Name:       "my-project-secret-qux",
							Kind:       "Secret",
							APIVersion: corev1.SchemeGroupVersion.String(),
						},
					},
				},
			},
		}
	)

	BeforeAll(func() {
		var err error
		testCollection, err = assets.New(testFilesystem)
		Expect(err).NotTo(HaveOccurred())
		Expect(testCollection).NotTo(BeNil())

		actuatorOpts = []packactuator.Option{
			packactuator.WithGardenerVersion("1.0.0"),
			packactuator.WithDecoder(decoder),
			packactuator.WithGardenletFeatures(featureGates),
			packactuator.WithPackCollection(testCollection),
		}

		// Serialize our test objects, so we can later re-use them.
		cloudProfileData, err = json.Marshal(cloudProfile)
		Expect(err).NotTo(HaveOccurred())
		seedData, err = json.Marshal(seed)
		Expect(err).NotTo(HaveOccurred())
		shootData, err = json.Marshal(shoot)
		Expect(err).NotTo(HaveOccurred())
		goodProviderConfigData, err = json.Marshal(goodProviderConfig)
		Expect(err).NotTo(HaveOccurred())

		// Create test actuator
		testActuator, err = packactuator.New(k8sClient, actuatorOpts...)
		Expect(err).NotTo(HaveOccurred())
		Expect(testActuator).NotTo(BeNil())

		// Create project and shoot namespace
		Expect(k8sClient.Create(ctx, projectNamespace)).To(Succeed())
		Expect(k8sClient.Create(ctx, shootNamespace)).To(Succeed())

		// Create a secret, which provides a patch for pack `foo'.  The
		// patch contained within the secret adds additional labels to
		// the configmap resource provided by the pack.
		patchDataFooPack := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo-configmap
  labels:
    label.foo: bar
    label.bar: baz
`
		goodPatchSecretForPackFoo = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.ReferencedResourcesPrefix + refResourceFooName,
				Namespace: shootNamespace.Name,
			},
			StringData: map[string]string{
				"configmap-patch.yaml": patchDataFooPack,
			},
			Type: corev1.SecretTypeOpaque,
		}

		Expect(k8sClient.Create(ctx, goodPatchSecretForPackFoo)).To(Succeed())

		// Create a secret, which contains a patch for pack `bar', which
		// is invalid, so that we can test that reconciliation with
		// invalid patches fails.
		badPatchSecretForPackBar = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.ReferencedResourcesPrefix + refResourceBarName,
				Namespace: shootNamespace.Name,
			},
			StringData: map[string]string{
				"configmap-patch-1.yaml": "invalid-patch-data",
				"configmap-patch-2.yaml": "yet-another-invalid-data",
			},
			Type: corev1.SecretTypeOpaque,
		}

		Expect(k8sClient.Create(ctx, badPatchSecretForPackBar)).To(Succeed())
	})

	BeforeEach(func() {
		extResource = &extensionsv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-pack",
				Namespace: shootNamespace.Name,
			},
			Spec: extensionsv1alpha1.ExtensionSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type:  packactuator.ExtensionType,
					Class: ptr.To(extensionsv1alpha1.ExtensionClassShoot),
				},
			},
		}

		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: shootNamespace.Name,
			},
			Spec: extensionsv1alpha1.ClusterSpec{
				CloudProfile: runtime.RawExtension{
					Raw: cloudProfileData,
				},
				Seed: runtime.RawExtension{
					Raw: seedData,
				},
				Shoot: runtime.RawExtension{
					Raw: shootData,
				},
			},
		}

		Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
	})

	It("should successfully create an actuator", func() {
		act, err := packactuator.New(k8sClient, actuatorOpts...)

		Expect(err).NotTo(HaveOccurred())
		Expect(act).NotTo(BeNil())
		Expect(act.Name()).To(Equal(packactuator.Name))
		Expect(act.ExtensionType()).To(Equal(packactuator.ExtensionType))
		Expect(act.FinalizerSuffix()).To(Equal(packactuator.FinalizerSuffix))
		Expect(act.ExtensionClass()).To(Equal(extensionsv1alpha1.ExtensionClassShoot))
	})

	It("should successfully create an actuator with no options", func() {
		act, err := packactuator.New(k8sClient)
		Expect(err).NotTo(HaveOccurred())
		Expect(act).NotTo(BeNil())
		Expect(act.Name()).To(Equal(packactuator.Name))
		Expect(act.ExtensionType()).To(Equal(packactuator.ExtensionType))
		Expect(act.FinalizerSuffix()).To(Equal(packactuator.FinalizerSuffix))
		Expect(act.ExtensionClass()).To(Equal(extensionsv1alpha1.ExtensionClassShoot))
	})

	It("should fail to create actuator with nil client", func() {
		act, err := packactuator.New(nil, actuatorOpts...)
		Expect(err).To(HaveOccurred())
		Expect(act).To(BeNil())
	})

	It("should fail to reconcile when no cluster exists", func() {
		// Change namespace of the extension resource, so that a
		// non-existing cluster is looked up.
		extResource.Namespace = "non-existing-namespace"

		err := testActuator.Reconcile(ctx, logger, extResource)
		Expect(err).Should(HaveOccurred())
		Expect(err).To(MatchError(ContainSubstring("failed to get cluster")))
	})

	It("should fail to reconcile without provider config", func() {
		err := testActuator.Reconcile(ctx, logger, extResource)
		Expect(err).Should(HaveOccurred())
		Expect(err).To(MatchError(ContainSubstring("no provider config specified")))
	})

	It("should successfully reconcile with good provider config", func() {
		// Ensure we have valid provider config
		extResource.Spec.ProviderConfig = &runtime.RawExtension{
			Raw: goodProviderConfigData,
		}

		Expect(testActuator.Reconcile(ctx, logger, extResource)).To(Succeed())

		// Ensure that the managed resources have been created.  The
		// default provider config includes the `foo' and `bar' packs,
		// so we expect to find managed resources for each.
		wantManagedResources := []string{
			"extension-shoot-pack-foo",
			"extension-shoot-pack-bar",
		}
		for _, mrName := range wantManagedResources {
			var mr resourcesv1alpha1.ManagedResource
			objKey := client.ObjectKey{
				Name:      mrName,
				Namespace: shootNamespace.Name,
			}

			Expect(k8sClient.Get(ctx, objKey, &mr)).To(Succeed())
		}
	})

	It("should successfully reconcile with a single pack and delete unmanaged packs", func() {
		// The test collection contains two packs. This test reconciles
		// with a single pack only, which should result in deletion of
		// the ManagedResources for unmanaged packs.
		providerConfig := config.PackConfig{
			Spec: config.PackConfigSpec{
				Packs: []config.Pack{
					{
						Name:    "foo",
						Version: "v1.0.0",
					},
				},
			},
		}
		data, err := json.Marshal(providerConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).NotTo(BeNil())

		extResource.Spec.ProviderConfig = &runtime.RawExtension{
			Raw: data,
		}

		Expect(testActuator.Reconcile(ctx, logger, extResource)).To(Succeed())

		// Ensure that the managed resources have been created.  The
		// provider config we use in this test includes only the `foo' pack.
		//
		// Since `bar' was not specified a Managed Resource for it
		// should not exist.
		wantManagedResources := []string{
			"extension-shoot-pack-foo",
		}
		for _, mrName := range wantManagedResources {
			var mr resourcesv1alpha1.ManagedResource
			objKey := client.ObjectKey{
				Name:      mrName,
				Namespace: shootNamespace.Name,
			}

			Expect(k8sClient.Get(ctx, objKey, &mr)).To(Succeed())
		}

		// The managed resource for `bar' pack should not exist.
		dontWantManagedResources := []string{
			"extension-shoot-pack-bar",
		}
		for _, mrName := range dontWantManagedResources {
			var mr resourcesv1alpha1.ManagedResource
			objKey := client.ObjectKey{
				Name:      mrName,
				Namespace: shootNamespace.Name,
			}

			err := k8sClient.Get(ctx, objKey, &mr)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}
	})

	It("should successfully reconcile a pack with patches", func() {
		providerConfig := config.PackConfig{
			Spec: config.PackConfigSpec{
				Packs: []config.Pack{
					{
						Name:    "foo",
						Version: "v1.0.0",
						Patches: []config.PatchSpec{
							{
								ResourceRef: refResourceFooName,
							},
						},
					},
				},
			},
		}
		data, err := json.Marshal(providerConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).NotTo(BeNil())

		extResource.Spec.ProviderConfig = &runtime.RawExtension{
			Raw: data,
		}

		Expect(testActuator.Reconcile(ctx, logger, extResource)).To(Succeed())

		var mr resourcesv1alpha1.ManagedResource
		objKey := client.ObjectKey{
			Name:      "extension-shoot-pack-foo",
			Namespace: shootNamespace.Name,
		}

		Expect(k8sClient.Get(ctx, objKey, &mr)).To(Succeed())
	})

	It("should fail to reconcile a pack with invalid patches", func() {
		providerConfig := config.PackConfig{
			Spec: config.PackConfigSpec{
				// The secret which contains the patch for `bar'
				// pack comes with invalid patch data.
				Packs: []config.Pack{
					{
						Name:    "bar",
						Version: "v1.0.0",

						Patches: []config.PatchSpec{
							{
								ResourceRef: refResourceBarName,
							},
						},
					},
				},
			},
		}
		data, err := json.Marshal(providerConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).NotTo(BeNil())

		extResource.Spec.ProviderConfig = &runtime.RawExtension{
			Raw: data,
		}

		err = testActuator.Reconcile(ctx, logger, extResource)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(ContainSubstring("trouble configuring builtin PatchTransformer with config")))
	})

	It("should fail to reconcile a pack with missing patch secrets", func() {
		providerConfig := config.PackConfig{
			Spec: config.PackConfigSpec{
				// The secret for [refResourceQuxName] does not exist.
				Packs: []config.Pack{
					{
						Name:    "foo",
						Version: "v1.0.0",

						Patches: []config.PatchSpec{
							{
								ResourceRef: refResourceQuxName,
							},
						},
					},
				},
			},
		}
		data, err := json.Marshal(providerConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).NotTo(BeNil())

		extResource.Spec.ProviderConfig = &runtime.RawExtension{
			Raw: data,
		}

		err = testActuator.Reconcile(ctx, logger, extResource)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(ContainSubstring("unable to get patch from referenced resource resource-qux")))
	})

	It("should successfully reconcile a hibernated shoot", func() {
		// Re-create test cluster and mark the shoot as hibernated
		Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
		shoot.Spec.Hibernation = &corev1beta1.Hibernation{Enabled: new(true)}
		hibernatedShoot, err := json.Marshal(shoot)
		Expect(err).NotTo(HaveOccurred())
		Expect(shootData).NotTo(BeNil())

		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: shootNamespace.Name,
			},
			Spec: extensionsv1alpha1.ClusterSpec{
				CloudProfile: runtime.RawExtension{
					Raw: cloudProfileData,
				},
				Seed: runtime.RawExtension{
					Raw: seedData,
				},
				Shoot: runtime.RawExtension{
					Raw: hibernatedShoot,
				},
			},
		}
		Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

		// No provider config is specified here
		extResource.Spec.ProviderConfig = &runtime.RawExtension{}
		Expect(testActuator.Reconcile(ctx, logger, extResource)).To(Succeed())
	})

	It("should fail to reconcile with bad provider config", func() {
		// Specify invalid provider config
		extResource.Spec.ProviderConfig = &runtime.RawExtension{
			Raw: []byte("foo\nbar"),
		}
		Expect(testActuator.Reconcile(ctx, logger, extResource)).NotTo(Succeed())
	})

	It("should fail to reconcile with unknown pack", func() {
		// Pack spec specifies a non-existing pack
		providerConfig := config.PackConfig{
			Spec: config.PackConfigSpec{
				Packs: []config.Pack{
					{
						Name:    "no-such-pack",
						Version: "v1.0.0",
					},
				},
			},
		}
		data, err := json.Marshal(providerConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).NotTo(BeNil())

		extResource.Spec.ProviderConfig = &runtime.RawExtension{
			Raw: data,
		}

		Expect(testActuator.Reconcile(ctx, logger, extResource)).NotTo(Succeed())
	})

	It("should fail to reconcile with invalid pack spec", func() {
		// Pack spec specifies a non-existing pack
		providerConfig := config.PackConfig{
			Spec: config.PackConfigSpec{
				Packs: []config.Pack{
					{
						Name: "foo",
						// Version is empty
					},
				},
			},
		}
		data, err := json.Marshal(providerConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).NotTo(BeNil())

		extResource.Spec.ProviderConfig = &runtime.RawExtension{
			Raw: data,
		}

		Expect(testActuator.Reconcile(ctx, logger, extResource)).NotTo(Succeed())
	})

	It("should succeed on Delete", func() {
		Expect(testActuator.Delete(ctx, logger, extResource)).To(Succeed())
	})

	It("should succeed on ForceDelete", func() {
		Expect(testActuator.ForceDelete(ctx, logger, extResource)).To(Succeed())
	})

	It("should succeed on Restore", func() {
		// Ensure we have valid provider config
		extResource.Spec.ProviderConfig = &runtime.RawExtension{
			Raw: goodProviderConfigData,
		}
		Expect(testActuator.Restore(ctx, logger, extResource)).To(Succeed())
	})

	It("should succeed on Migrate", func() {
		// Ensure we have valid provider config
		extResource.Spec.ProviderConfig = &runtime.RawExtension{
			Raw: goodProviderConfigData,
		}

		Expect(testActuator.Migrate(ctx, logger, extResource)).To(Succeed())
	})
})
