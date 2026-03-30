// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package actuator provides the implementation of a Gardener extension
// actuator.
package pack

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/extension"
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/component-base/featuregate"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/kustomize/kyaml/yaml"

	"github.com/gardener/gardener-extension-shoot-pack/pkg/apis/config"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/apis/config/validation"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/assets"
)

const (
	// PackLabelPrefix is the prefix for labels used by the extension.
	PackLabelPrefix = "pack.extensions.gardener.cloud/"
	// PackNameLabel is the label added to pack resources, which specifies
	// the name of the pack from which resources originate from.
	PackNameLabel = PackLabelPrefix + "name"
	// PackVersionLabel is the label added to pack resources, which
	// specifies the version of the pack from which resources originate
	// from.
	PackVersionLabel = PackLabelPrefix + "version"
)

// ErrInvalidActuator is an error, which is returned when creating an [Actuator]
// with invalid config settings.
var ErrInvalidActuator = errors.New("invalid actuator")

// ErrInvalidObject is an error, which is returned when an invalid object was
// specified, e.g. an empty or nil object.
var ErrInvalidObject = errors.New("invalid object specified")

const (
	// Name is the name of the actuator
	Name = "shoot-pack"
	// ExtensionType is the type of the extension resources, which the
	// actuator reconciles.
	ExtensionType = "shoot-pack"
	// FinalizerSuffix is the finalizer suffix used by the actuator
	FinalizerSuffix = "gardener-extension-shoot-pack"
	// ManagedResourcePrefix is the prefix of the ManagedResources created
	// by the extension controller.
	ManagedResourcePrefix = "extension-shoot-pack"
	// ManagedResourceOrigin is the origin of the ManagedResources created
	// by the extension controller.
	ManagedResourceOrigin = "gardener-extension-shoot-pack"
)

// Actuator is an implementation of [extension.Actuator].
type Actuator struct {
	client          client.Client
	decoder         runtime.Decoder
	resourceFactory *resource.Factory
	collection      *assets.Collection

	// The following fields are usually derived from the list of extra Helm
	// values provided by gardenlet during the deployment of the extension.
	//
	// See the link below for more details about how gardenlet provides
	// extra values to Helm during the extension deployment.
	//
	// https://github.com/gardener/gardener/blob/d5071c800378616eb6bb2c7662b4b28f4cfe7406/pkg/gardenlet/controller/controllerinstallation/controllerinstallation/reconciler.go#L236-L263
	gardenerVersion       string
	gardenletFeatureGates map[featuregate.Feature]bool
}

var _ extension.Actuator = &Actuator{}

// Option is a function, which configures the [Actuator].
type Option func(a *Actuator) error

// New creates a new actuator with the given options.
func New(c client.Client, opts ...Option) (*Actuator, error) {
	if c == nil {
		return nil, fmt.Errorf("%w: no client specified", ErrInvalidActuator)
	}

	act := &Actuator{
		client:                c,
		resourceFactory:       provider.NewDefaultDepProvider().GetResourceFactory(),
		gardenletFeatureGates: make(map[featuregate.Feature]bool),
	}

	for _, opt := range opts {
		if err := opt(act); err != nil {
			return nil, err
		}
	}

	if act.decoder == nil {
		act.decoder = serializer.NewCodecFactory(c.Scheme(), serializer.EnableStrict).UniversalDecoder()
	}

	if act.collection == nil {
		collection, err := assets.New(assets.FS, assets.WithSkipVerify(false))
		if err != nil {
			return nil, fmt.Errorf("unable to create pack collection: %w", err)
		}
		act.collection = collection
	}

	return act, nil
}

// WithDecoder is an [Option], which configures the [Actuator] with the given
// [runtime.Decoder].
func WithDecoder(d runtime.Decoder) Option {
	opt := func(a *Actuator) error {
		a.decoder = d

		return nil
	}

	return opt
}

// WithGardenerVersion is an [Option], which configures the [Actuator] with the
// given version of Gardener. This version of Gardener is usually provided by
// the gardenlet as part of the extra Helm values during deployment of the
// extension.
func WithGardenerVersion(v string) Option {
	opt := func(a *Actuator) error {
		a.gardenerVersion = v

		return nil
	}

	return opt
}

// WithGardenletFeatures is an [Option], which configures the [Actuator] with
// the given gardenlet feature gates. These feature gates are usually provided
// by the gardenlet as part of the extra Helm values during deployment of the
// extension.
func WithGardenletFeatures(feats map[featuregate.Feature]bool) Option {
	opt := func(a *Actuator) error {
		a.gardenletFeatureGates = feats

		return nil
	}

	return opt
}

