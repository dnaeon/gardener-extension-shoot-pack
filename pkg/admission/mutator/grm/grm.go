// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package grm

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	extensionswebhookctx "github.com/gardener/gardener/extensions/pkg/webhook/context"
	resourcemanagerv1alpha1 "github.com/gardener/gardener/pkg/apis/config/resourcemanager/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	packactuator "github.com/gardener/gardener-extension-shoot-pack/pkg/actuator/pack"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/apis/config"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/assets"
)

const (
	// grmConfigMapPrefix is the prefix of the configmap for the GRM.
	grmConfigMapPrefix = "gardener-resource-manager-"

	// grmConfigYAMLKey is the YAML key under which the GRM config resides
	// in a ConfigMap.
	grmConfigYAMLKey = "config.yaml"
)

// ErrInvalidMutator is an error, which is returned when attempting to create a
// new mutator with invalid configuration.
var ErrInvalidMutator = errors.New("invalid mutator")

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

// mutator is an implementation of the [extensionswebhook.Mutator] interface,
// which mutates the Gardener Resource Manager (GRM) in the shoot control-plane
// namespace, by adding the various namespaces of the packs provided by the
// extension controller.
type mutator struct {
	client        client.Client
	ser           *json.Serializer
	decoder       runtime.Decoder
	logger        logr.Logger
	extensionType string
}

var _ extensionswebhook.Mutator = &mutator{}

// newMutator creates a new [mutator], which implements the
// [extensionswebhook.Mutator] interface.
func newMutator(c client.Client, logger logr.Logger) (*mutator, error) {
	if c == nil {
		return nil, fmt.Errorf("%w: invalid client specified", ErrInvalidMutator)
	}

	scheme := c.Scheme()
	ser := json.NewSerializerWithOptions(
		json.DefaultMetaFactory,
		scheme,
		scheme,
		json.SerializerOptions{Yaml: true},
	)
	decoder := serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder()

	m := &mutator{
		client:        c,
		ser:           ser,
		decoder:       decoder,
		logger:        logger,
		extensionType: packactuator.ExtensionType,
	}

	return m, nil
}

// Mutate implements the [extensionswebhook.Mutator] interface.
func (m *mutator) Mutate(ctx context.Context, newObj, oldObj client.Object) error {
	if newObj.GetDeletionTimestamp() != nil {
		return nil
	}

	// Get cluster associated with the object we are about to mutate
	gardenCtx := extensionswebhookctx.NewGardenContext(m.client, newObj)
	cluster, err := gardenCtx.GetCluster(ctx)
	if err != nil {
		return fmt.Errorf("unable to find cluster: %w", err)
	}

	// Check whether the extension has been enabled for the cluster, and
	// mutate, only if it is enabled for the current cluster.
	if !m.isExtensionEnabled(cluster.Shoot) {
		return nil
	}

	newConfigMap, ok := newObj.(*corev1.ConfigMap)
	if !ok {
		return fmt.Errorf("invalid object type: %T", newObj)
	}

	oldConfigMap, ok := oldObj.(*corev1.ConfigMap)
	if !ok {
		oldConfigMap = nil
	}

	cfg, err := m.getExtensionConfig(cluster.Shoot)
	if err != nil {
		return IgnoreExtensionNotFound(err)
	}

	return m.mutateGRMConfigMap(newConfigMap, oldConfigMap, cfg)
}

// getExtensionConfig returns the [config.PackConfig] spec of the extension by
// extracting it from the given [gardencorev1beta1.Shoot] object.
func (m *mutator) getExtensionConfig(obj *gardencorev1beta1.Shoot) (config.PackConfig, error) {
	if obj == nil {
		return config.PackConfig{}, errors.New("invalid shoot resource provided")
	}

	idx := slices.IndexFunc(obj.Spec.Extensions, func(ext gardencorev1beta1.Extension) bool {
		return ext.Type == m.extensionType
	})

	if idx == -1 {
		return config.PackConfig{}, fmt.Errorf("%w: %s", ErrExtensionNotFound, m.extensionType)
	}

	ext := obj.Spec.Extensions[idx]
	if ext.ProviderConfig == nil {
		return config.PackConfig{}, fmt.Errorf("no provider config specified for %s", m.extensionType)
	}

	var cfg config.PackConfig
	if err := runtime.DecodeInto(m.decoder, ext.ProviderConfig.Raw, &cfg); err != nil {
		return config.PackConfig{}, fmt.Errorf("invalid provider spec configuration for %s: %w", m.extensionType, err)
	}

	return cfg, nil
}

