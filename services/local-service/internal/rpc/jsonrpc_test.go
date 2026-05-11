package rpc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONRPCDecodeHelpers(t *testing.T) {
	req := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "agent.settings.update",
		Params:  mustMarshal(t, map[string]any{"request_meta": map[string]any{"trace_id": "trace_rpc_helpers"}, "enabled": true, "count": 3, "labels": []string{"a", "b"}}),
	}
	decoded, err := decodeRequest(strings.NewReader(string(mustMarshal(t, req))))
	if err != nil {
		t.Fatalf("decodeRequest returned error: %v", err)
	}
	params, err := decodeParams(decoded.Params)
	if err != nil {
		t.Fatalf("decodeParams returned error: %v", err)
	}
	if requestTraceID(params) != "trace_rpc_helpers" || traceIDFromRequest(decoded.Params) != "trace_rpc_helpers" {
		t.Fatalf("expected trace helpers to extract request trace ids, params=%+v", params)
	}
	if !boolValue(params, "enabled", false) || intValue(params, "count", 0) != 3 {
		t.Fatalf("expected primitive decoders to round-trip values, params=%+v", params)
	}
	labels := stringSliceValue(params["labels"])
	if len(labels) != 2 || labels[1] != "b" {
		t.Fatalf("expected stringSliceValue to decode labels, got %+v", labels)
	}
	if boolValue(nil, "enabled", false) || intValue(nil, "count", 0) != 0 || len(stringSliceValue(map[string]any{"labels": 7}["labels"])) != 0 {
		t.Fatal("expected primitive decoders to handle nil and invalid inputs")
	}
}

func TestJSONRPCDecodeErrors(t *testing.T) {
	tests := []struct {
		name    string
		decode  func() *rpcError
		traceID string
	}{
		{
			name: "malformed request",
			decode: func() *rpcError {
				_, err := decodeRequest(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"agent.settings.get","params":`))
				return err
			},
			traceID: "trace_rpc_decode",
		},
		{
			name: "malformed params",
			decode: func() *rpcError {
				_, err := decodeParams(json.RawMessage(`{"broken":`))
				return err
			},
			traceID: "trace_rpc_params",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.decode()
			if err == nil {
				t.Fatal("expected rpc error")
			}
			if err.Code != errInvalidParams || err.Message != "INVALID_PARAMS" || err.TraceID != test.traceID {
				t.Fatalf("unexpected rpc error %+v", err)
			}
		})
	}
}

func TestJSONRPCEnvelopeHelpers(t *testing.T) {
	success := newSuccessEnvelope(nil, map[string]any{"ok": true}, "2026-04-08T10:00:00Z")
	if success.JSONRPC != "2.0" || string(success.ID) != "null" || success.Result.Meta.ServerTime != "2026-04-08T10:00:00Z" {
		t.Fatalf("unexpected success envelope %+v", success)
	}

	failure := newErrorEnvelope(nil, &rpcError{
		Code:    errMethodNotFound,
		Message: "JSON_RPC_METHOD_NOT_FOUND",
		Detail:  "missing",
		TraceID: "trace_missing",
	})
	if failure.JSONRPC != "2.0" || string(failure.ID) != "null" || failure.Error.Data.TraceID != "trace_missing" {
		t.Fatalf("unexpected error envelope %+v", failure)
	}

	notification := newNotificationEnvelope("task.updated", map[string]any{"task_id": "task_001"})
	if notification.JSONRPC != "2.0" || notification.Method != "task.updated" {
		t.Fatalf("unexpected notification envelope %+v", notification)
	}
}

func TestDispatchProtocolResponses(t *testing.T) {
	server := newTestServer()
	tests := []struct {
		name          string
		request       requestEnvelope
		expectedCode  int
		expectedError string
		expectedTrace string
	}{
		{
			name: "invalid jsonrpc version",
			request: requestEnvelope{
				JSONRPC: "1.0",
				ID:      json.RawMessage(`"req-version"`),
				Method:  methodAgentSettingsGet,
			},
			expectedCode:  errInvalidParams,
			expectedError: "INVALID_PARAMS",
			expectedTrace: "trace_rpc_version",
		},
		{
			name: "method not found",
			request: requestEnvelope{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`"req-missing"`),
				Method:  "agent.unknown.call",
				Params:  mustMarshal(t, map[string]any{"request_meta": map[string]any{"trace_id": "trace_unknown"}}),
			},
			expectedCode:  errMethodNotFound,
			expectedError: "JSON_RPC_METHOD_NOT_FOUND",
			expectedTrace: "trace_unknown",
		},
		{
			name: "invalid params",
			request: requestEnvelope{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`"req-invalid-params"`),
				Method:  methodAgentTaskStart,
				Params:  json.RawMessage(`[]`),
			},
			expectedCode:  errInvalidParams,
			expectedError: "INVALID_PARAMS",
			expectedTrace: "trace_rpc_params",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := server.dispatch(test.request)
			errEnvelope, ok := response.(errorEnvelope)
			if !ok {
				t.Fatalf("expected error response envelope, got %#v", response)
			}
			if errEnvelope.Error.Code != test.expectedCode || errEnvelope.Error.Message != test.expectedError || errEnvelope.Error.Data.TraceID != test.expectedTrace {
				t.Fatalf("unexpected response %+v", errEnvelope)
			}
		})
	}
}