// WithPackCollection is an [Option], which configures the [Actuator] with the
// given [assets.Collection]. Packs will be reconciled based on the provided
// [assets.Collection].
func WithPackCollection(collection *assets.Collection) Option {
	opt := func(a *Actuator) error {
		a.collection = collection

		return nil
	}

	return opt
}

// Name returns the name of the actuator. This name can be used when registering
// a controller for the actuator.
func (a *Actuator) Name() string {
	return Name
}

// FinalizerSuffix returns the finalizer suffix to use for the actuator. The
// result of this method may be used when registering a controller with the
// actuator.
func (a *Actuator) FinalizerSuffix() string {
	return FinalizerSuffix
}

// ExtensionType returns the type of extension resources the actuator
// reconciles. The result of this method may be used when registering a
// controller with the actuator.
func (a *Actuator) ExtensionType() string {
	return ExtensionType
}

// ExtensionClass returns the [extensionsv1alpha1.ExtensionClass] for the
// actuator. The result of this method may be used when registering a controller
// with the actuator.
func (a *Actuator) ExtensionClass() extensionsv1alpha1.ExtensionClass {
	return extensionsv1alpha1.ExtensionClassShoot
}

// Reconcile reconciles the [extensionsv1alpha1.Extension] resource by taking
// care of any resources managed by the [Actuator]. This method implements the
// [extension.Actuator] interface.
func (a *Actuator) Reconcile(ctx context.Context, logger logr.Logger, ex *extensionsv1alpha1.Extension) error {
	// The cluster name is the same as the name of the namespace for our
	// [extensionsv1alpha1.Extension] resource.
	clusterName := ex.Namespace

	logger.Info("reconciling extension", "name", ex.Name, "cluster", clusterName)

	cluster, err := extensionscontroller.GetCluster(ctx, a.client, clusterName)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	// Nothing to do here, if the shoot cluster is hibernated at the moment.
	if v1beta1helper.HibernationIsEnabled(cluster.Shoot) {
		return nil
	}

	// Parse and validate the provider config
	if ex.Spec.ProviderConfig == nil {
		return errors.New("no provider config specified")
	}

	// Decode provider spec configuration into our known config type.
	var cfg config.PackConfig
	if err := runtime.DecodeInto(a.decoder, ex.Spec.ProviderConfig.Raw, &cfg); err != nil {
		return fmt.Errorf("invalid provider spec configuration: %w", err)
	}

	if err := validation.Validate(cfg); err != nil {
		return err
	}

	// Each pack gets its own managed resource
	enabledPacks := make([]config.Pack, 0)
	disabledPacks := make([]config.Pack, 0)
	for _, packSpec := range cfg.Spec.Packs {
		if !a.collection.PackExists(packSpec.Name, packSpec.Version) {
			return fmt.Errorf("pack %s does not exist", packSpec.String())
		}

		packAsset, err := a.collection.GetPack(packSpec.Name, packSpec.Version)
		if err != nil {
			return fmt.Errorf("unable to get pack %s: %w", packSpec.String(), err)
		}

		logger.Info(
			"Reconciling pack",
			"pack_name", packSpec.Name,
			"pack_version", packSpec.Version,
		)
		if err := a.reconcilePack(ctx, ex.Namespace, cluster, packSpec, packAsset); err != nil {
			return fmt.Errorf("unable to reconcile %s: %w", packSpec.String(), err)
		}
		enabledPacks = append(enabledPacks, packSpec)
	}

	// Delete ManagedResources for packs, which have not been configured.
	for _, pack := range a.collection.Packs {
		if !slices.ContainsFunc(enabledPacks, func(item config.Pack) bool {
			return item.Name == pack.Name && item.Version == pack.Version
		}) {
			disabledPacks = append(disabledPacks, config.Pack{Name: pack.Name, Version: pack.Version})
		}
	}

	for _, disabledPack := range disabledPacks {
		mrName := fmt.Sprintf("%s-%s", ManagedResourcePrefix, disabledPack.Name)
		if err := managedresources.DeleteForShoot(ctx, a.client, ex.Namespace, mrName); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}

// reconcilePack reconciles the given pack.
func (a *Actuator) reconcilePack(ctx context.Context, shootNamespace string, cluster *extensions.Cluster, packSpec config.Pack, packAsset *assets.Pack) error {
	if cluster == nil {
		return fmt.Errorf("%w: cluster is nil", ErrInvalidObject)
	}
	if packAsset == nil {
		return fmt.Errorf("%w: pack asset is nil", ErrInvalidObject)
	}

	// Create a kustomization filesystem for the pack resources
	fs, err := a.kustomizeFilesystemForPack(ctx, shootNamespace, cluster, packSpec, packAsset)
	if err != nil {
		return fmt.Errorf("unable to create kustomize filesystem for %s: %w", packSpec.String(), err)
	}

	// Build the resources
	kustomizer := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	resMap, err := kustomizer.Run(fs, packAsset.BaseDir)
	if err != nil {
		return fmt.Errorf("unable to render kustomization for %s: %w", packSpec.String(), err)
	}

	// Register resources with the Managed Resources registry
	registry := managedresources.NewRegistry(
		kubernetes.ShootScheme,
		kubernetes.ShootCodec,
		kubernetes.ShootSerializer,
	)
	for idx, r := range resMap.Resources() {
		name := r.GetName()
		gvk := r.GetGvk()

		// Skip Namespace resources, since whatever we deploy goes into
		// [metav1.NamespaceSystem].
		if gvk.Kind == "Namespace" {
			continue
		}

		// Add pack labels
		labels := r.GetLabels()
		extraLabels := map[string]string{
			PackNameLabel:    packAsset.Name,
			PackVersionLabel: packAsset.Version,
		}
		maps.Copy(labels, extraLabels)
		if err := r.SetLabels(labels); err != nil {
			return fmt.Errorf("unable to set labels for %s %s: %w", name, gvk.String(), err)
		}

		data, err := r.AsYAML()
		if err != nil {
			return fmt.Errorf("unable to marshal %s %s: %w", name, gvk.String(), err)
		}

		registry.AddSerialized(
			filepath.Join(packAsset.BaseDir, strconv.Itoa(idx)),
			data,
		)
	}

	data, err := registry.SerializedObjects()
	if err != nil {
		return fmt.Errorf("unable to get registry resources: %w", err)
	}

	mrName := fmt.Sprintf("%s-%s", ManagedResourcePrefix, packAsset.Name)

	return managedresources.CreateForShoot(
		ctx,
		a.client,
		shootNamespace,
		mrName,
		ManagedResourceOrigin,
		false,
		data,
	)
}

// kustomizeFilesystemForPack creates a [filesys.FileSystem], which contains the
// resources from the given [assets.Pack] and any patches, which have been
// specified as part of the [config.Pack] spec.
func (a *Actuator) kustomizeFilesystemForPack(ctx context.Context, shootNamespace string, cluster *extensions.Cluster, packSpec config.Pack, packAsset *assets.Pack) (filesys.FileSystem, error) {
	if cluster == nil {
		return nil, fmt.Errorf("%w: cluster is nil", ErrInvalidObject)
	}
	if packAsset == nil {
		return nil, fmt.Errorf("%w: pack asset is nil", ErrInvalidObject)
	}

	fs := filesys.MakeFsInMemory()
	if err := fs.MkdirAll(packAsset.BaseDir); err != nil {
		return nil, fmt.Errorf("unable to create pack base dir: %w", err)
	}

	// Write pack resources into the filesystem
	resources := make([]string, 0)
	for _, packResource := range packAsset.Resources {
		resourceData, err := packResource.Read()
		if err != nil {
			return nil, fmt.Errorf("unable to read pack resource %s", packResource.Path)
		}

		if err := fs.WriteFile(packResource.Path, resourceData); err != nil {
			return nil, fmt.Errorf("unable to write resource data for %s: %w", packResource.Path, err)
		}
		resources = append(resources, filepath.Base(packResource.Path))
	}

	// Append patches, if any. The patches are fetched from the referenced
	// resource secrets.
	patches := make([]types.Patch, 0)
	for _, patchSpec := range packSpec.Patches {
		secret, err := a.secretFromResourceRef(ctx, shootNamespace, patchSpec.ResourceRef, cluster.Shoot)
		if err != nil {
			return nil, fmt.Errorf(
				"unable to get patch from referenced resource %s for %s: %w",
				patchSpec.ResourceRef,
				packSpec.String(),
				err,
			)
		}

		// Assemble the patch from the referenced resources
		for key, patchData := range secret.Data {
			patchFile := fmt.Sprintf("patch-%s-%s", secret.Name, key)
			if err := fs.WriteFile(filepath.Join(packAsset.BaseDir, patchFile), patchData); err != nil {
				return nil, fmt.Errorf(
					"unable to write patch from referenced resource %s/%s for %s: %w",
					patchSpec.ResourceRef,
					key,
					packSpec.String(),
					err,
				)
			}
			patches = append(
				patches,
				types.Patch{
					Path:   patchFile,
					Target: patchSpec.Target,
				},
			)
		}
	}

	// TODO(dnaeon): add common labels to indicate that we are managing this resource from a pack
	kustomization := types.Kustomization{
		// Gardener uses [metav1.NamespaceSystem] as a system namespace
		// for Gardener-related components.
		//
		// Any resource provided by a pack will be installed in the the
		// Gardener system namespace as well.
		//
		// As of today we cannot use a different namespace, because of
		// the following.
		//
		// https://github.com/gardener/gardener/issues/14342
		// https://github.com/gardener/gardener/pull/14335
		Namespace: metav1.NamespaceSystem,
		Resources: resources,
		BuildMetadata: []string{
			types.OriginAnnotations,
		},
		Patches: patches,
	}

	kustomizationData, err := yaml.Marshal(kustomization)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal kustomization file: %w", err)
	}
	kustomizationPath := filepath.Join(packAsset.BaseDir, konfig.DefaultKustomizationFileName())
	if err := fs.WriteFile(kustomizationPath, kustomizationData); err != nil {
		return nil, fmt.Errorf("unable to write kustomization file: %w", err)
	}

	return fs, nil
}

