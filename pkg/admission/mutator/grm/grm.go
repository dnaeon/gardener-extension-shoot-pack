// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package grm

import (
	"context"
	"errors"
	"fmt"
	"slices"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	extensionswebhookctx "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	oscutils "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/utils"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	packactuator "github.com/gardener/gardener-extension-shoot-pack/pkg/actuator/pack"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/apis/config"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/assets"
)

// ErrInvalidEnsurer is an error, which is returned when attempting to create a
// new [ensurer] with invalid configuration.
var ErrInvalidEnsurer = errors.New("invalid ensurer")

// ErrExtensionNotFound is an error, which is returned when the extension was
// not found in the [gardencorev1beta1.Shoot] spec.
var ErrExtensionNotFound = errors.New("extension not found")

// IgnoreExtensionNotFound returns nil if err is [ErrExtensionNotFound],
// otherwise it returns err.
func IgnoreExtensionNotFound(err error) error {
	if errors.Is(err, ErrExtensionNotFound) {
		return nil
	}

	return err
}

// ensurer is an implementation of the [genericmutator.Ensurer] interface, which
// mutates the Gardener Resource Manager (GRM) deployment in the shoot
// control-plane namespace, by adding the various namespaces of the packs
// provided by the extension controller as extra target namespaces.
type ensurer struct {
	genericmutator.NoopEnsurer

	client        client.Client
	decoder       runtime.Decoder
	logger        logr.Logger
	extensionType string
}

var _ genericmutator.Ensurer = &ensurer{}

// newEnsurer returns a new [genericmutator.Ensurer] implementation, which
// mutates the Gardener Resource Manager (GRM) deployment in the shoot
// control-plane namespace, by adding the various namespaces of the packs
// provided by the extension controller as extra target namespaces.
func newEnsurer(c client.Client, logger logr.Logger) (*ensurer, error) {
	if c == nil {
		return nil, fmt.Errorf("%w: invalid client specified", ErrInvalidEnsurer)
	}

	e := &ensurer{
		client:        c,
		decoder:       serializer.NewCodecFactory(c.Scheme(), serializer.EnableStrict).UniversalDecoder(),
		logger:        logger,
		extensionType: packactuator.ExtensionType,
	}

	return e, nil
}

// EnsureGardenerResourceManagerDeployment ensure that the Gardener Resource
// Manager deployment is configured with the extra pack namespaces provided by
// the extension controller.
func (e *ensurer) EnsureGardenerResourceManagerDeployment(ctx context.Context, gardenCtx extensionswebhookctx.GardenContext, newObj, oldObj *appsv1.Deployment) error {
	// Get cluster associated with the object we are about to mutate
	cluster, err := gardenCtx.GetCluster(ctx)
	if err != nil {
		return fmt.Errorf("unable to find cluster: %w", err)
	}

	// Nothing to do here, if the shoot cluster is hibernated or being deleted.
	if cluster.Shoot != nil && cluster.Shoot.GetDeletionTimestamp() != nil {
		return nil
	}
	if v1beta1helper.HibernationIsEnabled(cluster.Shoot) {
		return nil
	}

	// Check whether the extension has been enabled for the cluster, and
	// mutate, only if it is enabled for the current cluster.
	if !e.isExtensionEnabled(cluster.Shoot) {
		return nil
	}

	extensionCfg, err := e.getExtensionConfig(cluster.Shoot)
	if err != nil {
		return IgnoreExtensionNotFound(err)
	}

	return e.mutateGRMDeployment(newObj, oldObj, extensionCfg)
}

// getExtensionConfig returns the [config.PackConfig] spec of the extension by
// extracting it from the given [gardencorev1beta1.Shoot] object.
func (e *ensurer) getExtensionConfig(obj *gardencorev1beta1.Shoot) (config.PackConfig, error) {
	if obj == nil {
		return config.PackConfig{}, errors.New("invalid shoot resource provided")
	}

	idx := slices.IndexFunc(obj.Spec.Extensions, func(ext gardencorev1beta1.Extension) bool {
		return ext.Type == e.extensionType
	})

	if idx == -1 {
		return config.PackConfig{}, fmt.Errorf("%w: %s", ErrExtensionNotFound, e.extensionType)
	}

	ext := obj.Spec.Extensions[idx]
	if ext.ProviderConfig == nil {
		return config.PackConfig{}, fmt.Errorf("no provider config specified for %s", e.extensionType)
	}

	var cfg config.PackConfig
	if err := runtime.DecodeInto(e.decoder, ext.ProviderConfig.Raw, &cfg); err != nil {
		return config.PackConfig{}, fmt.Errorf("invalid provider spec configuration for %s: %w", e.extensionType, err)
	}

	return cfg, nil
}

