package resources

import (
	"context"
	"testing"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/settings"
)

func TestComputeInstanceRegistrationIncludesDisableDeletionProtection(t *testing.T) {
	reg := registry.GetRegistration(ComputeInstanceResource)
	if reg == nil {
		t.Fatal("registration for ComputeInstance was not found")
	}

	hasSetting := false
	for _, settingName := range reg.Settings {
		if settingName == "DisableDeletionProtection" {
			hasSetting = true
			break
		}
	}

	if !hasSetting {
		t.Fatal("ComputeInstance registration is missing DisableDeletionProtection setting")
	}
}

func TestComputeInstanceDisableDeletionProtectionReturnsNilWhenSettingDisabled(t *testing.T) {
	r := &ComputeInstance{}
	r.Settings(&settings.Setting{"DisableDeletionProtection": false})

	err := r.disableDeletionProtection(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestComputeInstanceDisableDeletionProtectionReturnsNilWhenSettingsNil(t *testing.T) {
	r := &ComputeInstance{}

	err := r.disableDeletionProtection(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}
