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
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddonValidator_ValidateCreate(t *testing.T) {
	tests := []struct {
		name      string
		addon     *Addon
		expectErr bool
	}{
		{
			name: "test addon validator create",
			addon: &Addon{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-addon",
				},
				Spec: AddonSpec{
					Helm: &HelmTypeSpec{
						ChartLocationURL: "charturl",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "test addon with valid dependencies",
			addon: &Addon{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-addon",
				},
				Spec: AddonSpec{
					Helm: &HelmTypeSpec{
						ChartLocationURL: "charturl",
					},
					Dependencies: []string{"dep1", "dep2"},
				},
			},
			expectErr: false,
		},
		{
			name: "test addon with duplicate dependencies",
			addon: &Addon{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-addon",
				},
				Spec: AddonSpec{
					Helm: &HelmTypeSpec{
						ChartLocationURL: "charturl",
					},
					Dependencies: []string{"dep1", "dep1"},
				},
			},
			expectErr: true,
		},
		{
			name: "test addon with self dependency",
			addon: &Addon{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-addon",
				},
				Spec: AddonSpec{
					Helm: &HelmTypeSpec{
						ChartLocationURL: "charturl",
					},
					Dependencies: []string{"test-addon"},
				},
			},
			expectErr: true,
		},
		{
			name: "test addon with empty dependency name",
			addon: &Addon{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-addon",
				},
				Spec: AddonSpec{
					Helm: &HelmTypeSpec{
						ChartLocationURL: "charturl",
					},
					Dependencies: []string{"dep1", ""},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.addon.ValidateCreate()
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAddonValidator_ValidateUpdateAndDelete(t *testing.T) {
	addon := &Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-addon",
		},
		Spec: AddonSpec{
			Helm: &HelmTypeSpec{
				ChartLocationURL: "charturl",
			},
		},
	}

	oldAddon := &Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-addon",
		},
		Spec: AddonSpec{
			Helm: &HelmTypeSpec{
				ChartLocationURL: "charturl-old",
			},
		},
	}

	// ValidateUpdate should work with different chart URL
	assert.NoError(t, addon.ValidateUpdate(oldAddon))

	// Test update with valid dependencies
	addonWithDeps := addon.DeepCopy()
	addonWithDeps.Spec.Dependencies = []string{"dep1", "dep2"}
	assert.NoError(t, addonWithDeps.ValidateUpdate(addon))

	// Test update with invalid dependencies (duplicate)
	addonWithInvalidDeps := addon.DeepCopy()
	addonWithInvalidDeps.Spec.Dependencies = []string{"dep1", "dep1"}
	err := addonWithInvalidDeps.ValidateUpdate(addon)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate dependency")

	// Test update with invalid dependencies (self reference)
	addonWithSelfDep := addon.DeepCopy()
	addonWithSelfDep.Spec.Dependencies = []string{"test-addon"}
	err = addonWithSelfDep.ValidateUpdate(addon)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot depend on itself")

	// ValidateDelete should always return nil
	assert.NoError(t, addon.ValidateDelete())
	assert.NoError(t, addonWithDeps.ValidateDelete())
}

func TestAddonValidator_ValidateUpdateAndDelete(t *testing.T) {
	tests := []struct {
		name      string
		addon     *Addon
		oldAddon  *Addon
		expectErr bool
	}{
		{
			name: "test basic update validation",
			addon: &Addon{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-addon",
				},
				Spec: AddonSpec{
					Helm: &HelmTypeSpec{
						ChartLocationURL: "charturl-new",
						ChartVersion:     "1.1.0",
					},
				},
			},
			oldAddon: &Addon{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-addon",
				},
				Spec: AddonSpec{
					Helm: &HelmTypeSpec{
						ChartLocationURL: "charturl",
						ChartVersion:     "1.0.0",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "test update with dependencies",
			addon: &Addon{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-addon",
				},
				Spec: AddonSpec{
					Helm: &HelmTypeSpec{
						ChartLocationURL: "charturl",
					},
					Dependencies: []string{"dep1", "dep2"},
				},
			},
			oldAddon: &Addon{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-addon",
				},
				Spec: AddonSpec{
					Helm: &HelmTypeSpec{
						ChartLocationURL: "charturl",
					},
					Dependencies: []string{"dep1"},
				},
			},
			expectErr: false,
		},
		{
			name: "test update removing dependencies",
			addon: &Addon{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-addon",
				},
				Spec: AddonSpec{
					Helm: &HelmTypeSpec{
						ChartLocationURL: "charturl",
					},
					Dependencies: []string{},
				},
			},
			oldAddon: &Addon{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-addon",
				},
				Spec: AddonSpec{
					Helm: &HelmTypeSpec{
						ChartLocationURL: "charturl",
					},
					Dependencies: []string{"dep1", "dep2"},
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.addon.ValidateUpdate(tt.oldAddon)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

	// Test delete validation with various scenarios
	deleteTests := []struct {
		name      string
		addon     *Addon
		expectErr bool
	}{
		{
			name: "test basic delete validation",
			addon: &Addon{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-addon",
				},
				Spec: AddonSpec{
					Helm: &HelmTypeSpec{
						ChartLocationURL: "charturl",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "test delete with dependencies",
			addon: &Addon{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-addon",
				},
				Spec: AddonSpec{
					Helm: &HelmTypeSpec{
						ChartLocationURL: "charturl",
					},
					Dependencies: []string{"dep1", "dep2"},
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range deleteTests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.addon.ValidateDelete()
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
