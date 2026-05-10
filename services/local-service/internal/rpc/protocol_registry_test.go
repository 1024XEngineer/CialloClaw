package rpc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestStableMethodRegistryMatchesProtocolSource(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "packages", "protocol", "rpc", "methods.ts"))
	if err != nil {
		t.Fatalf("read protocol method source: %v", err)
	}
	stableBlock := protocolStableMethodBlock(string(source))
	if stableBlock == "" {
		t.Fatal("expected RPC_METHODS_STABLE block in packages/protocol/rpc/methods.ts")
	}

	protocolMethods := protocolMethodSet(stableBlock)
	server := newTestServer()
	for _, method := range server.stableMethodRegistry() {
		if !protocolMethods[method.Name] {
			t.Fatalf("go rpc method %q is not declared in packages/protocol RPC_METHODS_STABLE", method.Name)
		}
		delete(protocolMethods, method.Name)
	}
	if len(protocolMethods) > 0 {
		t.Fatalf("packages/protocol stable methods are missing from Go registry: %+v", protocolMethods)
	}
}

func TestAgentTaskStartDTOOmitsUnsupportedIntentField(t *testing.T) {
	params, rpcErr := decodeAgentTaskStartParams(mustMarshal(t, map[string]any{
		"request_meta": map[string]any{
			"trace_id":    "trace_task_start_dto",
			"client_time": "2026-05-09T12:00:00+08:00",
		},
		"session_id": "sess_task_start_dto",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "selected content",
			"page_context": map[string]any{
				"title":      "Editor",
				"process_id": 42,
			},
		},
		"context": map[string]any{
			"selection": map[string]any{
				"text": "selected content",
			},
		},
		"delivery": map[string]any{
			"preferred": "bubble",
			"fallback":  "task_detail",
		},
		"options": map[string]any{
			"confirm_required": true,
		},
		"intent": map[string]any{
			"name": "write_file",
		},
	}))
	if rpcErr != nil {
		t.Fatalf("decode task.start params: %+v", rpcErr)
	}
	if _, ok := params["intent"]; ok {
		t.Fatalf("expected unsupported intent field to be omitted, got %+v", params["intent"])
	}
	if stringValue(params, "session_id", "") != "sess_task_start_dto" {
		t.Fatalf("expected session_id to survive dto normalization, got %+v", params)
	}
	if delivery := mapValue(params, "delivery"); stringValue(delivery, "preferred", "") != "bubble" || stringValue(delivery, "fallback", "") != "task_detail" {
		t.Fatalf("expected delivery preference to survive dto normalization, got %+v", delivery)
	}
	input := mapValue(params, "input")
	pageContext := mapValue(input, "page_context")
	if stringValue(pageContext, "title", "") != "Editor" || intValue(pageContext, "process_id", 0) != 42 {
		t.Fatalf("expected page context to survive dto normalization, got %+v", pageContext)
	}
}

func TestStableMethodRegistryDispatchMatrix(t *testing.T) {
	server := newTestServer()
	expectedDecoders := map[string]func(json.RawMessage) (map[string]any, *rpcError){
		methodAgentInputSubmit:            decodeAgentInputSubmitParams,
		methodAgentTaskStart:              decodeAgentTaskStartParams,
		methodAgentTaskConfirm:            decodeParams,
		methodAgentTaskControl:            decodeParams,
		methodAgentTaskDetailGet:          decodeParams,
		methodAgentTaskInspectorConfigGet: decodeParams,
		methodAgentTaskInspectorRun:       decodeParams,
		methodAgentDeliveryOpen:           decodeParams,
		methodAgentSettingsGet:            decodeParams,
		methodAgentPluginDetailGet:        decodeParams,
	}

	for _, method := range server.stableMethodRegistry() {
		if method.Handle == nil {
			t.Fatalf("expected registered handler for %s", method.Name)
		}
		if method.Decode == nil {
			t.Fatalf("expected decoder for %s", method.Name)
		}
		expectedDecode, ok := expectedDecoders[method.Name]
		if !ok {
			continue
		}
		if reflect.ValueOf(method.Decode).Pointer() != reflect.ValueOf(expectedDecode).Pointer() {
			t.Fatalf("unexpected decoder for %s", method.Name)
		}
	}
}

func protocolStableMethodBlock(source string) string {
	start := strings.Index(source, "RPC_METHODS_STABLE")
	end := strings.Index(source, "RPC_METHODS_PLANNED")
	if start < 0 || end <= start {
		return ""
	}
	return source[start:end]
}

func protocolMethodSet(source string) map[string]bool {
	methodPattern := regexp.MustCompile(`"agent\.[^"]+"`)
	matches := methodPattern.FindAllString(source, -1)
	result := make(map[string]bool, len(matches))
	for _, match := range matches {
		result[strings.Trim(match, `"`)] = true
	}
	return result
}
