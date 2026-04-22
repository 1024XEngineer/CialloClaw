package model

import "testing"

func TestLookupProviderDescriptorExposesVersionedRouteMetadata(t *testing.T) {
	descriptor, ok := LookupProviderDescriptor(OpenAIResponsesProvider)
	if !ok {
		t.Fatal("expected OpenAI Responses provider descriptor to resolve")
	}
	if descriptor.DisplayName != "OpenAI Responses" || descriptor.Version == "" || descriptor.Source != "builtin" || descriptor.Entry == "" {
		t.Fatalf("expected provider descriptor to expose route metadata, got %+v", descriptor)
	}
	if len(descriptor.Capabilities) != 2 || descriptor.Capabilities[0] != "generate_text" {
		t.Fatalf("expected provider descriptor to expose stable capabilities, got %+v", descriptor)
	}
	if len(descriptor.Permissions) != 2 || descriptor.Permissions[0] != "secret:model_api_key" {
		t.Fatalf("expected provider descriptor to expose boundary permissions, got %+v", descriptor)
	}

	descriptor.Capabilities[0] = "mutated"
	descriptor.Permissions[0] = "mutated"
	freshDescriptor, ok := LookupProviderDescriptor(OpenAIResponsesProvider)
	if !ok {
		t.Fatal("expected fresh provider descriptor lookup to succeed")
	}
	if freshDescriptor.Capabilities[0] != "generate_text" || freshDescriptor.Permissions[0] != "secret:model_api_key" {
		t.Fatalf("expected provider descriptor lookup to return cloned slices, got %+v", freshDescriptor)
	}
}
