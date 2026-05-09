package orchestrator

import (
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestPluginDeliveryMappingForBrowserTools(t *testing.T) {
	mapping := pluginDeliveryMappingForMetadata(tools.ToolMetadata{Name: "browser_snapshot"})
	if len(mapping.CitationSourceTypes) != 1 || mapping.CitationSourceTypes[0] != "web" {
		t.Fatalf("expected browser_snapshot citation mapping to stay web, got %+v", mapping)
	}
	if len(mapping.DeliveryTypes) != 1 || mapping.DeliveryTypes[0] != "task_detail" {
		t.Fatalf("expected browser_snapshot delivery mapping to stay task_detail, got %+v", mapping)
	}
}
