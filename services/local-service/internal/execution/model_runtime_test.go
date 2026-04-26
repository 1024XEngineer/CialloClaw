package execution

import (
	"testing"

	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
)

func TestReplaceModelAndCurrentModelHandleNilAndInstalledServices(t *testing.T) {
	var nilService *Service
	if nilService.ReplaceModel(nil) != nil {
		t.Fatal("expected nil receiver ReplaceModel to return nil")
	}
	if nilService.CurrentModel() != nil {
		t.Fatal("expected nil receiver CurrentModel to stay nil")
	}

	service := &Service{}
	modelService := model.NewService(serviceconfig.ModelConfig{
		Provider: model.OpenAIResponsesProvider,
		ModelID:  "gpt-4.1-mini",
		Endpoint: "https://example.invalid/v1/responses",
	})
	if service.ReplaceModel(modelService) != service {
		t.Fatal("expected ReplaceModel to support fluent receiver usage")
	}
	if service.CurrentModel() != modelService {
		t.Fatalf("expected CurrentModel to expose installed runtime model, got %+v", service.CurrentModel())
	}
	if service.currentModel() != modelService {
		t.Fatalf("expected currentModel helper to reuse CurrentModel, got %+v", service.currentModel())
	}

	service.ReplaceModel(nil)
	if service.CurrentModel() != nil {
		t.Fatalf("expected ReplaceModel(nil) to clear runtime model, got %+v", service.CurrentModel())
	}
}