// secretFromResourceRef reads a [corev1.Secret] from the given referenced
// resource name in the [gardencorev1beta1.Shoot] object.
func (a *Actuator) secretFromResourceRef(ctx context.Context, shootNamespace string, resourceName string, shoot *gardencorev1beta1.Shoot) (*corev1.Secret, error) {
	if shoot == nil {
		return nil, fmt.Errorf("%w: shoot is nil", ErrInvalidObject)
	}

	idx := slices.IndexFunc(shoot.Spec.Resources, func(item gardencorev1beta1.NamedResourceReference) bool {
		return item.Name == resourceName &&
			item.ResourceRef.Kind == "Secret" &&
			item.ResourceRef.APIVersion == corev1.SchemeGroupVersion.String()
	})

	if idx == -1 {
		return nil, fmt.Errorf("resource reference %s does not exist", resourceName)
	}

	secretName := v1beta1constants.ReferencedResourcesPrefix + resourceName
	var secret corev1.Secret
	objKey := client.ObjectKey{
		Name:      secretName,
		Namespace: shootNamespace,
	}
	if err := a.client.Get(ctx, objKey, &secret); err != nil {
		eerr := fmt.Errorf(
			"unable to get secret %s/%s from resource ref %s: %w",
			shootNamespace,
			secretName,
			resourceName,
			err,
		)

		return nil, eerr
	}

	return &secret, nil
}

