package model

import (
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
)

const (
	// ProviderRoadmapStageStable marks a provider that is allowed on the current
	// owner-5 mainline.
	ProviderRoadmapStageStable = "stable"
	// ProviderExposureSettingsOnly keeps provider selection behind the existing
	// settings surface until the roadmap explicitly promotes dedicated model APIs.
	ProviderExposureSettingsOnly = "settings_only"
)

// ProviderDescriptor reserves the future multi-provider contract without
// enabling additional providers before the roadmap says the mainline is ready.
type ProviderDescriptor struct {
	DisplayName         string
	Name                string
	Version             string
	Source              string
	Entry               string
	Summary             string
	SupportsToolCalling bool
	SupportsStreaming   bool
	Capabilities        []string
	Permissions         []string
	RoadmapStage        string
	Exposure            string
}

type providerAdapter struct {
	descriptor ProviderDescriptor
	validate   func(cfg config.ModelConfig) error
	build      func(cfg ServiceConfig, apiKey string) (Client, error)
}

var defaultProviderRegistry = newProviderRegistry([]providerAdapter{{
	descriptor: ProviderDescriptor{
		DisplayName:         "OpenAI Responses",
		Name:                OpenAIResponsesProvider,
		Version:             "builtin-v1",
		Source:              "builtin",
		Entry:               "builtin://model-provider/openai_responses",
		Summary:             "Built-in model provider route for OpenAI Responses text generation and tool-calling.",
		SupportsToolCalling: true,
		SupportsStreaming:   false,
		Capabilities:        []string{"generate_text", "generate_tool_calls"},
		Permissions:         []string{"secret:model_api_key", "network:model_api"},
		RoadmapStage:        ProviderRoadmapStageStable,
		Exposure:            ProviderExposureSettingsOnly,
	},
	validate: func(cfg config.ModelConfig) error {
		if strings.TrimSpace(cfg.Endpoint) == "" {
			return ErrOpenAIEndpointRequired
		}
		if strings.TrimSpace(cfg.ModelID) == "" {
			return ErrOpenAIModelIDRequired
		}
		return nil
	},
	build: func(cfg ServiceConfig, apiKey string) (Client, error) {
		return NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
			APIKey:   apiKey,
			Endpoint: strings.TrimSpace(cfg.ModelConfig.Endpoint),
			ModelID:  strings.TrimSpace(cfg.ModelConfig.ModelID),
		})
	},
}})

type providerRegistry struct {
	order []string
	items map[string]providerAdapter
}

func newProviderRegistry(items []providerAdapter) providerRegistry {
	registry := providerRegistry{order: make([]string, 0, len(items)), items: make(map[string]providerAdapter, len(items))}
	for _, item := range items {
		name := strings.TrimSpace(item.descriptor.Name)
		if name == "" {
			continue
		}
		registry.order = append(registry.order, name)
		registry.items[name] = item
	}
	return registry
}

func (r providerRegistry) descriptor(provider string) (ProviderDescriptor, bool) {
	item, ok := r.items[strings.TrimSpace(provider)]
	if !ok {
		return ProviderDescriptor{}, false
	}
	return cloneProviderDescriptor(item.descriptor), true
}

func (r providerRegistry) adapter(provider string) (providerAdapter, bool) {
	item, ok := r.items[strings.TrimSpace(provider)]
	return item, ok
}

func (r providerRegistry) descriptors() []ProviderDescriptor {
	result := make([]ProviderDescriptor, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, cloneProviderDescriptor(r.items[name].descriptor))
	}
	return result
}

// RegisteredProviderDescriptors exposes the currently supported provider list in
// a stable order so future expansion does not require rewiring current callers.
func RegisteredProviderDescriptors() []ProviderDescriptor {
	return defaultProviderRegistry.descriptors()
}

// LookupProviderDescriptor resolves one provider route descriptor by provider name.
func LookupProviderDescriptor(provider string) (ProviderDescriptor, bool) {
	return defaultProviderRegistry.descriptor(provider)
}

func cloneProviderDescriptor(item ProviderDescriptor) ProviderDescriptor {
	item.Capabilities = append([]string(nil), item.Capabilities...)
	item.Permissions = append([]string(nil), item.Permissions...)
	return item
}

func validateProviderConfig(cfg config.ModelConfig) error {
	adapter, ok := defaultProviderRegistry.adapter(cfg.Provider)
	if !ok {
		return ErrModelProviderUnsupported
	}
	if err := adapter.validate(cfg); err != nil {
		return err
	}
	return nil
}

func buildProviderClient(cfg ServiceConfig, apiKey string) (Client, error) {
	adapter, ok := defaultProviderRegistry.adapter(cfg.ModelConfig.Provider)
	if !ok {
		return nil, ErrModelProviderUnsupported
	}
	client, err := adapter.build(cfg, apiKey)
	if err != nil {
		return nil, fmt.Errorf("build provider client %s: %w", strings.TrimSpace(cfg.ModelConfig.Provider), err)
	}
	return client, nil
}
