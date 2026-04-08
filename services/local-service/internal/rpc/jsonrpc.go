// 该文件负责 JSON-RPC 协议结构、解析与响应封装。
package rpc

import (
	"bytes"
	"encoding/json"
	"io"
)

const (
	errInvalidParams  = 1002001
	errMethodNotFound = 1002005
)

type requestEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type responseMeta struct {
	ServerTime string `json:"server_time"`
}

type resultEnvelope struct {
	Data     any          `json:"data"`
	Meta     responseMeta `json:"meta"`
	Warnings []string     `json:"warnings"`
}

type successEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  resultEnvelope  `json:"result"`
}

type errorData struct {
	TraceID string `json:"trace_id,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

type errorEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   struct {
		Code    int       `json:"code"`
		Message string    `json:"message"`
		Data    errorData `json:"data,omitempty"`
	} `json:"error"`
}

type notificationEnvelope struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type rpcError struct {
	Code    int
	Message string
	Detail  string
	TraceID string
}

type methodHandler func(params map[string]any) (any, *rpcError)

func decodeRequest(reader io.Reader) (requestEnvelope, *rpcError) {
	var request requestEnvelope
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&request); err != nil {
		return requestEnvelope{}, &rpcError{
			Code:    errInvalidParams,
			Message: "INVALID_PARAMS",
			Detail:  "invalid json-rpc payload",
			TraceID: "trace_rpc_decode",
		}
	}

	return request, nil
}

func decodeParams(raw json.RawMessage) (map[string]any, *rpcError) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return map[string]any{}, nil
	}

	var params map[string]any
	if err := json.Unmarshal(trimmed, &params); err != nil {
		return nil, &rpcError{
			Code:    errInvalidParams,
			Message: "INVALID_PARAMS",
			Detail:  "params must be a json object",
			TraceID: "trace_rpc_params",
		}
	}

	return params, nil
}

func newSuccessEnvelope(id json.RawMessage, data any, serverTime string) successEnvelope {
	return successEnvelope{
		JSONRPC: "2.0",
		ID:      normalizeID(id),
		Result: resultEnvelope{
			Data: data,
			Meta: responseMeta{
				ServerTime: serverTime,
			},
			Warnings: []string{},
		},
	}
}

func newErrorEnvelope(id json.RawMessage, rpcErr *rpcError) errorEnvelope {
	var envelope errorEnvelope
	envelope.JSONRPC = "2.0"
	envelope.ID = normalizeID(id)
	envelope.Error.Code = rpcErr.Code
	envelope.Error.Message = rpcErr.Message
	envelope.Error.Data = errorData{
		TraceID: rpcErr.TraceID,
		Detail:  rpcErr.Detail,
	}
	return envelope
}

func newNotificationEnvelope(method string, params any) notificationEnvelope {
	return notificationEnvelope{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
}

func normalizeID(id json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(id)
	if len(trimmed) == 0 {
		return json.RawMessage("null")
	}

	return trimmed
}

func traceIDFromRequest(raw json.RawMessage) string {
	params, err := decodeParams(raw)
	if err != nil {
		return "trace_rpc_unknown"
	}

	return requestTraceID(params)
}

func requestTraceID(params map[string]any) string {
	requestMeta := mapValue(params, "request_meta")
	return stringValue(requestMeta, "trace_id", "trace_rpc_unknown")
}

func mapValue(values map[string]any, key string) map[string]any {
	rawValue, ok := values[key]
	if !ok {
		return map[string]any{}
	}

	value, ok := rawValue.(map[string]any)
	if !ok {
		return map[string]any{}
	}

	return value
}

func stringValue(values map[string]any, key, fallback string) string {
	rawValue, ok := values[key]
	if !ok {
		return fallback
	}

	value, ok := rawValue.(string)
	if !ok || value == "" {
		return fallback
	}

	return value
}

func boolValue(values map[string]any, key string, fallback bool) bool {
	rawValue, ok := values[key]
	if !ok {
		return fallback
	}

	value, ok := rawValue.(bool)
	if !ok {
		return fallback
	}

	return value
}

func intValue(values map[string]any, key string, fallback int) int {
	rawValue, ok := values[key]
	if !ok {
		return fallback
	}

	value, ok := rawValue.(float64)
	if !ok {
		return fallback
	}

	return int(value)
}

func stringSliceValue(rawValue any) []string {
	values, ok := rawValue.([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(values))
	for _, rawItem := range values {
		item, ok := rawItem.(string)
		if ok && item != "" {
			result = append(result, item)
		}
	}

	return result
}