// isExtensionEnabled is a predicate which returns true, if the extension is
// enabled for the given [gardencorev1beta1.Shoot], otherwise it returns
// false.
func (m *mutator) isExtensionEnabled(obj *gardencorev1beta1.Shoot) bool {
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

// mutateGRMConfigMap mutates the Gardener Resource Manager (GRM) ConfigMap by
// adding the list of namespaces to the `targetClientConnection.namespaces'
// list.
func (m *mutator) mutateGRMConfigMap(newObj *corev1.ConfigMap, _ *corev1.ConfigMap, cfg config.PackConfig) error {
	if newObj == nil || newObj.Data == nil {
		return nil
	}

	extensionswebhook.LogMutation(m.logger, newObj.Kind, newObj.Namespace, newObj.Name)
	grmConfigRaw, ok := newObj.Data[grmConfigYAMLKey]
	if !ok {
		return fmt.Errorf(
			"%s/%s configmap does not provide %s key",
			newObj.Namespace,
			newObj.Name,
			grmConfigYAMLKey,
		)
	}

	// Collect the additional namespaces provided by the packs.
	additionalNamespaces := make([]string, 0)
	collection, err := assets.New(assets.FS, assets.WithSkipVerify(false))
	if err != nil {
		return err
	}
	for _, packSpec := range cfg.Spec.Packs {
		pack, err := collection.GetPack(packSpec.Name, packSpec.Version)
		if err != nil {
			return err
		}

		additionalNamespaces = append(additionalNamespaces, pack.Namespace)
	}
	slices.Sort(additionalNamespaces)

	m.logger.Info(
		"Appending pack namespaces to GRM .targetClientConnection.namespaces",
		"configmap", fmt.Sprintf("%s/%s", newObj.Namespace, newObj.Name),
		"namespaces", additionalNamespaces,
	)

	var grmConfig resourcemanagerv1alpha1.ResourceManagerConfiguration
	if err := runtime.DecodeInto(m.decoder, []byte(grmConfigRaw), &grmConfig); err != nil {
		return fmt.Errorf("unable to decode configuration: %w", err)
	}

	if grmConfig.TargetClientConnection != nil {
		grmConfig.TargetClientConnection.Namespaces = append(
			grmConfig.TargetClientConnection.Namespaces,
			additionalNamespaces...,
		)
	}

	data, err := runtime.Encode(m.ser, &grmConfig)
	if err != nil {
		return fmt.Errorf("unable to encode configuration: %w", err)
	}

	newObj.Data[grmConfigYAMLKey] = string(data)

	return nil
}

// NewMutatorWebhook returns a new mutating [extensionswebhook.Webhook].
func NewMutatorWebhook(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	logger := mgr.GetLogger()
	mutator, err := newMutator(mgr.GetClient(), logger)
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("mutator.%s", mutator.extensionType)
	extensionLabel := fmt.Sprintf("%s%s", v1beta1constants.LabelExtensionPrefix, mutator.extensionType)
	path := fmt.Sprintf("/webhooks/mutate/%s", mutator.extensionType)
	logger.Info("setting up webhook", "name", name, "path", path, "label", extensionLabel)

	args := extensionswebhook.Args{
		Provider: mutator.extensionType,
		Name:     name,
		Path:     path,
		Mutators: map[extensionswebhook.Mutator][]extensionswebhook.Type{
			mutator: {{Obj: &corev1.ConfigMap{}}},
		},
		Target: extensionswebhook.TargetSeed,
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				extensionLabel: "true",
			},
		},
		Predicates: []predicate.Predicate{
			predicate.NewPredicateFuncs(func(obj client.Object) bool {
				configMap, ok := obj.(*corev1.ConfigMap)
				if !ok {
					return false
				}

				return strings.HasPrefix(configMap.Name, grmConfigMapPrefix)
			}),
		},
	}

	return extensionswebhook.New(mgr, args)
}
