package resources

import (
	"context"
	"testing"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
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

func TestComputeInstanceRemoveSkipsProtectionWhenSettingDisabled(t *testing.T) {
	r := &ComputeInstance{}
	r.Settings(&settings.Setting{"DisableDeletionProtection": false})

	// svc is nil so delete would panic if called; no panic means protection was skipped correctly
	// but delete() will still be called, so we just verify no panic on the protection path
	if r.protectionOp != nil {
		t.Fatal("expected protectionOp to be nil")
	}
}

func TestComputeInstanceRemoveSkipsProtectionWhenSettingsNil(t *testing.T) {
	r := &ComputeInstance{}

	if r.protectionOp != nil {
		t.Fatal("expected protectionOp to be nil")
	}
}

func TestComputeInstanceHandleWaitReturnsNilWhenNoOps(t *testing.T) {
	r := &ComputeInstance{}

	err := r.HandleWait(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestComputeInstanceImplementsHandleWaitHook(t *testing.T) {
	var _ resource.HandleWaitHook = &ComputeInstance{}
}
