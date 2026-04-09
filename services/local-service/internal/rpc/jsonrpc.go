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

// requestEnvelope 定义当前模块的数据结构。

// requestEnvelope 对应一条进入本地服务的 JSON-RPC 2.0 请求。
// 这里保留原始 id 和 params 载荷，后续再做分步解码。
type requestEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// responseMeta 定义当前模块的数据结构。

// responseMeta 保存所有成功响应都会附带的元信息。
type responseMeta struct {
	ServerTime string `json:"server_time"`
}

// resultEnvelope 定义当前模块的数据结构。

// resultEnvelope 封装成功结果的 data、meta 和 warning 列表。
type resultEnvelope struct {
	Data     any          `json:"data"`
	Meta     responseMeta `json:"meta"`
	Warnings []string     `json:"warnings"`
}

// successEnvelope 定义当前模块的数据结构。

// successEnvelope 是稳定的 JSON-RPC 成功响应包装体。
type successEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  resultEnvelope  `json:"result"`
}

// errorData 定义当前模块的数据结构。

// errorData 保存统一错误码之外的可观测调试信息。
type errorData struct {
	TraceID string `json:"trace_id,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// errorEnvelope 定义当前模块的数据结构。

// errorEnvelope 是稳定的 JSON-RPC 错误响应包装体。
type errorEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   struct {
		Code    int       `json:"code"`
		Message string    `json:"message"`
		Data    errorData `json:"data,omitempty"`
	} `json:"error"`
}

// notificationEnvelope 定义当前模块的数据结构。

// notificationEnvelope 用于 named pipe 和调试事件流里追加发送通知消息。
type notificationEnvelope struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

// rpcError 定义当前模块的数据结构。

// rpcError 是 rpc 模块内部使用的错误结构。
// 它最终会被转换成统一的 JSON-RPC error envelope。
type rpcError struct {
	Code    int
	Message string
	Detail  string
	TraceID string
}

type methodHandler func(params map[string]any) (any, *rpcError)

// decodeRequest 处理当前模块的相关逻辑。

// decodeRequest 负责把输入流解码成 requestEnvelope。
// 这里只校验 JSON 结构合法，不负责校验 method 是否存在。
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

// decodeParams 处理当前模块的相关逻辑。

// decodeParams 将 params 原始载荷解码为对象形式。
// 当前主链路要求 params 必须是 JSON object 或空值。
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

// newSuccessEnvelope 处理当前模块的相关逻辑。

// newSuccessEnvelope 组装统一成功响应。
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

// newErrorEnvelope 处理当前模块的相关逻辑。

// newErrorEnvelope 组装统一错误响应。
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

// newNotificationEnvelope 处理当前模块的相关逻辑。

// newNotificationEnvelope 把内部通知记录包装成 JSON-RPC notification。
func newNotificationEnvelope(method string, params any) notificationEnvelope {
	return notificationEnvelope{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
}

// normalizeID 处理当前模块的相关逻辑。

// normalizeID 保证所有响应里的 id 都满足 JSON-RPC 2.0 语义。
// 缺省 id 会统一回写为 null。
func normalizeID(id json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(id)
	if len(trimmed) == 0 {
		return json.RawMessage("null")
	}

	return trimmed
}

// traceIDFromRequest 处理当前模块的相关逻辑。

// traceIDFromRequest 从请求参数中提取 trace_id，方便错误定位。
func traceIDFromRequest(raw json.RawMessage) string {
	params, err := decodeParams(raw)
	if err != nil {
		return "trace_rpc_unknown"
	}

	return requestTraceID(params)
}

// requestTraceID 处理当前模块的相关逻辑。

// requestTraceID 从 request_meta 中读取 trace_id。
func requestTraceID(params map[string]any) string {
	requestMeta := mapValue(params, "request_meta")
	return stringValue(requestMeta, "trace_id", "trace_rpc_unknown")
}

// mapValue 处理当前模块的相关逻辑。

// mapValue 安全读取嵌套对象，避免在解码阶段反复写类型断言。
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

// stringValue 处理当前模块的相关逻辑。

// stringValue 安全读取字符串字段，缺省时回退到指定默认值。
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

// boolValue 处理当前模块的相关逻辑。

// boolValue 安全读取布尔字段。
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

// intValue 处理当前模块的相关逻辑。

// intValue 安全读取经过 JSON 解码后的数值字段。
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

// stringSliceValue 处理当前模块的相关逻辑。

// stringSliceValue 把 JSON 里的数组值转换成 []string。
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