// Delete deletes any resources managed by the [Actuator]. This method
// implements the [extension.Actuator] interface.
func (a *Actuator) Delete(ctx context.Context, logger logr.Logger, ex *extensionsv1alpha1.Extension) error {
	logger.Info("deleting resources managed by extension")

	// Delete all managed resources for all packs
	for _, pack := range a.collection.Packs {
		mrName := fmt.Sprintf("%s-%s", ManagedResourcePrefix, pack.Name)
		if err := managedresources.DeleteForShoot(ctx, a.client, ex.Namespace, mrName); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}

// ForceDelete signals the [Actuator] to delete any resources managed by it,
// because of a force-delete event of the shoot cluster. This method implements
// the [extension.Actuator] interface.
func (a *Actuator) ForceDelete(ctx context.Context, logger logr.Logger, ex *extensionsv1alpha1.Extension) error {
	logger.Info("shoot has been force-deleted, deleting resources managed by extension")

	return a.Delete(ctx, logger, ex)
}

// Restore restores the resources managed by the extension [Actuator]. This
// method implements the [extension.Actuator] interface.
func (a *Actuator) Restore(ctx context.Context, logger logr.Logger, ex *extensionsv1alpha1.Extension) error {
	return a.Reconcile(ctx, logger, ex)
}

// Migrate signals the [Actuator] to reconcile the resources managed by it,
// because of a shoot control-plane migration event. This method implements the
// [extension.Actuator] interface.
func (a *Actuator) Migrate(ctx context.Context, logger logr.Logger, ex *extensionsv1alpha1.Extension) error {
	return a.Reconcile(ctx, logger, ex)
}
