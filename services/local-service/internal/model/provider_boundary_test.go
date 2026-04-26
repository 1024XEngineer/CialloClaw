package model

import "testing"

func TestRegisteredProviderDescriptorsFreezeCurrentBoundary(t *testing.T) {
	descriptors := RegisteredProviderDescriptors()
	if len(descriptors) != 1 {
		t.Fatalf("expected one current provider descriptor, got %+v", descriptors)
	}
	descriptor := descriptors[0]
	if descriptor.Name != OpenAIResponsesProvider {
		t.Fatalf("expected %q provider descriptor, got %+v", OpenAIResponsesProvider, descriptor)
	}
	if descriptor.RoadmapStage != ProviderRoadmapStageStable {
		t.Fatalf("expected stable roadmap stage, got %+v", descriptor)
	}
	if descriptor.Exposure != ProviderExposureSettingsOnly {
		t.Fatalf("expected settings-only exposure boundary, got %+v", descriptor)
	}
	if !descriptor.SupportsToolCalling {
		t.Fatalf("expected current provider to support tool calling, got %+v", descriptor)
	}
}

func TestProviderRegistryDescriptorLookupPreservesBoundaryFields(t *testing.T) {
	descriptor, ok := defaultProviderRegistry.descriptor(OpenAIResponsesProvider)
	if !ok {
		t.Fatal("expected default provider descriptor lookup to succeed")
	}
	if descriptor.RoadmapStage != ProviderRoadmapStageStable || descriptor.Exposure != ProviderExposureSettingsOnly {
		t.Fatalf("expected descriptor lookup to keep boundary metadata, got %+v", descriptor)
	}
}
