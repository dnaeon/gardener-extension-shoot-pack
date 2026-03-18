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
	"path/filepath"
	"slices"
	"strconv"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/extension"
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/component-base/featuregate"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resource"

	"github.com/gardener/gardener-extension-shoot-pack/pkg/apis/config"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/apis/config/validation"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/assets"
)

// ErrInvalidActuator is an error which is returned when creating an [Actuator]
// with invalid config settings.
var ErrInvalidActuator = errors.New("invalid actuator")

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
	collection, err := assets.New(assets.FS, assets.WithSkipVerify(false))
	if err != nil {
		return err
	}

	enabledPacks := make([]config.Pack, 0)
	disabledPacks := make([]config.Pack, 0)
	for _, pack := range cfg.Spec.Packs {
		if err := a.createManagedResourceForPack(ctx, ex.Namespace, collection, pack); err != nil {
			return err
		}
		enabledPacks = append(enabledPacks, pack)
	}

	// Delete ManagedResources for packs, which are have not been
	// configured.
	for _, pack := range collection.Packs {
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

// createManagedResourceForPack creates a new ManagedResource with the resources
// provided by the pack.
func (a *Actuator) createManagedResourceForPack(ctx context.Context, shootNamespace string, collection *assets.Collection, packConfig config.Pack) error {
	if !collection.PackExists(packConfig.Name, packConfig.Version) {
		return fmt.Errorf("pack %s@%s does not exist", packConfig.Name, packConfig.Version)
	}

	pack, err := collection.GetPack(packConfig.Name, packConfig.Version)
	if err != nil {
		return err
	}

	registry := managedresources.NewRegistry(
		kubernetes.ShootScheme,
		kubernetes.ShootCodec,
		kubernetes.ShootSerializer,
	)

	// Register pack resources with the registry
	for _, packResource := range pack.Resources {
		resourceData, err := packResource.Read()
		if err != nil {
			return fmt.Errorf("unable to read pack resource %s", packResource.Path)
		}

		// Create [resource.Resource] items from the pack resource.
		// Take into account that a single pack resource may contain
		// multiple Kubernetes resources.
		resourceItems, err := a.resourceFactory.SliceFromBytes(resourceData)
		if err != nil {
			return fmt.Errorf("unable to create resource from %s: %w", packResource.Path, err)
		}

		// Adjust the namespace for each namespace-scoped resources.
		for idx, r := range resourceItems {
			gvk := r.GetGvk()
			if !gvk.IsClusterScoped() {
				// Gardener uses [metav1.NamespaceSystem] as a
				// system namespace for Gardener-related
				// components.
				//
				// Any resource provided by a pack will be
				// installed in the the Gardener system
				// namespace as well.
				if err := r.SetNamespace(metav1.NamespaceSystem); err != nil {
					return fmt.Errorf("unable to set namespace for %s: %w", packResource.Path, err)
				}
			}

			// ... and register
			data, err := r.AsYAML()
			if err != nil {
				return fmt.Errorf("unable to marshal %s: %w", packResource.Path, err)
			}

			registry.AddSerialized(
				filepath.Join(packResource.Path, strconv.Itoa(idx)),
				data,
			)
		}
	}

	data, err := registry.SerializedObjects()
	if err != nil {
		return err
	}

	mrName := fmt.Sprintf("%s-%s", ManagedResourcePrefix, pack.Name)

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

// Delete deletes any resources managed by the [Actuator]. This method
// implements the [extension.Actuator] interface.
func (a *Actuator) Delete(ctx context.Context, logger logr.Logger, ex *extensionsv1alpha1.Extension) error {
	logger.Info("deleting resources managed by extension")

	// Delete all managed resources for all packs
	collection, err := assets.New(assets.FS, assets.WithSkipVerify(false))
	if err != nil {
		return err
	}

	for _, pack := range collection.Packs {
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
