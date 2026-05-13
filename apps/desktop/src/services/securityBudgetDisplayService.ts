import type { TokenCostSummary } from "@cialloclaw/protocol";
import { loadStoredValue, saveStoredValue } from "@/platform/storage";

export const SECURITY_BUDGET_DISPLAY_SETTINGS_KEY = "cialloclaw.security-budget-display";

export type SecurityBudgetDisplaySettings = {
  today_tokens: number | null;
  today_cost: number | null;
  single_task_limit: number | null;
  daily_limit: number | null;
};

type StoredSecurityBudgetDisplaySettings = Partial<Record<keyof SecurityBudgetDisplaySettings, number | string | null>>;

/**
 * Returns the local-only defaults used when no budget-display override exists.
 *
 * @returns Empty budget-display settings.
 */
export function buildDefaultSecurityBudgetDisplaySettings(): SecurityBudgetDisplaySettings {
  return {
    today_tokens: null,
    today_cost: null,
    single_task_limit: null,
    daily_limit: null,
  };
}

function normalizeBudgetDisplayInteger(value: unknown): number | null {
  if (value === null || value === undefined || value === "") {
    return null;
  }

  const parsedValue = typeof value === "number" ? value : Number.parseFloat(String(value).trim());
  if (!Number.isFinite(parsedValue) || parsedValue < 0) {
    return null;
  }

  return Math.trunc(parsedValue);
}

function normalizeBudgetDisplayDecimal(value: unknown): number | null {
  if (value === null || value === undefined || value === "") {
    return null;
  }

  const parsedValue = typeof value === "number" ? value : Number.parseFloat(String(value).trim());
  if (!Number.isFinite(parsedValue) || parsedValue < 0) {
    return null;
  }

  return Number(parsedValue.toFixed(2));
}

/**
 * Normalizes locally stored budget-display settings into a stable shape.
 *
 * @param settings Raw local storage payload.
 * @returns Sanitized budget-display settings.
 */
export function normalizeSecurityBudgetDisplaySettings(
  settings: StoredSecurityBudgetDisplaySettings | SecurityBudgetDisplaySettings | null | undefined,
): SecurityBudgetDisplaySettings {
  const defaults = buildDefaultSecurityBudgetDisplaySettings();

  return {
    today_tokens: normalizeBudgetDisplayInteger(settings?.today_tokens) ?? defaults.today_tokens,
    today_cost: normalizeBudgetDisplayDecimal(settings?.today_cost) ?? defaults.today_cost,
    single_task_limit: normalizeBudgetDisplayInteger(settings?.single_task_limit) ?? defaults.single_task_limit,
    daily_limit: normalizeBudgetDisplayInteger(settings?.daily_limit) ?? defaults.daily_limit,
  };
}

/**
 * Loads the local budget-display overrides used by desktop presentation flows.
 *
 * @returns The persisted budget-display settings.
 */
export function loadSecurityBudgetDisplaySettings(): SecurityBudgetDisplaySettings {
  return normalizeSecurityBudgetDisplaySettings(loadStoredValue<StoredSecurityBudgetDisplaySettings>(SECURITY_BUDGET_DISPLAY_SETTINGS_KEY));
}

/**
 * Persists the latest local budget-display overrides.
 *
 * @param settings The overrides to store locally.
 * @returns The normalized snapshot that was saved.
 */
export function saveSecurityBudgetDisplaySettings(settings: SecurityBudgetDisplaySettings): SecurityBudgetDisplaySettings {
  const normalizedSettings = normalizeSecurityBudgetDisplaySettings(settings);
  saveStoredValue(SECURITY_BUDGET_DISPLAY_SETTINGS_KEY, normalizedSettings);
  return normalizedSettings;
}

/**
 * Overlays local budget-display values onto the formal token-cost summary
 * without mutating the RPC payload shape.
 *
 * @param summary The formal token-cost summary.
 * @param settings The local-only presentation overrides.
 * @returns The merged token-cost summary.
 */
export function applySecurityBudgetDisplaySettings(
  summary: TokenCostSummary,
  settings: SecurityBudgetDisplaySettings,
): TokenCostSummary {
  return {
    ...summary,
    ...(settings.today_tokens === null ? {} : { today_tokens: settings.today_tokens }),
    ...(settings.today_cost === null ? {} : { today_cost: settings.today_cost }),
    ...(settings.single_task_limit === null ? {} : { single_task_limit: settings.single_task_limit }),
    ...(settings.daily_limit === null ? {} : { daily_limit: settings.daily_limit }),
  };
}
