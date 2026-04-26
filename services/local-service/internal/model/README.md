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
- Function tool calling with custom function definitions
- Transport: synchronous HTTP request/response

## Explicitly Not Supported Yet

- Streaming output
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
- Keep optional extension interfaces for streaming and richer tool-calling flows without breaking existing callers
- Expand provider implementations behind the same module-local abstractions

## Known Unfrozen Decisions

- API key source: environment, Stronghold, or external injection
- Whether streaming is required in the first integrated flow
- Whether tool calling belongs in the first integrated flow
- Final mapping from module errors to protocol error codes

## Current Architecture Blockers

Because the current change scope is limited to `services/local-service/internal/model`, this module is still not fully v10-compliant.

The remaining blockers that require changes outside this directory are:

- Move model input/output contracts into `/packages/protocol`
- Add `api_key` and budget-related settings into the shared config and secret-management path

Inside the current module-only scope, the package now preserves `task_id`, `run_id`, `request_id`, usage, and latency through `GenerateTextResponse` and `InvocationRecord`, and those structures are mirrored to `/packages/protocol/types/core.ts`. The Go types remain temporary backend mirrors until a cross-language protocol generation path is introduced.

## Current Validation Path

- Unit tests cover the minimal request/response path with `httptest`
- `bootstrap` now wires model service through `NewServiceFromConfig(...)` and fails fast on invalid configuration
- An opt-in live smoke test can be run with:
  - `RUN_LIVE_OPENAI_RESPONSES_TEST=1`
  - `OPENAI_API_KEY`
  - optional `OPENAI_RESPONSES_ENDPOINT`
  - optional `OPENAI_RESPONSES_MODEL`

The live smoke test stays skipped by default unless `RUN_LIVE_OPENAI_RESPONSES_TEST=1`, so CI remains deterministic even when a shell or CI runner happens to export `OPENAI_API_KEY`.

## Current Protocol Alignment

- The minimal model request/response/invocation structures are now registered in `/packages/protocol/types/core.ts`
- The Go structures in `internal/model/types.go` remain temporary backend mirrors until a cross-language protocol generation path is introduced
- Field names and JSON tags are aligned with protocol naming so later migration cost is reduced

## Current Config Path

- `config.ModelConfig` now carries:
  - `provider`
  - `model_id`
  - `endpoint`
  - `api_key`
  - `single_task_limit`
  - `daily_limit`
  - `budget_auto_downgrade`
  - `max_tool_iterations`
  - `context_compress_chars`
  - `context_keep_recent`
- `bootstrap` consumes the config-backed API key instead of directly reading the environment
- `ServiceConfig.APIKey` remains as a temporary fallback input so the module can migrate without breaking existing tests and callers in one step

The loop-related fields are consumed by the execution layer through `model.Service`
so the first Agent Loop runtime remains configurable without wiring a second
parallel config object through every caller.

## Secret Integration Boundary

- `ServiceConfig` now supports an optional `SecretSource`
- `SecretSource` is a Stronghold-ready boundary for resolving provider API keys without binding this module to a concrete secret backend yet
- Current resolution order is:
  1. `ServiceConfig.APIKey`
  2. `ModelConfig.APIKey`
  3. `SecretSource.ResolveModelAPIKey(provider)`

This keeps the module ready for Stronghold integration while avoiding direct coupling before the secret-management path is frozen.
