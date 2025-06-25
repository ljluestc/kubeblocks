/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var addonlog = logf.Log.WithName("addon-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *Addon) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate-apps-kubeblocks-io-v1alpha1-addon,mutating=false,failurePolicy=fail,groups=apps.kubeblocks.io,resources=addons,verbs=create;update,versions=v1alpha1,name=vaddon.kb.io,sideEffects=None,admissionReviewVersions=v1

var _ webhook.Validator = &Addon{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Addon) ValidateCreate() error {
	addonlog.Info("validate create", "name", r.Name)

	// validate mandatory spec
	if r.Spec.Helm == nil {
		return fmt.Errorf("helm is required in addon spec")
	}

	// validate helm chart location
	if err := validateHelmLocation(r.Spec.Helm); err != nil {
		return err
	}

	// validation selector
	if r.Spec.Selector != nil {
		if err := validateSelector(r.Spec.Selector); err != nil {
			return err
		}
	}

	// validate addon install parameters
	if err := validateAddonInstallParams(r); err != nil {
		return err
	}

	// validate addon dependencies
	if err := validateAddonDependencies(r); err != nil {
		return err
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Addon) ValidateUpdate(old runtime.Object) error {
	addonlog.Info("validate update", "name", r.Name)
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Addon) ValidateDelete() error {
	addonlog.Info("validate delete", "name", r.Name)
	return nil
}

// validateHelmLocation validates the helm chart location
func validateHelmLocation(helm *HelmTypeSpec) error {
	if helm.ChartLocationURL == "" {
		return fmt.Errorf("chartLocationURL is required in helm spec")
	}
	return nil
}

// validateSelector validates the addon selector
func validateSelector(selector *AddonSelector) error {
	return nil
}

// validateAddonInstallParams validates addon install parameters
func validateAddonInstallParams(r *Addon) error {
	return nil
}

// validateAddonDependencies validates addon dependencies
func validateAddonDependencies(r *Addon) error {
	if len(r.Spec.Dependencies) == 0 {
		return nil
	}

	// Check for duplicate dependencies
	dependencySet := make(map[string]struct{}, len(r.Spec.Dependencies))
	for _, dep := range r.Spec.Dependencies {
		if dep == "" {
			return fmt.Errorf("dependency name cannot be empty")
		}

		if _, exists := dependencySet[dep]; exists {
			return fmt.Errorf("duplicate dependency: %s", dep)
		}
		dependencySet[dep] = struct{}{}
	}

	// Check for self-dependency
	for _, dep := range r.Spec.Dependencies {
		if dep == r.Name {
			return fmt.Errorf("addon cannot depend on itself: %s", r.Name)
		}
	}

	return nil
}
