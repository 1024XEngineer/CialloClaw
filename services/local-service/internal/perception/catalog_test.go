package perception

import "testing"

func TestDefaultPackageDescriptorExposesStableBoundaryMetadata(t *testing.T) {
	descriptor := DefaultPackageDescriptor()
	if descriptor.PackageID != "desktop_context_core" || descriptor.Version == "" || descriptor.Source != "builtin" || descriptor.Entry == "" {
		t.Fatalf("expected default perception package descriptor metadata, got %+v", descriptor)
	}
	if len(descriptor.Capabilities) != 4 || descriptor.Capabilities[0] != "page_context" {
		t.Fatalf("expected default perception package capabilities, got %+v", descriptor)
	}
	if len(descriptor.Permissions) != 4 || descriptor.Permissions[0] != "screen:read" {
		t.Fatalf("expected default perception package permissions, got %+v", descriptor)
	}

	descriptor.Capabilities[0] = "mutated"
	descriptor.Permissions[0] = "mutated"
	freshDescriptor := DefaultPackageDescriptor()
	if freshDescriptor.Capabilities[0] != "page_context" || freshDescriptor.Permissions[0] != "screen:read" {
		t.Fatalf("expected default perception package descriptor to be cloned, got %+v", freshDescriptor)
	}
}

func TestLookupPackageDescriptorHandlesMissingPackages(t *testing.T) {
	descriptor, ok := LookupPackageDescriptor("desktop_context_core")
	if !ok || descriptor.PackageID != "desktop_context_core" {
		t.Fatalf("expected builtin perception package lookup to succeed, got descriptor=%+v ok=%v", descriptor, ok)
	}
	if _, ok := LookupPackageDescriptor("missing_package"); ok {
		t.Fatal("expected unknown perception package lookup to fail")
	}
}
