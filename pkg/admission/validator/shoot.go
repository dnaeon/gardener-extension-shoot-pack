// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"errors"
	"fmt"
	"slices"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	packactuator "github.com/gardener/gardener-extension-shoot-pack/pkg/actuator/pack"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/apis/config"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/apis/config/validation"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/assets"
)

// ErrExtensionNotFound is an error, which is returned when the extension was
// not found in the [core.Shoot] spec.
var ErrExtensionNotFound = errors.New("extension not found")

// IgnoreExtensionNotFound returns nil if err is [ErrExtensionNotFound],
// otherwise it returns err.
func IgnoreExtensionNotFound(err error) error {
	if errors.Is(err, ErrExtensionNotFound) {
		return nil
	}

	return err
}

// shootValidator is an implementation of [extensionswebhook.Validator], which
// validates the provider configuration of the extension from a [core.Shoot]
// spec.
type shootValidator struct {
	decoder       runtime.Decoder
	extensionType string
	collection    *assets.Collection
}

var _ extensionswebhook.Validator = &shootValidator{}

// Option is a function which configures the shoot validator.
type Option func(v *shootValidator) error

// WithPackCollection is an [Option] which configures the shoot validator to use
// the given pack collection.
func WithPackCollection(collection *assets.Collection) Option {
	opt := func(a *shootValidator) error {
		a.collection = collection

		return nil
	}

	return opt
}

// newShootValidator returns a new [shootValidator], which implements the
// [extensionswebhook.Validator] interface.
func newShootValidator(decoder runtime.Decoder, opts ...Option) (*shootValidator, error) {
	validator := &shootValidator{
		decoder:       decoder,
		extensionType: packactuator.ExtensionType,
	}

	if decoder == nil {
		return nil, fmt.Errorf("invalid decoder specified for shoot validator %s", validator.extensionType)
	}

	for _, opt := range opts {
		if err := opt(validator); err != nil {
			return nil, err
		}
	}

	if validator.collection == nil {
		collection, err := assets.New(assets.FS, assets.WithSkipVerify(false))
		if err != nil {
			return nil, err
		}
		validator.collection = collection
	}

	return validator, nil
}

// NewShootValidator returns a new [extensionswebhook.Validator] for
// [core.Shoot] objects.
func NewShootValidator(decoder runtime.Decoder, opts ...Option) (extensionswebhook.Validator, error) {
	return newShootValidator(decoder, opts...)
}

// Validate implements the [extensionswebhook.Validator] interface.
func (v *shootValidator) Validate(ctx context.Context, newObj, oldObj client.Object) error {
	newShoot, ok := newObj.(*gardencore.Shoot)
	if !ok {
		return fmt.Errorf("invalid object type: %T", newObj)
	}
	oldShoot, ok := oldObj.(*gardencore.Shoot)
	if !ok {
		oldShoot = nil
	}

	if newShoot.DeletionTimestamp != nil {
		return nil
	}

	return v.validateExtension(newShoot, oldShoot)
}

// getExtension returns the [core.Extension] by extracting it from the given
// [core.Shoot] object.
func (v *shootValidator) getExtension(obj *gardencore.Shoot) (gardencore.Extension, error) {
	if obj == nil {
		return gardencore.Extension{}, errors.New("invalid shoot resource provided")
	}

	idx := slices.IndexFunc(obj.Spec.Extensions, func(ext gardencore.Extension) bool {
		return ext.Type == v.extensionType
	})

	if idx == -1 {
		return gardencore.Extension{}, fmt.Errorf("%w: %s", ErrExtensionNotFound, v.extensionType)
	}

	return obj.Spec.Extensions[idx], nil
}

// validateExtension validates the extension configuration from the given
// [core.Shoot] specs.
func (v *shootValidator) validateExtension(newObj *gardencore.Shoot, _ *gardencore.Shoot) error {
	ext, err := v.getExtension(newObj)
	if err != nil {
		return IgnoreExtensionNotFound(err)
	}

	// Extension is disabled, nothing to validate
	if ext.Disabled != nil && *ext.Disabled {
		return nil
	}

	if ext.ProviderConfig == nil {
		return fmt.Errorf("no provider config specified for %s", v.extensionType)
	}

	var cfg config.PackConfig
	if err := runtime.DecodeInto(v.decoder, ext.ProviderConfig.Raw, &cfg); err != nil {
		return fmt.Errorf("invalid provider spec configuration for %s: %w", v.extensionType, err)
	}

	if err := validation.Validate(cfg); err != nil {
		return fmt.Errorf("invalid extension configuration for %s: %w", v.extensionType, err)
	}

	for _, packSpec := range cfg.Spec.Packs {
		if !v.collection.PackExists(packSpec.Name, packSpec.Version) {
			return fmt.Errorf("pack %s does not exist", packSpec.String())
		}

		// Validate patches, which are described by referenced resources stored
		// as secrets.
		for _, patchSpec := range packSpec.Patches {
			idx := slices.IndexFunc(newObj.Spec.Resources, func(item gardencore.NamedResourceReference) bool {
				return item.Name == patchSpec.ResourceRef &&
					item.ResourceRef.Kind == "Secret" &&
					item.ResourceRef.APIVersion == corev1.SchemeGroupVersion.String()
			})

			if idx == -1 {
				return fmt.Errorf(
					"pack %s uses patch which refers to a non-existing secret resource %s",
					packSpec.String(),
					patchSpec.ResourceRef,
				)
			}
		}
	}

	return nil
}

// NewShootValidatorWebhook returns a new validating [extensionswebhook.Webhook]
// for [core.Shoot] objects.
func NewShootValidatorWebhook(mgr manager.Manager, opts ...Option) (*extensionswebhook.Webhook, error) {
	decoder := serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder()
	validator, err := newShootValidator(decoder, opts...)
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("validator.%s", validator.extensionType)
	extensionLabel := fmt.Sprintf("%s%s", v1beta1constants.LabelExtensionExtensionTypePrefix, validator.extensionType)
	path := fmt.Sprintf("/webhooks/validate/%s", validator.extensionType)

	logger := mgr.GetLogger()
	logger.Info("setting up webhook", "name", name, "path", path, "label", extensionLabel)

	args := extensionswebhook.Args{
		Provider: validator.extensionType,
		Name:     name,
		Path:     path,
		Validators: map[extensionswebhook.Validator][]extensionswebhook.Type{
			validator: {{Obj: &core.Shoot{}}},
		},
		Target: extensionswebhook.TargetSeed,
		ObjectSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				extensionLabel: "true",
			},
		},
	}

	return extensionswebhook.New(mgr, args)
}
