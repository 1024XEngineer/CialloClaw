# Memory Module

This module owns the backend-local memory boundary inside `services/local-service/internal/memory`.

## Current In-Module Scope

- Validate memory summary writes
- Normalize retrieval queries and recent-reference limits
- Search retrieval hits and normalize them by `memory_id`
- Provide in-memory store behavior for local testing and bootstrap-safe fallback
- Convert retrieval data into mirror-reference payloads for upper layers

## Current Retrieval Rules

- Default retrieval limit: `5`
- Max retrieval limit: `20`
- Empty or non-positive limit falls back to the default
- Oversized limit is capped to the max
- Duplicate hits are merged by `memory_id`
- Duplicate hits keep the highest-score item
- Final hit order is descending by score
- Current `task_id + run_id` summaries are skipped by the in-memory search path

## Current In-Memory Store Behavior

- `SaveSummary` appends validated summaries in memory
- `Search` performs case-insensitive term matching over summary text
- Match score is the ratio of matched query terms to total query terms
- `ListSummaries` returns the most recently appended summaries first

## Boundary Rules

- `task` / `run` orchestration does not belong here
- RPC handler response assembly does not belong here
- Real SQLite / FTS5 / sqlite-vec persistence does not belong here yet
- Protocol schema ownership stays in `/packages/protocol`

## Known Unfrozen Decisions

- Whether `memory_id` must always equal `memory_summary_id`
- Whether one task can persist multiple `MemorySummary` records
- Final candidate-generation rules for `MemoryCandidate`
- Final ranking inputs beyond score, such as recency or source weighting
- The exact persistent storage interface and index layout