// isExtensionEnabled is a predicate which returns true, if the extension is
// enabled for the given [gardencorev1beta1.Shoot], otherwise it returns
// false.
func (m *ensurer) isExtensionEnabled(obj *gardencorev1beta1.Shoot) bool {
	if obj == nil {
		return false
	}

	idx := slices.IndexFunc(obj.Spec.Extensions, func(ext gardencorev1beta1.Extension) bool {
		return ext.Type == m.extensionType
	})

	if idx == -1 {
		return false
	}

	// Check whether the extension has been explicitly disabled
	if obj.Spec.Extensions[idx].Disabled != nil && *obj.Spec.Extensions[idx].Disabled {
		return false
	}

	return true
}

// mutateGRMDeployment mutates the Gardener Resource Manager (GRM) Deployment by
// adding the list of pack namespaces as extra namespaces for GRM.
func (e *ensurer) mutateGRMDeployment(newObj *appsv1.Deployment, _ *appsv1.Deployment, extensionCfg config.PackConfig) error {
	if newObj == nil {
		return nil
	}

	grmContainer := extensionswebhook.ContainerWithName(
		newObj.Spec.Template.Spec.Containers,
		v1beta1constants.DeploymentNameGardenerResourceManager,
	)

	if grmContainer == nil {
		return nil
	}

	// Collect the additional namespaces provided by the packs.
	additionalNamespaces := make([]string, 0)
	collection, err := assets.New(assets.FS, assets.WithSkipVerify(false))
	if err != nil {
		return err
	}
	for _, packSpec := range extensionCfg.Spec.Packs {
		pack, err := collection.GetPack(packSpec.Name, packSpec.Version)
		if err != nil {
			return err
		}
		additionalNamespaces = append(additionalNamespaces, pack.Namespace)
	}
	slices.Sort(additionalNamespaces)

	if grmContainer.Args == nil {
		grmContainer.Args = make([]string, 0)
	}

	extensionswebhook.LogMutation(e.logger, newObj.Kind, newObj.Namespace, newObj.Name)
	e.logger.Info(
		"Appending pack namespaces to GRM extra namespaces",
		"namespaces", additionalNamespaces,
	)

	for _, ns := range additionalNamespaces {
		grmContainer.Args = append(
			grmContainer.Args,
			fmt.Sprintf("--extra-target-namespace=%s", ns),
		)
	}

	return nil
}

// NewWebhook returns a new mutating [extensionswebhook.Webhook], which ensures
// that Gardener Resource Manager deployment is configured with the extra
// namespaces provided by the extension controller.
func NewWebhook(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	logger := mgr.GetLogger()
	ensurer, err := newEnsurer(mgr.GetClient(), logger)
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("ensurer.grm.%s", ensurer.extensionType)
	extensionLabel := fmt.Sprintf("%s%s", v1beta1constants.LabelExtensionPrefix, ensurer.extensionType)
	path := fmt.Sprintf("/webhooks/ensurer/grm/%s", ensurer.extensionType)
	logger.Info("setting up webhook", "name", name, "path", path, "label", extensionLabel)

	fciCodec := oscutils.NewFileContentInlineCodec()
	mutator := genericmutator.NewMutator(
		mgr,
		ensurer,
		oscutils.NewUnitSerializer(),
		kubelet.NewConfigCodec(fciCodec),
		fciCodec,
		logger,
	)

	objTypes := []extensionswebhook.Type{
		{Obj: &appsv1.Deployment{}},
	}

	handler, err := extensionswebhook.NewBuilder(mgr, logger).WithMutator(mutator, objTypes...).Build()
	webhook := &extensionswebhook.Webhook{
		Provider: ensurer.extensionType,
		Name:     name,
		Path:     path,
		Types:    objTypes,
		Target:   extensionswebhook.TargetSeed,
		Webhook:  &admission.Webhook{Handler: handler},
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				extensionLabel: "true",
			},
		},
		ObjectSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
				v1beta1constants.LabelApp:   v1beta1constants.DeploymentNameGardenerResourceManager,
			},
		},
	}

	return webhook, nil
}
