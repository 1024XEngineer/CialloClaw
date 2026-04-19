package model

import (
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
)

// ProviderDescriptor reserves the future multi-provider contract without
// enabling additional providers before the roadmap says the mainline is ready.
type ProviderDescriptor struct {
	Name                string
	SupportsToolCalling bool
}

type providerAdapter struct {
	descriptor ProviderDescriptor
	validate   func(cfg config.ModelConfig) error
	build      func(cfg ServiceConfig, apiKey string) (Client, error)
}

var defaultProviderRegistry = newProviderRegistry([]providerAdapter{{
	descriptor: ProviderDescriptor{Name: OpenAIResponsesProvider, SupportsToolCalling: true},
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
	return item.descriptor, true
}

func (r providerRegistry) adapter(provider string) (providerAdapter, bool) {
	item, ok := r.items[strings.TrimSpace(provider)]
	return item, ok
}

func (r providerRegistry) descriptors() []ProviderDescriptor {
	result := make([]ProviderDescriptor, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, r.items[name].descriptor)
	}
	return result
}

// RegisteredProviderDescriptors exposes the currently supported provider list in
// a stable order so future expansion does not require rewiring current callers.
func RegisteredProviderDescriptors() []ProviderDescriptor {
	return defaultProviderRegistry.descriptors()
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
