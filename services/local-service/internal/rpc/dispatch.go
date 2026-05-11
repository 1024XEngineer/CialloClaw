package rpc

import "time"

// dispatch is the single RPC dispatch path that validates protocol shape,
// resolves handlers, decodes params, and rewraps orchestrator output.
func (s *Server) dispatch(request requestEnvelope) any {
	if request.JSONRPC != "2.0" {
		return newErrorEnvelope(request.ID, &rpcError{
			Code:    errInvalidParams,
			Message: "INVALID_PARAMS",
			Detail:  "jsonrpc version must be 2.0",
			TraceID: "trace_rpc_version",
		})
	}

	handler, ok := s.handlers[request.Method]
	if !ok {
		return newErrorEnvelope(request.ID, &rpcError{
			Code:    errMethodNotFound,
			Message: "JSON_RPC_METHOD_NOT_FOUND",
			Detail:  "method is not registered in the stable stub router",
			TraceID: traceIDFromRequest(request.Params),
		})
	}

	params, rpcErr := s.decodeMethodParams(request.Method, request.Params)
	if rpcErr != nil {
		return newErrorEnvelope(request.ID, rpcErr)
	}

	data, handlerErr := handler(params)
	if handlerErr != nil {
		return newErrorEnvelope(request.ID, handlerErr)
	}

	return newSuccessEnvelope(request.ID, data, s.nowRFC3339())
}

func (s *Server) decodeMethodParams(method string, rawParams []byte) (map[string]any, *rpcError) {
	if spec, ok := s.methodSpecs[method]; ok {
		return spec.Decode(rawParams)
	}
	return decodeParams(rawParams)
}

// nowRFC3339 returns the shared response timestamp format.
func (s *Server) nowRFC3339() string {
	return s.now().Format(time.RFC3339)
}
