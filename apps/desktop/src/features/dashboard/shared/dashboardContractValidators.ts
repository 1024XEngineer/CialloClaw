import { APPROVAL_STATUSES, RISK_LEVELS } from "@cialloclaw/protocol";
import type { ApprovalRequest, Artifact, MirrorReference, RecoveryPoint, TaskStep } from "@cialloclaw/protocol";

type Guard<T> = (value: unknown) => value is T;
const approvalStatuses = new Set<string>(APPROVAL_STATUSES);
const riskLevels = new Set<string>(RISK_LEVELS);

export function isBinaryPendingAuthorizations(value: unknown): value is 0 | 1 {
  return value === 0 || value === 1;
}

export function isApprovalRequest(value: unknown): value is ApprovalRequest {
  if (!value || typeof value !== "object") {
    return false;
  }

  const candidate = value as Partial<ApprovalRequest>;
  return (
    typeof candidate.approval_id === "string" &&
    typeof candidate.task_id === "string" &&
    typeof candidate.operation_name === "string" &&
    typeof candidate.risk_level === "string" &&
    riskLevels.has(candidate.risk_level) &&
    typeof candidate.target_object === "string" &&
    typeof candidate.reason === "string" &&
    typeof candidate.status === "string" &&
    approvalStatuses.has(candidate.status) &&
    typeof candidate.created_at === "string"
  );
}

export function isRecoveryPoint(value: unknown): value is RecoveryPoint {
  if (!value || typeof value !== "object") {
    return false;
  }

  const candidate = value as Partial<RecoveryPoint>;
  return (
    typeof candidate.recovery_point_id === "string" &&
    typeof candidate.task_id === "string" &&
    typeof candidate.summary === "string" &&
    typeof candidate.created_at === "string" &&
    Array.isArray(candidate.objects) &&
    candidate.objects.every((item) => typeof item === "string")
  );
}

export function isArtifact(value: unknown): value is Artifact {
  if (!value || typeof value !== "object") {
    return false;
  }

  const candidate = value as Partial<Artifact>;
  return (
    typeof candidate.artifact_id === "string" &&
    typeof candidate.task_id === "string" &&
    typeof candidate.artifact_type === "string" &&
    typeof candidate.title === "string" &&
    typeof candidate.path === "string" &&
    typeof candidate.mime_type === "string"
  );
}

export function isMirrorReference(value: unknown): value is MirrorReference {
  if (!value || typeof value !== "object") {
    return false;
  }

  const candidate = value as Partial<MirrorReference>;
  return typeof candidate.memory_id === "string" && typeof candidate.reason === "string" && typeof candidate.summary === "string";
}

export function isTaskStep(value: unknown, taskStepStatuses: ReadonlySet<string>): value is TaskStep {
  if (!value || typeof value !== "object") {
    return false;
  }

  const candidate = value as Partial<TaskStep>;
  return (
    typeof candidate.step_id === "string" &&
    typeof candidate.task_id === "string" &&
    typeof candidate.name === "string" &&
    typeof candidate.order_index === "number" &&
    typeof candidate.input_summary === "string" &&
    typeof candidate.output_summary === "string" &&
    typeof candidate.status === "string" &&
    taskStepStatuses.has(candidate.status)
  );
}

export function normalizeNullable<T>(value: unknown, guard: Guard<T>, label: string): T | null {
  if (value === null || value === undefined) {
    return null;
  }

  if (!guard(value)) {
    throw new Error(`${label} is invalid`);
  }

  return value;
}

export function normalizeArray<T>(value: unknown, guard: Guard<T>, label: string): T[] {
  if (!Array.isArray(value)) {
    throw new Error(`${label} is invalid`);
  }

  for (let index = 0; index < value.length; index += 1) {
    if (!guard(value[index])) {
      throw new Error(`${label}[${index}] is invalid`);
    }
  }

  return value;
}
