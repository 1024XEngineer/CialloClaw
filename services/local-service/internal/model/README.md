# Model Module

This module is the only backend entry point for LLM provider access inside `services/local-service`.

## Current Scope

- Build and validate model clients from `config.ModelConfig`
- Send minimal text generation requests through the configured provider
- Normalize provider responses into module-local request and response types
- Return request metadata, token usage, and latency to upper layers
- Classify provider request, timeout, HTTP status, and response parsing errors

## Current Supported Capability

- Provider: `openai_responses`
- Request shape: single text input
- Response shape: single text output
- Transport: synchronous HTTP request/response

## Explicitly Not Supported Yet

- Streaming output
- Tool calling
- Multi-modal input
- Structured JSON output contracts
- Provider failover or routing strategies
- Stronghold-backed API key loading
- Protocol-layer error code mapping

## Boundary Rules

- Upper layers should depend on `Service` and module-local types, not provider-specific payloads
- Provider SDK or HTTP payload details must stay inside this module
- `task` / `run` state transitions do not belong here
- Audit, storage, checkpoint, and risk logic do not belong here

## Planned Extension Direction

- Keep `GenerateText` as the stable minimum path
- Add optional extension interfaces for streaming and tool calling without breaking existing callers
- Expand provider implementations behind the same module-local abstractions

## Known Unfrozen Decisions

- API key source: environment, Stronghold, or external injection
- Whether streaming is required in the first integrated flow
- Whether tool calling belongs in the first integrated flow
- Final mapping from module errors to protocol error codes

## Current Architecture Blockers

Because the current change scope is limited to `services/local-service/internal/model`, this module is still not fully v10-compliant.

The remaining blockers that require changes outside this directory are:

- Replace the temporary local HTTP payload implementation with the official OpenAI Responses SDK
- Move model input/output contracts into `/packages/protocol`
- Add `api_key` and budget-related settings into the shared config and secret-management path
- Switch bootstrap wiring to `NewServiceFromConfig(...)` so startup fails fast on invalid model configuration

Inside the current module-only scope, the package now preserves `task_id`, `run_id`, `request_id`, usage, and latency through `GenerateTextResponse` and `InvocationRecord`, but this is still a temporary local contract until protocol-level types are introduced.
