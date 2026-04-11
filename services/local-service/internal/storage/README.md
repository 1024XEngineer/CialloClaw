# Storage Module

This module owns the backend-local storage boundary inside `services/local-service/internal/storage`.

## Current In-Module Scope

- Expose the storage backend name used by the local service
- Normalize and validate the configured database path
- Report whether the storage service is configured
- Provide a typed descriptor snapshot for upper layers
- Expose a storage-local memory persistence contract with an in-memory implementation
- Expose a storage-local task/run persistence contract with both SQLite-backed and in-memory implementations
- Expose storage-backed audit and recovery-point writer boundaries
- Prefer a SQLite-backed memory store with WAL when a database path is configured
- Prefer a SQLite-backed task/run store with WAL when a database path is configured
- Report a typed capability snapshot for future module integration
- Expose whether SQLite initialization fell back to in-memory behavior
- Expose retrieval-hit persistence and FTS5/sqlite-vec skeleton capabilities for later memory integration

## Current P0 Boundary

- Backend: `sqlite_wal`
- Adapter contract: `platform.StorageAdapter`
- Required configuration: non-empty `DatabasePath()`
- Memory persistence contract prefers SQLite + WAL and falls back to in-memory storage if SQLite initialization is unavailable
- Task/run persistence contract prefers SQLite + WAL and falls back to in-memory storage if SQLite initialization is unavailable
- Audit and recovery-point writers prefer SQLite + WAL and fall back to in-memory writers if initialization is unavailable
- SQLite-backed memory writes require non-empty `memory_summary_id`, `task_id`, `run_id`, `summary`, and RFC3339 `created_at`
- SQLite-backed task/run writes require non-empty `task_id`, `session_id`, `run_id`, `status`, and non-zero `started_at` / `updated_at`
- SQLite-backed retrieval-hit writes require non-empty hit identifiers and RFC3339 `created_at`
- SQLite-backed audit writes require non-empty `audit_id`, `task_id`, `type`, `action`, `result`, and `created_at`
- SQLite-backed recovery-point writes require non-empty `recovery_point_id`, `task_id`, `summary`, and `created_at`
- FTS5 is initialized as the current local full-text skeleton, while sqlite-vec remains a storage-level skeleton placeholder only

## Boundary Rules

- `task` / `run` orchestration does not belong here; only persistence-facing contracts and repository implementations live here
- RPC response assembly does not belong here
- Memory retrieval business logic does not belong here; only storage-facing contracts and temporary local implementations live here
- Governance writer persistence now also belongs here, but governance rule evaluation still does not
- Artifact and Stronghold implementations do not belong here yet
- Protocol schema ownership stays in `/packages/protocol`
- Current SQLite search implementation uses an FTS5 skeleton plus fallback SQL scanning, and is still not the final FTS/vec retrieval design
- Callers that create the storage service should close it so SQLite handles are released

## Known Unfrozen Decisions

- The exact storage-facing interface for memory persistence
- Whether artifact storage and secret storage share this module or split further
- The final read/query interfaces for persisted audit and recovery-point data
- Whether database path validation should later depend on stronger path-policy checks
- The final capability snapshot fields required by bootstrap or orchestrator
- When the in-memory memory store should be replaced by SQLite-backed persistence
- Whether SQLite repository initialization errors should later fail fast instead of falling back
