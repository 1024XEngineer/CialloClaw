import { loadStoredValue, saveStoredValue } from "@/platform/storage";
import { FLOATING_MIRROR_DIRECTION_KEYS, type FloatingMirrorDirectionKey } from "./mirrorDirections";

type MirrorStoredPosition = {
  x: number;
  y: number;
};

type MirrorLayoutSnapshot = {
  version: 1;
  positions: Record<FloatingMirrorDirectionKey, MirrorStoredPosition>;
};

const MIRROR_LAYOUT_STORAGE_KEY = "cialloclaw.mirror.layout";

function isStoredPosition(value: unknown): value is MirrorStoredPosition {
  if (!value || typeof value !== "object") {
    return false;
  }

  const candidate = value as Record<string, unknown>;
  return typeof candidate.x === "number" && Number.isFinite(candidate.x) && typeof candidate.y === "number" && Number.isFinite(candidate.y);
}

export function loadMirrorFloatingPositions(): Record<FloatingMirrorDirectionKey, MirrorStoredPosition> | null {
  try {
    const snapshot = loadStoredValue<unknown>(MIRROR_LAYOUT_STORAGE_KEY);

    if (!snapshot || typeof snapshot !== "object") {
      return null;
    }

    const candidate = snapshot as Record<string, unknown>;

    if (candidate.version !== 1 || !candidate.positions || typeof candidate.positions !== "object") {
      return null;
    }

    const positions = candidate.positions as Record<string, unknown>;
    const nextPositions = {} as Record<FloatingMirrorDirectionKey, MirrorStoredPosition>;

    for (const key of FLOATING_MIRROR_DIRECTION_KEYS) {
      const value = positions[key];

      if (!isStoredPosition(value)) {
        return null;
      }

      nextPositions[key] = value;
    }

    return nextPositions;
  } catch {
    return null;
  }
}

export function saveMirrorFloatingPositions(positions: Record<FloatingMirrorDirectionKey, MirrorStoredPosition>) {
  saveStoredValue<MirrorLayoutSnapshot>(MIRROR_LAYOUT_STORAGE_KEY, {
    version: 1,
    positions,
  });
}
