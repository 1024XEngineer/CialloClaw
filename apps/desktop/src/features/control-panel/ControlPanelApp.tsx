/**
 * ControlPanelApp renders the desktop settings surface with a sidebar-driven
 * layout while preserving the existing draft, inspection, and save flows.
 */
import { useEffect, useRef, useState, type PointerEvent as ReactPointerEvent, type ReactNode } from "react";
import { Window } from "@tauri-apps/api/window";
import {
  BrainCircuit,
  CircleHelp,
  GripHorizontal,
  PlugZap,
  Power,
  RefreshCw,
  Settings2,
  ShieldCheck,
  Sparkles,
  Workflow,
  X,
  type LucideIcon,
} from "lucide-react";
import { Button, Heading, Select, Slider, Text, TextArea, TextField } from "@radix-ui/themes";
import isEqual from "react-fast-compare";
import {
  copyControlPanelAboutValue,
  getControlPanelAboutFeedbackChannels,
  getControlPanelAboutFallbackSnapshot,
  loadControlPanelAboutSnapshot,
  runControlPanelAboutAction,
  type ControlPanelAboutAction,
  type ControlPanelAboutFeedbackChannel,
  type ControlPanelAboutSnapshot,
} from "@/services/controlPanelAboutService";
import {
  buildControlPanelRestoreDefaultsData,
  ControlPanelSaveError,
  loadControlPanelData,
  type ControlPanelData,
  runControlPanelInspection,
  saveControlPanelData,
  validateControlPanelModel,
  type ControlPanelModelValidationOptions,
  type ControlPanelSaveResult,
} from "@/services/controlPanelService";
import {
  loadControlPanelPluginDetail,
  loadControlPanelPluginSnapshot,
  saveControlPanelPluginMockEnabled,
  type ControlPanelPluginDetail,
  type ControlPanelPluginSnapshot,
  type ControlPanelPluginSummary,
} from "@/services/controlPanelPluginService";
import {
  buildDefaultSecurityBudgetDisplaySettings,
  loadSecurityBudgetDisplaySettings,
  saveSecurityBudgetDisplaySettings,
  type SecurityBudgetDisplaySettings,
} from "@/services/securityBudgetDisplayService";
import { loadDesktopRuntimeDefaultsSnapshot, loadHydratedSettings } from "@/services/settingsService";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { buildDesktopOnboardingPresentation } from "@/features/onboarding/onboardingGeometry";
import {
  advanceDesktopOnboarding,
  startManualDesktopOnboardingReplay,
  setDesktopOnboardingPresentation,
} from "@/features/onboarding/onboardingService";
import { useDesktopOnboardingActions } from "@/features/onboarding/useDesktopOnboardingActions";
import { useDesktopOnboardingSession } from "@/features/onboarding/useDesktopOnboardingSession";
import { openDesktopRuntimeWorkspaceDirectory } from "@/platform/desktopRuntimeDefaults";
import { requestCurrentDesktopWindowClose, startCurrentDesktopWindowDragging } from "@/platform/desktopWindowFrame";
import { ensureOnboardingWindow } from "@/platform/onboardingWindowController";
import "./controlPanel.css";

type ControlPanelSectionId = "general" | "desktop" | "memory" | "automation" | "plugins" | "models" | "about";
type ControlPanelAppearance = "light" | "dark";

type NavigationGroup = {
  label: string;
  items: ControlPanelSectionId[];
};

type SectionMeta = {
  group: string;
  icon: LucideIcon;
  navLabel: string;
  title: string;
};

type StatusPillProps = {
  children: ReactNode;
  tone: "danger" | "live" | "mock" | "pending" | "synced" | "warning";
};

type ModelValidationFeedback = {
  message: string;
  tone: "neutral" | "warning";
};

type SidebarItemProps = {
  active: boolean;
  item: SectionMeta;
  onSelect: () => void;
};

type SettingsCardProps = {
  children: ReactNode;
  className?: string;
  description?: string;
  title: string;
};

type ControlLineProps = {
  children: ReactNode;
  className?: string;
  disabled?: boolean;
  hint?: string;
  label: string;
};

type ToggleLineProps = {
  checked: boolean;
  disabled?: boolean;
  description?: string;
  label: string;
  onCheckedChange: (checked: boolean) => void;
};

type InfoRowProps = {
  label: string;
  value: ReactNode;
};

type TimeIntervalInputProps = {
  interval: {
    unit: string;
    value: number;
  };
  onUnitChange: (unit: string) => void;
  onValueChange: (value: number) => void;
};

type BudgetDisplayFieldKey = keyof SecurityBudgetDisplaySettings;

type ChoiceOption<T extends string = string> = {
  label: string;
  value: T;
};

type PluginHealthTone = "danger" | "live" | "synced" | "warning";

const LANGUAGE_OPTIONS = [
  { label: "简体中文", value: "zh-CN" },
  { label: "English", value: "en-US" },
] as const;

const MEMORY_LIFECYCLE_OPTIONS = [
  { label: "3 天", value: "3d" },
  { label: "7 天", value: "7d" },
  { label: "15 天", value: "15d" },
  { label: "30 天", value: "30d" },
] as const;

const INSPECTION_INTERVAL_OPTIONS = [
  { label: "15 分钟", unit: "minute", value: 15 },
  { label: "30 分钟", unit: "minute", value: 30 },
  { label: "1 小时", unit: "hour", value: 1 },
  { label: "6 小时", unit: "hour", value: 6 },
  { label: "1 天", unit: "day", value: 1 },
] as const;

const THEME_MODE_OPTIONS = [
  { label: "跟随系统", value: "follow_system" },
  { label: "浅色", value: "light" },
  { label: "深色", value: "dark" },
] as const satisfies readonly ChoiceOption<"follow_system" | "light" | "dark">[];

const POSITION_MODE_OPTIONS = [
  { label: "固定", value: "fixed" },
  { label: "可拖动", value: "draggable" },
] as const satisfies readonly ChoiceOption<"fixed" | "draggable">[];

const FLOATING_BALL_SIZE_VALUES = ["small", "medium", "large"] as const;
const CONTROL_PANEL_ABOUT_FEEDBACK_CHANNELS = getControlPanelAboutFeedbackChannels();

const DEFAULT_TIME_UNIT_OPTIONS = [
  { label: "分钟", value: "minute" },
  { label: "小时", value: "hour" },
  { label: "天", value: "day" },
  { label: "周", value: "week" },
  { label: "个月", value: "month" },
] as const satisfies readonly ChoiceOption[];

const TIME_UNIT_LABELS: Record<string, string> = {
  minute: "分钟",
  hour: "小时",
  day: "天",
  week: "周",
  month: "个月",
};

function buildInspectionIntervalOptionValue(interval: { unit: string; value: number }) {
  return `${interval.value}:${interval.unit}`;
}

function parseInspectionIntervalOptionValue(optionValue: string) {
  const matchedOption = INSPECTION_INTERVAL_OPTIONS.find(
    (option) => buildInspectionIntervalOptionValue(option) === optionValue,
  );

  if (matchedOption) {
    return {
      unit: matchedOption.unit,
      value: matchedOption.value,
    };
  }

  return INSPECTION_INTERVAL_OPTIONS[0];
}

function buildTimeUnitOptions(currentUnit: string): ChoiceOption[] {
  if (DEFAULT_TIME_UNIT_OPTIONS.some((option) => option.value === currentUnit)) {
    return [...DEFAULT_TIME_UNIT_OPTIONS];
  }

  return [
    ...DEFAULT_TIME_UNIT_OPTIONS,
    {
      label: TIME_UNIT_LABELS[currentUnit] ?? currentUnit,
      value: currentUnit,
    },
  ];
}

function normalizeIntervalNumberInput(rawValue: string, fallbackValue: number) {
  const parsedValue = Number.parseInt(rawValue, 10);

  if (!Number.isFinite(parsedValue) || parsedValue < 1) {
    return fallbackValue;
  }

  return parsedValue;
}

function normalizeBudgetDisplayNumberInput(rawValue: string, allowDecimal: boolean) {
  const trimmedValue = rawValue.trim();
  if (trimmedValue.length === 0) {
    return null;
  }

  const parsedValue = allowDecimal ? Number.parseFloat(trimmedValue) : Number.parseInt(trimmedValue, 10);
  if (!Number.isFinite(parsedValue) || parsedValue < 0) {
    return null;
  }

  return allowDecimal ? Number(parsedValue.toFixed(2)) : Math.trunc(parsedValue);
}

function formatBudgetDisplayInputValue(value: number | null) {
  return value === null ? "" : String(value);
}

function normalizeDisplayPath(rawPath: string) {
  // Runtime path payloads are display-only here, so trim Windows extended-path
  // prefixes away instead of leaking host-internal `//?/` forms into the UI.
  const trimmed = rawPath.trim();
  if (trimmed.startsWith("//?/UNC/")) {
    return `//${trimmed.slice("//?/UNC/".length)}`;
  }

  if (trimmed.startsWith("//?/")) {
    return trimmed.slice("//?/".length);
  }

  return trimmed;
}

function getFloatingBallSizeSliderValue(size: string) {
  const matchedIndex = FLOATING_BALL_SIZE_VALUES.indexOf(size as (typeof FLOATING_BALL_SIZE_VALUES)[number]);
  return matchedIndex === -1 ? 1 : matchedIndex;
}

function getFloatingBallSizeFromSliderValue(value: number | undefined) {
  if (value === 0) {
    return "small";
  }

  if (value === 2) {
    return "large";
  }

  return "medium";
}

const SECTION_META: Record<ControlPanelSectionId, SectionMeta> = {
  automation: {
    group: "协作策略",
    icon: Workflow,
    navLabel: "任务巡检",
    title: "任务与巡检",
  },
  desktop: {
    group: "基础控制",
    icon: Sparkles,
    navLabel: "悬浮球",
    title: "悬浮球",
  },
  general: {
    group: "基础控制",
    icon: Settings2,
    navLabel: "通用设置",
    title: "通用设置",
  },
  memory: {
    group: "协作策略",
    icon: BrainCircuit,
    navLabel: "镜子记忆",
    title: "记忆设置",
  },
  plugins: {
    group: "治理与应用",
    icon: PlugZap,
    navLabel: "插件扩展",
    title: "插件扩展",
  },
  about: {
    group: "治理与应用",
    icon: CircleHelp,
    navLabel: "关于",
    title: "关于",
  },
  models: {
    group: "治理与应用",
    icon: ShieldCheck,
    navLabel: "模型与安全",
    title: "模型与安全",
  },
};

const NAVIGATION_GROUPS: NavigationGroup[] = [
  {
    label: "基础控制",
    items: ["general", "desktop"],
  },
  {
    label: "协作策略",
    items: ["memory", "automation"],
  },
  {
    label: "治理与应用",
    items: ["plugins", "models", "about"],
  },
];

/**
 * Resolves the save feedback copy shown after settings are persisted.
 *
 * @param applyMode Backend apply mode returned by the settings snapshot.
 * @param needRestart Whether the current change set requires an app restart.
 * @returns User-facing save feedback copy.
 */
function getApplyModeCopy(applyMode: string, needRestart: boolean) {
  if (needRestart) {
    return "部分设置需要重启桌面端后生效。";
  }

  if (applyMode === "next_task_effective") {
    return "设置已保存，将在下一个任务周期生效。";
  }

  return "设置已即时生效。";
}

function buildLocalInspectorFallback(settings: ControlPanelData["settings"]): ControlPanelData["inspector"] {
  return {
    task_sources: settings.task_automation.task_sources,
    inspection_interval: settings.task_automation.inspection_interval,
    inspect_on_file_change: settings.task_automation.inspect_on_file_change,
    inspect_on_startup: settings.task_automation.inspect_on_startup,
    remind_before_deadline: settings.task_automation.remind_before_deadline,
    remind_when_stale: settings.task_automation.remind_when_stale,
  };
}

/**
 * Keeps the control-panel shell renderable when the RPC bootstrap fails by
 * falling back to the last persisted local snapshot under an explicit error banner.
 */
async function buildLocalControlPanelSnapshot(): Promise<ControlPanelData> {
  const settings = (await loadHydratedSettings()).settings;
  const runtimeDefaults = await loadDesktopRuntimeDefaultsSnapshot();

  return {
    settings,
    inspector: buildLocalInspectorFallback(settings),
    providerApiKeyInput: "",
    runtimeWorkspacePath: runtimeDefaults?.workspace_path ?? null,
    securitySummary: {
      security_status: "execution_error",
      pending_authorizations: 0,
      latest_restore_point: null,
      token_cost_summary: {
        current_task_tokens: 0,
        current_task_cost: 0,
        today_tokens: 0,
        today_cost: 0,
        single_task_limit: 0,
        daily_limit: 0,
        budget_auto_downgrade: settings.models.budget_auto_downgrade,
      },
    },
    source: "rpc",
    warnings: [],
  };
}

function shouldSurfaceRpcErrorBanner(message: string) {
  return message.includes("暂时不可用") || message.includes("重新获取最新配置失败");
}

function resolveControlPanelAppearance(
  themeMode: ControlPanelData["settings"]["general"]["theme_mode"],
  systemAppearance: ControlPanelAppearance,
): ControlPanelAppearance {
  if (themeMode === "dark") {
    return "dark";
  }

  if (themeMode === "light") {
    return "light";
  }

  return systemAppearance;
}

function StatusPill({ children, tone }: StatusPillProps) {
  return <span className={`control-panel-shell__status-pill control-panel-shell__status-pill--${tone}`}>{children}</span>;
}

function HelpTooltip({ content }: { content: string }) {
  return (
    <Tooltip>
      <TooltipTrigger className="control-panel-shell__help-trigger" aria-label={content}>
        <CircleHelp size={14} strokeWidth={1.75} />
      </TooltipTrigger>
      <TooltipContent className="control-panel-shell__tooltip">{content}</TooltipContent>
    </Tooltip>
  );
}

function SidebarItem({ active, item, onSelect }: SidebarItemProps) {
  const Icon = item.icon;

  return (
    <button
      type="button"
      className="control-panel-shell__nav-item"
      data-active={active ? "true" : "false"}
      onClick={onSelect}
    >
      <span className="control-panel-shell__nav-icon" aria-hidden="true">
        <Icon size={17} strokeWidth={1.85} />
      </span>
      <span className="control-panel-shell__nav-label">{item.navLabel}</span>
    </button>
  );
}

function SettingsCard({ children, className, description, title }: SettingsCardProps) {
  return (
    <section className={className ? `control-panel-shell__card ${className}` : "control-panel-shell__card"}>
      <div className="control-panel-shell__card-header">
        <div className="control-panel-shell__title-row">
          <Heading as="h2" size="4" className="control-panel-shell__card-title">
            {title}
          </Heading>
          {description ? <HelpTooltip content={description} /> : null}
        </div>
      </div>
      <div className="control-panel-shell__card-body">{children}</div>
    </section>
  );
}

function ControlLine({ children, className, disabled = false, hint, label }: ControlLineProps) {
  const classes = ["control-panel-shell__row", className].filter(Boolean).join(" ");

  return (
    <div className={classes} data-disabled={disabled ? "true" : "false"}>
      <div className="control-panel-shell__row-copy">
        <div className="control-panel-shell__title-row control-panel-shell__title-row--field">
          <Text as="p" size="2" weight="medium" className="control-panel-shell__row-label">
            {label}
          </Text>
          {hint ? <HelpTooltip content={hint} /> : null}
        </div>
      </div>
      <div className="control-panel-shell__row-field">{children}</div>
    </div>
  );
}

function ToggleLine({ checked, description, disabled = false, label, onCheckedChange }: ToggleLineProps) {
  return (
    <div className="control-panel-shell__row control-panel-shell__row--toggle" data-disabled={disabled ? "true" : "false"}>
      <div className="control-panel-shell__row-copy">
        <div className="control-panel-shell__title-row control-panel-shell__title-row--field">
          <Text as="p" size="2" weight="medium" className="control-panel-shell__row-label">
            {label}
          </Text>
          {description ? <HelpTooltip content={description} /> : null}
        </div>
      </div>
      <div className="control-panel-shell__row-field control-panel-shell__row-field--inline">
        <button
          type="button"
          role="switch"
          aria-disabled={disabled}
          aria-checked={checked}
          className="control-panel-shell__toggle"
          data-state={checked ? "checked" : "unchecked"}
          disabled={disabled}
          onClick={() => onCheckedChange(!checked)}
        >
          <span className="control-panel-shell__toggle-handle" aria-hidden="true" />
        </button>
      </div>
    </div>
  );
}

function ChoiceGroup<T extends string>({
  className,
  options,
  value,
  onValueChange,
}: {
  className?: string;
  options: readonly ChoiceOption<T>[];
  value: T;
  onValueChange: (value: T) => void;
}) {
  const classes = ["control-panel-shell__choice-group", className].filter(Boolean).join(" ");

  return (
    <div className={classes} role="radiogroup">
      {options.map((option) => {
        const checked = option.value === value;

        return (
          <button
            key={option.value}
            type="button"
            role="radio"
            aria-checked={checked}
            className="control-panel-shell__choice-option"
            data-state={checked ? "checked" : "unchecked"}
            onClick={() => onValueChange(option.value)}
          >
            <span className="control-panel-shell__choice-label">{option.label}</span>
          </button>
        );
      })}
    </div>
  );
}

function TimeIntervalInput({ interval, onUnitChange, onValueChange }: TimeIntervalInputProps) {
  const unitOptions = buildTimeUnitOptions(interval.unit);

  return (
    <div className="control-panel-shell__interval-field">
      <TextField.Root
        className="control-panel-shell__input control-panel-shell__input--compact"
        type="number"
        min={1}
        step={1}
        inputMode="numeric"
        value={String(interval.value)}
        aria-label="间隔数值"
        onChange={(event) => onValueChange(normalizeIntervalNumberInput(event.target.value, interval.value))}
      />

      <Select.Root value={interval.unit} onValueChange={onUnitChange}>
        <Select.Trigger className="control-panel-shell__select-trigger" radius="full" />
        <Select.Content className="control-panel-shell__select-content" position="popper">
          {unitOptions.map((option) => (
            <Select.Item key={option.value} value={option.value}>
              {option.label}
            </Select.Item>
          ))}
        </Select.Content>
      </Select.Root>
    </div>
  );
}

function InfoRow({ label, value }: InfoRowProps) {
  return (
    <div className="control-panel-shell__info-row">
      <Text as="span" size="2" className="control-panel-shell__info-label">
        {label}
      </Text>
      <div className="control-panel-shell__info-value">{value}</div>
    </div>
  );
}

function formatPluginHealthLabel(health: ControlPanelPluginSummary["runtime_health"]) {
  switch (health) {
    case "healthy":
      return "healthy";
    case "degraded":
      return "degraded";
    case "failed":
      return "failed";
    case "stopped":
      return "stopped";
    case "unavailable":
      return "unavailable";
    default:
      return "unknown";
  }
}

function getPluginHealthTone(health: ControlPanelPluginSummary["runtime_health"]): PluginHealthTone {
  switch (health) {
    case "healthy":
      return "live";
    case "degraded":
      return "warning";
    case "failed":
      return "danger";
    default:
      return "synced";
  }
}

function formatPluginSourceLabel(source: ControlPanelPluginSummary["plugin"]["source"]) {
  switch (source) {
    case "builtin":
      return "Built-in";
    case "local_dir":
      return "Local";
    case "github":
      return "GitHub";
    case "marketplace":
      return "Marketplace";
    default:
      return source;
  }
}

function formatPluginRuntimeKindLabel(kind: string) {
  switch (kind) {
    case "worker":
      return "Worker";
    case "sidecar":
      return "Sidecar";
    default:
      return kind;
  }
}

function formatPluginRiskHintLabel(riskHint: string) {
  switch (riskHint) {
    case "green":
      return "green";
    case "yellow":
      return "yellow";
    case "red":
      return "red";
    default:
      return riskHint;
  }
}

function formatPluginControlLabel(plugin: { control: ControlPanelPluginSummary["control"] }) {
  if (plugin.control.source === "mock") {
    return plugin.control.effective_enabled ? "mock started" : "mock stopped";
  }

  return plugin.control.effective_enabled ? "live enabled" : "live disabled";
}

function formatPluginTimestampLabel(timestamp: string | null) {
  if (!timestamp) {
    return "暂无记录";
  }

  const parsed = Date.parse(timestamp);
  if (Number.isNaN(parsed)) {
    return timestamp;
  }

  return new Date(parsed).toLocaleString("zh-CN", {
    hour12: false,
  });
}

function FeedbackChannelCard({ channel, onCopyLink }: { channel: ControlPanelAboutFeedbackChannel; onCopyLink: (url: string) => void }) {
  return (
    <article className="control-panel-shell__feedback-card" data-kind={channel.kind}>
      <div className="control-panel-shell__feedback-card-copy">
        <Text as="p" size="2" weight="medium" className="control-panel-shell__feedback-card-title">
          {channel.title}
        </Text>
        <Text as="p" size="1" className="control-panel-shell__feedback-card-description">
          {channel.description}
        </Text>
      </div>

      {channel.kind === "link" ? (
        <div className="control-panel-shell__feedback-card-body">
          <button
            type="button"
            className="control-panel-shell__feedback-link-button"
            onClick={() => onCopyLink(channel.href)}
          >
            {channel.actionLabel}
          </button>
          <code className="control-panel-shell__feedback-link-copy">{channel.hrefLabel}</code>
        </div>
      ) : null}

      {channel.kind === "image" ? (
        <div className="control-panel-shell__feedback-card-body">
          <img className="control-panel-shell__feedback-image" src={channel.previewSrc} alt={channel.previewAlt} />
          {channel.note ? (
            <Text as="p" size="1" className="control-panel-shell__feedback-note">
              {channel.note}
            </Text>
          ) : null}
        </div>
      ) : null}

      {channel.kind === "placeholder" ? (
        <div className="control-panel-shell__feedback-card-body">
          <div className="control-panel-shell__feedback-placeholder" aria-hidden="true">
            <span>{channel.placeholderLabel}</span>
          </div>
          <Text as="p" size="1" className="control-panel-shell__feedback-note">
            {channel.note}
          </Text>
        </div>
      ) : null}
    </article>
  );
}

// applyControlPanelSaveResult keeps unsaved groups intact so partial saves do
// not accidentally discard the user's remaining local edits.
function applyControlPanelSaveResult(base: ControlPanelData, result: ControlPanelSaveResult): ControlPanelData {
  const nextSettings = result.savedSettings || result.savedInspector
    ? {
        ...base.settings,
        ...(result.savedSettings ? result.effectiveSettings : {}),
        task_automation: {
          ...base.settings.task_automation,
          ...(result.effectiveSettings.task_automation ?? {}),
        },
      }
    : base.settings;

  return {
    ...base,
    inspector: result.savedInspector ? result.effectiveInspector : base.inspector,
    providerApiKeyInput: result.savedSettings ? "" : base.providerApiKeyInput,
    runtimeWorkspacePath: base.runtimeWorkspacePath,
    settings: nextSettings,
    source: result.source,
    warnings: result.warnings,
  };
}

/**
 * ControlPanelApp renders the desktop settings surface with a sidebar-driven
 * layout while keeping the current settings data model untouched.
 *
 * @returns The desktop control panel window.
 */
export function ControlPanelApp() {
  const onboardingSession = useDesktopOnboardingSession();
  const autoAdvancedControlPanelStepRef = useRef(false);
  const [activeSection, setActiveSection] = useState<ControlPanelSectionId>("general");
  const [aboutSnapshot, setAboutSnapshot] = useState<ControlPanelAboutSnapshot>(() => getControlPanelAboutFallbackSnapshot());
  // About actions only affect local clipboard/help affordances, so their
  // feedback must stay in local UI state instead of polluting formal settings.
  const [aboutActionFeedback, setAboutActionFeedback] = useState<string | null>(null);
  const [workspaceActionFeedback, setWorkspaceActionFeedback] = useState<string | null>(null);
  const [isRestoreDefaultsConfirming, setIsRestoreDefaultsConfirming] = useState(false);
  const [panelData, setPanelData] = useState<ControlPanelData | null>(null);
  const [draft, setDraft] = useState<ControlPanelData | null>(null);
  const [savedBudgetDisplaySettings, setSavedBudgetDisplaySettings] = useState<SecurityBudgetDisplaySettings>(() =>
    loadSecurityBudgetDisplaySettings(),
  );
  const [budgetDisplayDraft, setBudgetDisplayDraft] = useState<SecurityBudgetDisplaySettings>(() =>
    loadSecurityBudgetDisplaySettings(),
  );
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saveFeedback, setSaveFeedback] = useState<string | null>(null);
  const [modelValidationFeedback, setModelValidationFeedback] = useState<ModelValidationFeedback | null>(null);
  const [inspectionSummary, setInspectionSummary] = useState<string | null>(null);
  const [pluginFeedback, setPluginFeedback] = useState<string | null>(null);
  const [pluginSnapshot, setPluginSnapshot] = useState<ControlPanelPluginSnapshot | null>(null);
  const [pluginDetail, setPluginDetail] = useState<ControlPanelPluginDetail | null>(null);
  const [selectedPluginId, setSelectedPluginId] = useState<string | null>(null);
  const [pluginLoadError, setPluginLoadError] = useState<string | null>(null);
  const [pluginReloadToken, setPluginReloadToken] = useState(0);
  const [isSaving, setIsSaving] = useState(false);
  const [isValidatingModel, setIsValidatingModel] = useState(false);
  const [isRunningInspection, setIsRunningInspection] = useState(false);
  const [isReplayingOnboarding, setIsReplayingOnboarding] = useState(false);
  const [isLoadingPlugins, setIsLoadingPlugins] = useState(false);
  const [isLoadingPluginDetail, setIsLoadingPluginDetail] = useState(false);
  const [systemAppearance, setSystemAppearance] = useState<ControlPanelAppearance>(() => {
    if (typeof window === "undefined") {
      return "light";
    }

    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  });

  useEffect(() => {
    if (typeof window === "undefined") {
      return undefined;
    }

    const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
    const updateSystemAppearance = (event?: MediaQueryListEvent) => {
      setSystemAppearance((event?.matches ?? mediaQuery.matches) ? "dark" : "light");
    };

    updateSystemAppearance();

    if (typeof mediaQuery.addEventListener === "function") {
      mediaQuery.addEventListener("change", updateSystemAppearance);
      return () => mediaQuery.removeEventListener("change", updateSystemAppearance);
    }

    mediaQuery.addListener(updateSystemAppearance);
    return () => mediaQuery.removeListener(updateSystemAppearance);
  }, []);

  useEffect(() => {
    let cancelled = false;

    void (async () => {
      try {
        const nextData = await loadControlPanelData();
        const nextBudgetDisplaySettings = loadSecurityBudgetDisplaySettings();

        if (cancelled) {
          return;
        }

        setLoadError(null);
        setPanelData(nextData);
        setDraft(nextData);
        setSavedBudgetDisplaySettings(nextBudgetDisplaySettings);
        setBudgetDisplayDraft(nextBudgetDisplaySettings);
        setModelValidationFeedback(null);
      } catch (error) {
        if (cancelled) {
          return;
        }

        const fallbackData = await buildLocalControlPanelSnapshot();
        if (cancelled) {
          return;
        }
        setLoadError(error instanceof Error ? error.message : "控制面板加载失败。");
        setPanelData((current) => current ?? fallbackData);
        setDraft((current) => current ?? fallbackData);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, []);

  const applyPluginControlState = (pluginId: string, effectiveControl: ControlPanelPluginSummary["control"]) => {
    setPluginSnapshot((current) => {
      if (!current) {
        return current;
      }

      const items = current.items.map((item) =>
        item.plugin.plugin_id === pluginId
          ? {
              ...item,
              control: effectiveControl,
            }
          : item,
      );

      return {
        ...current,
        items,
        summary: {
          ...current.summary,
          live_enabled: current.summary.live_enabled,
          mock_overrides: items.filter((item) => item.control.source === "mock").length,
          total: items.length,
        },
      };
    });
    setPluginDetail((current) =>
      current?.plugin.plugin_id === pluginId
        ? {
            ...current,
            control: effectiveControl,
          }
        : current,
    );
  };

  useEffect(() => {
    let cancelled = false;

    setIsLoadingPlugins(true);
    void loadControlPanelPluginSnapshot().then(
      (snapshot) => {
        if (cancelled) {
          return;
        }

        setPluginSnapshot(snapshot);
        setPluginLoadError(null);
        if (pluginReloadToken > 0) {
          setPluginFeedback("插件运行态已刷新。");
        }
      },
      (error: unknown) => {
        if (!cancelled) {
          setPluginLoadError(error instanceof Error ? error.message : "插件扩展加载失败。");
        }
      },
    ).finally(() => {
      if (!cancelled) {
        setIsLoadingPlugins(false);
      }
    });

    return () => {
      cancelled = true;
    };
  }, [pluginReloadToken]);

  useEffect(() => {
    if (!pluginSnapshot) {
      return;
    }

    if (pluginSnapshot.items.length === 0) {
      setSelectedPluginId(null);
      setPluginDetail(null);
      return;
    }

    setSelectedPluginId((current) =>
      pluginSnapshot.items.some((item) => item.plugin.plugin_id === current)
        ? current
        : pluginSnapshot.items[0].plugin.plugin_id,
    );
  }, [pluginSnapshot]);

  useEffect(() => {
    if (!selectedPluginId) {
      setPluginDetail(null);
      return;
    }

    let cancelled = false;
    setPluginDetail(null);
    setIsLoadingPluginDetail(true);

    void loadControlPanelPluginDetail(selectedPluginId).then(
      (detail) => {
        if (!cancelled) {
          setPluginLoadError(null);
          setPluginDetail(detail);
        }
      },
      (error: unknown) => {
        if (!cancelled) {
          setPluginLoadError(error instanceof Error ? error.message : "插件详情加载失败。");
        }
      },
    ).finally(() => {
      if (!cancelled) {
        setIsLoadingPluginDetail(false);
      }
    });

    return () => {
      cancelled = true;
    };
  }, [selectedPluginId]);

  useEffect(() => {
    let cancelled = false;

    void ensureOnboardingWindow().catch((error) => {
      if (!cancelled) {
        console.warn("control-panel onboarding prewarm failed", error);
      }
    });

    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;

    void loadControlPanelAboutSnapshot().then((nextSnapshot) => {
      if (!cancelled) {
        setAboutSnapshot(nextSnapshot);
      }
    });

    return () => {
      cancelled = true;
    };
  }, []);

  const handleReload = async () => {
    setLoadError(null);

    try {
      const nextData = await loadControlPanelData();
      const nextBudgetDisplaySettings = loadSecurityBudgetDisplaySettings();
      setLoadError(null);
      setPanelData(nextData);
      setDraft(nextData);
      setSavedBudgetDisplaySettings(nextBudgetDisplaySettings);
      setBudgetDisplayDraft(nextBudgetDisplaySettings);
      setModelValidationFeedback(null);
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : "控制面板加载失败。");
    }
  };

  useEffect(() => {
    if (onboardingSession?.isOpen !== true) {
      autoAdvancedControlPanelStepRef.current = false;
      return;
    }

    if (onboardingSession.step === "tray_hint" && !autoAdvancedControlPanelStepRef.current) {
      autoAdvancedControlPanelStepRef.current = true;
      setActiveSection("models");
      void Window.getByLabel("dashboard").then((windowHandle) => {
        void windowHandle?.close();
      });
      void advanceDesktopOnboarding("control_panel_api_key");
      return;
    }

    if (onboardingSession.step === "control_panel_api_key") {
      autoAdvancedControlPanelStepRef.current = true;
      setActiveSection("models");
    }
  }, [onboardingSession]);

  useDesktopOnboardingActions(
    "control-panel",
    (action) => {
      if (action.type === "close_control_panel") {
        void requestCurrentDesktopWindowClose();
      }
    },
  );

  useEffect(() => {
    if (onboardingSession?.isOpen !== true) {
      return;
    }

    if (onboardingSession.step === "control_panel_api_key" && draft?.settings.models.provider_api_key_configured) {
      void advanceDesktopOnboarding("done");
    }
  }, [draft?.settings.models.provider_api_key_configured, onboardingSession]);

  useEffect(() => {
    if (
      onboardingSession?.isOpen !== true ||
      (onboardingSession.step !== "control_panel_api_key" && onboardingSession.step !== "done")
    ) {
      return;
    }

    void (async () => {
      const presentation = await buildDesktopOnboardingPresentation({
        anchors: [],
        placement: onboardingSession.step === "control_panel_api_key" ? "top-right" : "center",
        step: onboardingSession.step,
        windowLabel: "control-panel",
      });

      if (presentation !== null) {
        await setDesktopOnboardingPresentation(presentation);
      }
    })();
  }, [onboardingSession]);

  const controlPanelAppearance = draft ? resolveControlPanelAppearance(draft.settings.general.theme_mode, systemAppearance) : systemAppearance;

  useEffect(() => {
    if (typeof document === "undefined") {
      return undefined;
    }

    // Tooltip popups render through a portal, so the window appearance needs a
    // document-level marker instead of relying on the local shell subtree.
    document.body.dataset.controlPanelAppearance = controlPanelAppearance;

    return () => {
      delete document.body.dataset.controlPanelAppearance;
    };
  }, [controlPanelAppearance]);

  if (!draft || !panelData) {
    return (
      <main className="app-shell control-panel-shell" data-appearance={controlPanelAppearance}>
        <div className="control-panel-shell__loading">
          <div className="control-panel-shell__loading-stack">
            <Text size="2" className="control-panel-shell__loading-copy">
              {loadError ?? "正在载入控制面板…"}
            </Text>
            {loadError ? (
              <Button className="control-panel-shell__button" variant="soft" onClick={() => void handleReload()}>
                重新加载
              </Button>
            ) : null}
          </div>
        </div>
      </main>
    );
  }

  const activeMeta = SECTION_META[activeSection];
  const inspectorDirty = !isEqual(draft.inspector, panelData.inspector);
  const settingsDirty = !isEqual(draft.settings, panelData.settings) || draft.providerApiKeyInput.trim() !== "";
  const modelSettingsDirty = !isEqual(draft.settings.models, panelData.settings.models) || draft.providerApiKeyInput.trim() !== "";
  const budgetDisplayDirty = !isEqual(budgetDisplayDraft, savedBudgetDisplaySettings);
  const hasChanges = inspectorDirty || settingsDirty || budgetDisplayDirty;
  const providerApiKeyStatus = draft.settings.models.provider_api_key_configured ? "已配置" : "未配置";
  const providerApiKeyHint = "通过 JSON-RPC `agent.settings.update` 提交；只写入后端 Stronghold，不会回显明文。";
  const hasRpcLoadError = loadError !== null;
  const onboardingReplayDisabled = isSaving || isRunningInspection || isReplayingOnboarding;
  const runtimeWorkspacePath = draft.runtimeWorkspacePath?.trim() ?? "";
  const runtimeWorkspacePathLabel = runtimeWorkspacePath || "当前运行时工作区暂不可用";
  const canOpenRuntimeWorkspace = runtimeWorkspacePath.length > 0;
  const localDataPath = normalizeDisplayPath(aboutSnapshot.localDataPath ?? "");
  const localDataPathLabel = localDataPath || "当前本地存储目录暂不可用";
  const restoreDefaultsDisabled = isSaving || isRunningInspection || isValidatingModel || isReplayingOnboarding;

  const saveStateValue = hasChanges ? <StatusPill tone="pending">待保存</StatusPill> : <StatusPill tone="synced">已同步</StatusPill>;

  const updateSettings = (updater: (current: ControlPanelData) => ControlPanelData) => {
    setDraft((current) => {
      if (!current) {
        return current;
      }

      const next = updater(current);
      const modelRouteChanged = !isEqual(next.settings.models, current.settings.models) || next.providerApiKeyInput !== current.providerApiKeyInput;
      if (modelRouteChanged) {
        setModelValidationFeedback(null);
      }
      return next;
    });
  };

  const updateBudgetDisplay = (key: BudgetDisplayFieldKey, rawValue: string, allowDecimal = false) => {
    setBudgetDisplayDraft((current) => ({
      ...current,
      [key]: normalizeBudgetDisplayNumberInput(rawValue, allowDecimal),
    }));
  };

  // The custom titlebar is draggable, but embedded controls must keep their own
  // pointer behavior instead of starting a native window move.
  const handleTopbarPointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    const target = event.target as HTMLElement | null;

    if (target?.closest("button, input, textarea, select, [role='switch']")) {
      return;
    }

    void startCurrentDesktopWindowDragging();
  };

  const handleWindowDragPointerDown = (event: ReactPointerEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    void startCurrentDesktopWindowDragging();
  };

  const handleWindowClosePointerDown = (event: ReactPointerEvent<HTMLButtonElement>) => {
    event.stopPropagation();
  };

  const handleWindowCloseClick = () => {
    void requestCurrentDesktopWindowClose();
  };

  const handleReset = () => {
    setDraft(panelData);
    setBudgetDisplayDraft(savedBudgetDisplaySettings);
    setSaveFeedback("已恢复为上次载入的设置快照。");
    setModelValidationFeedback(null);
    setWorkspaceActionFeedback(null);
    setIsRestoreDefaultsConfirming(false);
  };

  const handleOpenCurrentWorkspaceDirectory = async () => {
    if (!canOpenRuntimeWorkspace) {
      setWorkspaceActionFeedback("当前运行时工作区暂不可用。");
      return;
    }

    try {
      await openDesktopRuntimeWorkspaceDirectory();
      setWorkspaceActionFeedback("已在系统中打开当前工作区目录。");
    } catch (error) {
      const message = error instanceof Error ? error.message : "打开当前工作区目录失败。";
      setWorkspaceActionFeedback(`打开当前工作区目录失败：${message}`);
    }
    setIsRestoreDefaultsConfirming(false);
  };

  const handlePrepareRestoreDefaults = () => {
    if (restoreDefaultsDisabled) {
      return;
    }

    setIsRestoreDefaultsConfirming(true);
    setSaveFeedback(null);
  };

  const handleCancelRestoreDefaults = () => {
    setIsRestoreDefaultsConfirming(false);
  };

  const handleValidateModel = async (options: ControlPanelModelValidationOptions = {}) => {
    setIsValidatingModel(true);
    try {
      const result = await validateControlPanelModel(draft, options);
      setLoadError(null);
      setModelValidationFeedback({
        message: result.message,
        tone: result.ok ? "neutral" : "warning",
      });
      return result;
    } catch (error) {
      const message = error instanceof Error ? error.message : "模型配置校验失败，请稍后重试。";
      if (shouldSurfaceRpcErrorBanner(message)) {
        setLoadError(message);
      }
      setModelValidationFeedback({
        message,
        tone: "warning",
      });
      throw error;
    } finally {
      setIsValidatingModel(false);
    }
  };

  const handleReplayOnboarding = () => {
    if (onboardingReplayDisabled) {
      return;
    }

    setIsReplayingOnboarding(true);
    void (async () => {
      try {
        setSaveFeedback(null);
        setLoadError(null);
        let session = await startManualDesktopOnboardingReplay("control-panel");

        if (session === null) {
          const errorMessage = "重新打开新手引导失败。";
          setLoadError(errorMessage);
          setSaveFeedback(errorMessage);
          return;
        }

        await requestCurrentDesktopWindowClose();
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : "重新打开新手引导失败。";
        setLoadError(errorMessage);
        setSaveFeedback(errorMessage);
      } finally {
        setIsReplayingOnboarding(false);
      }
    })();
  };

  const handleSave = async () => {
    if (!hasChanges) {
      return;
    }

    setIsSaving(true);
    try {
      let nextPanelData = panelData;
      let nextDraft = draft;

      if (settingsDirty || inspectorDirty) {
        const result = await saveControlPanelData(draft, {
          confirmedInspector: panelData.inspector,
          saveInspector: inspectorDirty,
          saveSettings: settingsDirty,
          validateModel: modelSettingsDirty,
        });
        nextPanelData = applyControlPanelSaveResult(panelData, result);
        nextDraft = applyControlPanelSaveResult(draft, result);
        setSaveFeedback(getApplyModeCopy(result.applyMode, result.needRestart));
        if (result.modelValidation) {
          setModelValidationFeedback({
            message: result.modelValidation.message,
            tone: result.modelValidation.ok ? "neutral" : "warning",
          });
        }

        // Only a successful RPC-backed save can clear the backend connectivity banner.
        setLoadError(null);
      }

      if (budgetDisplayDirty) {
        const savedBudgetDisplay = saveSecurityBudgetDisplaySettings(budgetDisplayDraft);
        setSavedBudgetDisplaySettings(savedBudgetDisplay);
        setBudgetDisplayDraft(savedBudgetDisplay);
      }

      setPanelData(nextPanelData);
      setDraft(nextDraft);
      if (!settingsDirty && !inspectorDirty) {
        setSaveFeedback("设置已即时生效。");
      }
    } catch (error) {
      if (error instanceof ControlPanelSaveError && error.partialResult) {
        const nextPanelData = applyControlPanelSaveResult(panelData, error.partialResult);
        const nextDraft = applyControlPanelSaveResult(draft, error.partialResult);
        setPanelData(nextPanelData);
        setDraft(nextDraft);
      }

      const errorMessage = error instanceof Error ? error.message : "保存控制面板设置失败。";
      if (error instanceof ControlPanelSaveError && error.kind === "model_validation_failed") {
        setSaveFeedback("模型配置校验未通过，当前设置未保存。");
        setModelValidationFeedback({
          message: errorMessage,
          tone: "warning",
        });
        return;
      }
      if (shouldSurfaceRpcErrorBanner(errorMessage)) {
        setLoadError(errorMessage);
      }
      setSaveFeedback(errorMessage);
    } finally {
      setIsSaving(false);
    }
  };

  const handleRestoreDefaults = async () => {
    if (restoreDefaultsDisabled) {
      return;
    }

    const persistedPanelData = panelData;
    if (!persistedPanelData) {
      return;
    }

    const restoreDraft = buildControlPanelRestoreDefaultsData(draft, persistedPanelData);
    const defaultBudgetDisplaySettings = buildDefaultSecurityBudgetDisplaySettings();

    setIsSaving(true);
    try {
      const result = await saveControlPanelData(restoreDraft, {
        confirmedInspector: persistedPanelData.inspector,
        saveInspector: true,
        saveSettings: true,
        validateModel: false,
      });
      const nextPanelData = applyControlPanelSaveResult(restoreDraft, result);
      const nextDraft = applyControlPanelSaveResult(restoreDraft, result);
      const savedBudgetDisplay = saveSecurityBudgetDisplaySettings(defaultBudgetDisplaySettings);
      setLoadError(null);
      setPanelData(nextPanelData);
      setDraft(nextDraft);
      setSavedBudgetDisplaySettings(savedBudgetDisplay);
      setBudgetDisplayDraft(savedBudgetDisplay);
      setSaveFeedback(`已恢复默认设置。${getApplyModeCopy(result.applyMode, result.needRestart)}`);
      if (result.modelValidation) {
        setModelValidationFeedback({
          message: result.modelValidation.message,
          tone: result.modelValidation.ok ? "neutral" : "warning",
        });
      } else {
        setModelValidationFeedback(null);
      }
    } catch (error) {
      if (error instanceof ControlPanelSaveError && error.partialResult) {
        const nextPanelData = applyControlPanelSaveResult(restoreDraft, error.partialResult);
        const nextDraft = applyControlPanelSaveResult(restoreDraft, error.partialResult);
        setPanelData(nextPanelData);
        setDraft(nextDraft);
      }

      const errorMessage = error instanceof Error ? error.message : "恢复默认设置失败。";
      if (shouldSurfaceRpcErrorBanner(errorMessage)) {
        setLoadError(errorMessage);
      }
      setSaveFeedback(errorMessage);
    } finally {
      setIsSaving(false);
      setIsRestoreDefaultsConfirming(false);
    }
  };

  const handleRunInspection = async () => {
    if (isSaving) {
      return;
    }

    setIsRunningInspection(true);
    try {
      const result = await runControlPanelInspection(draft);
      setInspectionSummary(
        `本次巡检解析 ${result.summary.parsed_files} 个文件，识别 ${result.summary.identified_items} 条事项，逾期 ${result.summary.overdue} 条。`,
      );
    } catch (error) {
      setInspectionSummary(error instanceof Error ? error.message : "手动巡检执行失败。");
    } finally {
      setIsRunningInspection(false);
    }
  };
  void handleRunInspection;

  const handleAboutAction = async (action: ControlPanelAboutAction) => {
    const feedback = await runControlPanelAboutAction(action);
    setAboutActionFeedback(feedback);
  };

  const handleAboutLinkCopy = async (url: string) => {
    const feedback = await copyControlPanelAboutValue(url, "已复制反馈渠道链接。");
    setAboutActionFeedback(feedback);
  };

  const selectedPluginSummary = pluginSnapshot?.items.find((item) => item.plugin.plugin_id === selectedPluginId) ?? null;

  const handlePluginRefresh = () => {
    setPluginReloadToken((current) => current + 1);
  };

  const handlePluginToggle = (plugin: ControlPanelPluginSummary, nextEnabled: boolean) => {
    const nextControl = saveControlPanelPluginMockEnabled(plugin.plugin.plugin_id, nextEnabled, plugin.plugin.enabled);
    applyPluginControlState(plugin.plugin.plugin_id, nextControl);
    setPluginFeedback(
      nextControl.source === "mock"
        ? `已切换 ${plugin.plugin.display_name || plugin.plugin.name} 的 mock 启停状态；实时运行态仍以当前 RPC 返回为准。`
        : `已移除 ${plugin.plugin.display_name || plugin.plugin.name} 的 mock 启停覆盖，页面恢复显示正式状态。`,
    );
  };

  const renderSectionContent = () => {
    switch (activeSection) {
      case "general":
        return (
          <>
            <SettingsCard title="界面偏好" description="语言与主题会影响整个桌面端界面。">
              <ControlLine label="语言" hint="统一控制仪表盘与操作面板界面语言。">
                <Select.Root
                  value={draft.settings.general.language}
                  onValueChange={(value) =>
                    updateSettings((current) => ({
                      ...current,
                      settings: {
                        ...current.settings,
                        general: { ...current.settings.general, language: value },
                      },
                    }))
                  }
                >
                  <Select.Trigger className="control-panel-shell__select-trigger" radius="full" />
                  <Select.Content className="control-panel-shell__select-content" position="popper">
                    {LANGUAGE_OPTIONS.map((option) => (
                      <Select.Item key={option.value} value={option.value}>
                        {option.label}
                      </Select.Item>
                    ))}
                  </Select.Content>
                </Select.Root>
              </ControlLine>

              <ControlLine label="主题" hint="支持跟随系统或直接指定浅色、深色。">
                <ChoiceGroup
                  value={draft.settings.general.theme_mode}
                  onValueChange={(value) =>
                    updateSettings((current) => ({
                      ...current,
                      settings: {
                        ...current.settings,
                        general: { ...current.settings.general, theme_mode: value },
                      },
                    }))
                  }
                  className="control-panel-shell__choice-group--wide"
                  options={THEME_MODE_OPTIONS}
                />
              </ControlLine>
            </SettingsCard>

            <SettingsCard title="系统行为" description="影响应用启动方式和通知表现。">
              <ToggleLine
                label="开机自启"
                description="仅影响下次系统启动时是否自动运行。"
                checked={draft.settings.general.auto_launch}
                onCheckedChange={(checked) =>
                  updateSettings((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      general: { ...current.settings.general, auto_launch: checked },
                    },
                  }))
                }
              />

              <ToggleLine
                label="语音通知"
                description="控制应用内语音提示和音效反馈。"
                checked={draft.settings.general.voice_notification_enabled}
                onCheckedChange={(checked) =>
                  updateSettings((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      general: { ...current.settings.general, voice_notification_enabled: checked },
                    },
                  }))
                }
              />

              <ControlLine
                label="提示声线"
                hint="控制正式 `general.voice_type`，保存后重新打开控制面板会回显当前值。"
                disabled={!draft.settings.general.voice_notification_enabled}
              >
                <TextField.Root
                  className="control-panel-shell__input"
                  disabled={!draft.settings.general.voice_notification_enabled}
                  value={draft.settings.general.voice_type}
                  onChange={(event) =>
                    updateSettings((current) => ({
                      ...current,
                      settings: {
                        ...current.settings,
                        general: { ...current.settings.general, voice_type: event.target.value },
                      },
                    }))
                  }
                />
              </ControlLine>
            </SettingsCard>

            <SettingsCard title="工作区与下载" description="当前目录以后端当前运行时为准，仅影响本地打开范围与后续文件默认落盘语义。">
              <ControlLine
                label="当前工作区目录"
                hint="这里展示桌面端当前实际生效的工作区目录；待重启的 settings 草稿不会改变本地打开范围。"
                className="control-panel-shell__row--stacked"
              >
                <div className="control-panel-shell__path-stack">
                  <code className="control-panel-shell__path-value">{runtimeWorkspacePathLabel}</code>
                  <div className="control-panel-shell__path-actions">
                    <Button
                      type="button"
                      variant="soft"
                      color="gray"
                      className="control-panel-shell__button control-panel-shell__button--ghost"
                      onClick={() => void handleOpenCurrentWorkspaceDirectory()}
                      disabled={!canOpenRuntimeWorkspace}
                    >
                      打开当前目录
                    </Button>
                  </div>
                  {workspaceActionFeedback ? (
                    <Text as="p" size="2" className="control-panel-shell__action-feedback control-panel-shell__path-feedback" aria-live="polite">
                      {workspaceActionFeedback}
                    </Text>
                  ) : null}
                </div>
              </ControlLine>

              <ToggleLine
                label="下载前逐个确认保存位置"
                description="开启后，每次下载都会先确认目标保存路径。"
                checked={draft.settings.general.download.ask_before_save_each_file}
                onCheckedChange={(checked) =>
                  updateSettings((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      general: {
                        ...current.settings.general,
                        download: {
                          ...current.settings.general.download,
                          ask_before_save_each_file: checked,
                        },
                      },
                    },
                  }))
                }
              />
            </SettingsCard>
          </>
        );

      case "desktop":
        return (
          <>
            <SettingsCard title="悬浮球状态" description="控制悬浮球在桌面上的默认表现。">
              <ToggleLine
                label="自动贴边"
                description="停止拖拽后自动贴边，减少桌面遮挡。"
                checked={draft.settings.floating_ball.auto_snap}
                onCheckedChange={(checked) =>
                  updateSettings((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      floating_ball: { ...current.settings.floating_ball, auto_snap: checked },
                    },
                  }))
                }
              />

              <ToggleLine
                label="空闲半透明"
                description="在无操作时降低存在感，减少桌面遮挡。"
                checked={draft.settings.floating_ball.idle_translucent}
                onCheckedChange={(checked) =>
                  updateSettings((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      floating_ball: { ...current.settings.floating_ball, idle_translucent: checked },
                    },
                  }))
                }
              />
            </SettingsCard>

            <SettingsCard title="在场方式" description="调整悬浮球的尺寸与停靠模式。">
              <ControlLine label="尺寸" hint="在多窗口协作时决定悬浮球的可发现程度。">
                <div className="control-panel-shell__slider-stack">
                  <Slider
                    min={0}
                    max={2}
                    step={1}
                    value={[getFloatingBallSizeSliderValue(draft.settings.floating_ball.size)]}
                    onValueChange={(values) =>
                      updateSettings((current) => ({
                        ...current,
                        settings: {
                          ...current.settings,
                          floating_ball: {
                            ...current.settings.floating_ball,
                            size: getFloatingBallSizeFromSliderValue(values[0]),
                          },
                        },
                      }))
                    }
                  />
                  <div className="control-panel-shell__slider-legend">
                    <span>小</span>
                    <span>中</span>
                    <span>大</span>
                  </div>
                </div>
              </ControlLine>

              <ControlLine label="停靠方式" hint="固定更稳定，可拖动更适合多屏与复杂工作区。">
                <ChoiceGroup
                  value={draft.settings.floating_ball.position_mode}
                  onValueChange={(value) =>
                    updateSettings((current) => ({
                      ...current,
                      settings: {
                        ...current.settings,
                        floating_ball: {
                          ...current.settings.floating_ball,
                          position_mode: value,
                        },
                      },
                    }))
                  }
                  className="control-panel-shell__choice-group--wide"
                  options={POSITION_MODE_OPTIONS}
                />
              </ControlLine>
            </SettingsCard>
          </>
        );

      case "memory":
        return (
          <>
            <SettingsCard title="镜子记忆" description="控制长期记忆是否开启以及保留方式。">
              <ToggleLine
                label="启用记忆"
                description="关闭后不再记录新的长期记忆。"
                checked={draft.settings.memory.enabled}
                onCheckedChange={(checked) =>
                  updateSettings((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      memory: { ...current.settings.memory, enabled: checked },
                    },
                  }))
                }
              />

              <ControlLine label="生命周期" hint="控制镜子记忆默认保留周期。">
                <Select.Root
                  value={draft.settings.memory.lifecycle}
                  onValueChange={(value) =>
                    updateSettings((current) => ({
                      ...current,
                      settings: {
                        ...current.settings,
                        memory: { ...current.settings.memory, lifecycle: value },
                      },
                    }))
                  }
                >
                  <Select.Trigger className="control-panel-shell__select-trigger" radius="full" />
                  <Select.Content className="control-panel-shell__select-content" position="popper">
                    {MEMORY_LIFECYCLE_OPTIONS.map((option) => (
                      <Select.Item key={option.value} value={option.value}>
                        {option.label}
                      </Select.Item>
                    ))}
                  </Select.Content>
                </Select.Root>
              </ControlLine>
            </SettingsCard>

            <SettingsCard title="记忆节奏" description="控制工作总结与画像刷新的默认频率。">
              <ControlLine label="工作总结间隔" hint="控制自动工作总结的生成频率。">
                <TimeIntervalInput
                  interval={draft.settings.memory.work_summary_interval}
                  onValueChange={(value) =>
                    updateSettings((current) => ({
                      ...current,
                      settings: {
                        ...current.settings,
                        memory: {
                          ...current.settings.memory,
                          work_summary_interval: {
                            ...current.settings.memory.work_summary_interval,
                            value,
                          },
                        },
                      },
                    }))
                  }
                  onUnitChange={(unit) =>
                    updateSettings((current) => ({
                      ...current,
                      settings: {
                        ...current.settings,
                        memory: {
                          ...current.settings.memory,
                          work_summary_interval: {
                            ...current.settings.memory.work_summary_interval,
                            unit: unit as (typeof current.settings.memory.work_summary_interval)["unit"],
                          },
                        },
                      },
                    }))
                  }
                />
              </ControlLine>

              <ControlLine label="画像刷新间隔" hint="控制偏好画像的刷新频率。">
                <TimeIntervalInput
                  interval={draft.settings.memory.profile_refresh_interval}
                  onValueChange={(value) =>
                    updateSettings((current) => ({
                      ...current,
                      settings: {
                        ...current.settings,
                        memory: {
                          ...current.settings.memory,
                          profile_refresh_interval: {
                            ...current.settings.memory.profile_refresh_interval,
                            value,
                          },
                        },
                      },
                    }))
                  }
                  onUnitChange={(unit) =>
                    updateSettings((current) => ({
                      ...current,
                      settings: {
                        ...current.settings,
                        memory: {
                          ...current.settings.memory,
                          profile_refresh_interval: {
                            ...current.settings.memory.profile_refresh_interval,
                            unit: unit as (typeof current.settings.memory.profile_refresh_interval)["unit"],
                          },
                        },
                      },
                    }))
                  }
                />
              </ControlLine>
            </SettingsCard>
          </>
        );

      case "automation":
        return (
          <>
            <SettingsCard title="巡检规则" description="控制任务巡检的启动方式与提醒节奏。">
              <ControlLine label="巡检频率" hint="控制系统定时扫描待办来源的时间间隔。">
                <Select.Root
                  value={buildInspectionIntervalOptionValue(draft.inspector.inspection_interval)}
                  onValueChange={(value) =>
                    updateSettings((current) => ({
                      ...current,
                      inspector: {
                        ...current.inspector,
                        inspection_interval: parseInspectionIntervalOptionValue(value),
                      },
                    }))
                  }
                >
                  <Select.Trigger className="control-panel-shell__select-trigger" radius="full" />
                  <Select.Content className="control-panel-shell__select-content" position="popper">
                    {INSPECTION_INTERVAL_OPTIONS.map((option) => (
                      <Select.Item
                        key={buildInspectionIntervalOptionValue(option)}
                        value={buildInspectionIntervalOptionValue(option)}
                      >
                        {option.label}
                      </Select.Item>
                    ))}
                  </Select.Content>
                </Select.Root>
              </ControlLine>

              <ToggleLine
                label="开机巡检"
                description="应用启动后自动运行一次任务巡检。"
                checked={draft.inspector.inspect_on_startup}
                onCheckedChange={(checked) =>
                  updateSettings((current) => ({
                    ...current,
                    inspector: { ...current.inspector, inspect_on_startup: checked },
                  }))
                }
              />

              <ToggleLine
                label="文件变化时巡检"
                description="监听任务文件变化并刷新巡检结果。"
                checked={draft.inspector.inspect_on_file_change}
                onCheckedChange={(checked) =>
                  updateSettings((current) => ({
                    ...current,
                    inspector: { ...current.inspector, inspect_on_file_change: checked },
                  }))
                }
              />

              <ToggleLine
                label="截止前提醒"
                description="在任务接近截止前推送预警。"
                checked={draft.inspector.remind_before_deadline}
                onCheckedChange={(checked) =>
                  updateSettings((current) => ({
                    ...current,
                    inspector: { ...current.inspector, remind_before_deadline: checked },
                  }))
                }
              />

              <ToggleLine
                label="陈旧任务提醒"
                description="对长时间未推进的任务发出提醒。"
                checked={draft.inspector.remind_when_stale}
                onCheckedChange={(checked) =>
                  updateSettings((current) => ({
                    ...current,
                    inspector: { ...current.inspector, remind_when_stale: checked },
                  }))
                }
              />
            </SettingsCard>

            <SettingsCard title="任务来源" description="每行填写一个路径或标签作为巡检来源。">
              <InfoRow label="已配置来源" value={`${draft.inspector.task_sources.length} 项`} />

              <ControlLine label="任务来源列表" hint="支持多个工作区路径或任务标签。" className="control-panel-shell__row--stacked">
                <TextArea
                  className="control-panel-shell__textarea"
                  value={draft.inspector.task_sources.join("\n")}
                  onChange={(event) =>
                    updateSettings((current) => {
                      const taskSources = event.target.value
                        .split(/\r?\n/)
                        .map((item) => item.trim())
                        .filter(Boolean);

                      return {
                        ...current,
                        inspector: { ...current.inspector, task_sources: taskSources },
                      };
                    })
                  }
                />
              </ControlLine>
            </SettingsCard>
          </>
        );

      case "plugins":
        return (
          <>
            <SettingsCard title="插件运行态总览" description="列表、详情与运行态来自正式 `agent.plugin.*` 查询；启停按钮当前只写入控制面板本地 mock 状态。">
              <InfoRow label="已注册插件" value={pluginSnapshot ? `${pluginSnapshot.summary.total} 个` : "载入中…"} />
              <InfoRow label="健康插件" value={pluginSnapshot ? `${pluginSnapshot.summary.healthy} 个` : "载入中…"} />
              <InfoRow label="失败 / 降级" value={pluginSnapshot ? `${pluginSnapshot.summary.failed + pluginSnapshot.summary.degraded} 个` : "载入中…"} />
              <InfoRow label="本地 mock 覆盖" value={pluginSnapshot ? `${pluginSnapshot.summary.mock_overrides} 个` : "载入中…"} />
              <ControlLine
                label="刷新运行态"
                hint="重新读取插件列表、运行态、指标和最近事件；不会清空本地 mock 启停覆盖。"
                className="control-panel-shell__row--stacked"
              >
                <div className="control-panel-shell__plugin-toolbar">
                  <Button
                    type="button"
                    variant="soft"
                    color="gray"
                    className="control-panel-shell__button control-panel-shell__button--ghost"
                    onClick={() => void handlePluginRefresh()}
                    disabled={isLoadingPlugins}
                  >
                    <RefreshCw size={15} strokeWidth={1.8} />
                    <span>{isLoadingPlugins ? "刷新中…" : "刷新插件状态"}</span>
                  </Button>
                  <Text as="p" size="1" className="control-panel-shell__field-note">
                    Mock 启停只影响当前控制面板的演示态，不会向后端提交正式 enable / disable。
                  </Text>
                </div>
              </ControlLine>
              {pluginLoadError ? (
                <Text as="p" size="2" color="amber" className="control-panel-shell__action-feedback">
                  {pluginLoadError}
                </Text>
              ) : null}
            </SettingsCard>

            <SettingsCard title="已注册插件" description="每张卡片同时展示正式运行态和控制面板本地 mock 启停态。">
              <div className="control-panel-shell__plugin-grid">
                {pluginSnapshot && pluginSnapshot.items.length > 0 ? (
                  pluginSnapshot.items.map((item) => {
                    const selected = item.plugin.plugin_id === selectedPluginId;
                    const pluginLabel = item.plugin.display_name || item.plugin.name;
                    return (
                      <article
                        key={item.plugin.plugin_id}
                        className="control-panel-shell__plugin-card"
                        data-active={selected ? "true" : "false"}
                      >
                        <button
                          type="button"
                          className="control-panel-shell__plugin-card-button"
                          onClick={() => setSelectedPluginId(item.plugin.plugin_id)}
                        >
                          <div className="control-panel-shell__plugin-card-heading">
                            <div>
                              <Text as="p" size="2" weight="medium" className="control-panel-shell__plugin-card-title">
                                {pluginLabel}
                              </Text>
                              <Text as="p" size="1" className="control-panel-shell__plugin-card-subtitle">
                                {item.plugin.plugin_id}
                              </Text>
                            </div>
                            <StatusPill tone={getPluginHealthTone(item.runtime_health)}>{formatPluginHealthLabel(item.runtime_health)}</StatusPill>
                          </div>
                          <Text as="p" size="2" className="control-panel-shell__plugin-card-copy">
                            {item.plugin.summary}
                          </Text>
                          <div className="control-panel-shell__plugin-pill-row">
                            <StatusPill tone={item.control.source === "mock" ? "mock" : "synced"}>{formatPluginControlLabel(item)}</StatusPill>
                            <span className="control-panel-shell__plugin-meta-pill">{formatPluginSourceLabel(item.plugin.source)}</span>
                            <span className="control-panel-shell__plugin-meta-pill">v{item.plugin.version}</span>
                          </div>
                          <div className="control-panel-shell__plugin-stat-grid">
                            <span>{item.runtime_count} runtimes</span>
                            <span>{item.capability_count} tools</span>
                            <span>{item.permission_count} permissions</span>
                          </div>
                        </button>
                        <div className="control-panel-shell__plugin-card-actions">
                          <Button
                            type="button"
                            variant={item.control.effective_enabled ? "soft" : undefined}
                            color="gray"
                            className="control-panel-shell__button control-panel-shell__button--ghost"
                            onClick={() => handlePluginToggle(item, !item.control.effective_enabled)}
                          >
                            <Power size={15} strokeWidth={1.8} />
                            <span>{item.control.effective_enabled ? "Mock Stop" : "Mock Start"}</span>
                          </Button>
                        </div>
                      </article>
                    );
                  })
                ) : (
                  <div className="control-panel-shell__plugin-empty">
                    <Text as="p" size="2" className="control-panel-shell__plugin-card-copy">
                      {isLoadingPlugins ? "正在读取插件扩展…" : "当前还没有可展示的已注册插件。"}
                    </Text>
                  </div>
                )}
              </div>
            </SettingsCard>

            <SettingsCard title="插件详情" description="详情面板展示正式 manifest、runtime、metrics、recent events 与 tools 合同。">
              {selectedPluginSummary ? (
                <div className="control-panel-shell__plugin-detail-stack">
                  <div className="control-panel-shell__plugin-detail-header">
                    <div>
                      <Heading as="h3" size="4" className="control-panel-shell__card-title">
                        {selectedPluginSummary.plugin.display_name || selectedPluginSummary.plugin.name}
                      </Heading>
                      <Text as="p" size="1" className="control-panel-shell__plugin-card-subtitle">
                        {selectedPluginSummary.plugin.plugin_id}
                      </Text>
                    </div>
                    <div className="control-panel-shell__plugin-pill-row">
                      <StatusPill tone={getPluginHealthTone(selectedPluginSummary.runtime_health)}>
                        {formatPluginHealthLabel(selectedPluginSummary.runtime_health)}
                      </StatusPill>
                      <StatusPill tone={selectedPluginSummary.control.source === "mock" ? "mock" : "synced"}>
                        {formatPluginControlLabel(selectedPluginSummary)}
                      </StatusPill>
                    </div>
                  </div>

                  <Text as="p" size="2" className="control-panel-shell__plugin-card-copy">
                    {selectedPluginSummary.plugin.summary}
                  </Text>

                  {isLoadingPluginDetail && !pluginDetail ? (
                    <Text as="p" size="2" className="control-panel-shell__plugin-card-copy">
                      正在加载插件详情…
                    </Text>
                  ) : pluginDetail ? (
                    <>
                      <div className="control-panel-shell__plugin-detail-grid">
                        <InfoRow label="入口" value={<code className="control-panel-shell__about-link">{pluginDetail.plugin.entry}</code>} />
                        <InfoRow label="来源" value={formatPluginSourceLabel(pluginDetail.plugin.source)} />
                        <InfoRow label="正式 enabled" value={pluginDetail.plugin.enabled ? "enabled" : "disabled"} />
                        <InfoRow label="本地控制态" value={formatPluginControlLabel(pluginDetail)} />
                      </div>

                      <section className="control-panel-shell__plugin-block">
                        <Text as="p" size="2" weight="medium" className="control-panel-shell__plugin-block-title">
                          权限与能力
                        </Text>
                        <div className="control-panel-shell__plugin-pill-row">
                          {pluginDetail.plugin.permissions.map((permission) => (
                            <span key={permission} className="control-panel-shell__plugin-meta-pill">
                              {permission}
                            </span>
                          ))}
                          {pluginDetail.plugin.permissions.length === 0 ? (
                            <span className="control-panel-shell__plugin-meta-pill">No declared permissions</span>
                          ) : null}
                        </div>
                      </section>

                      <section className="control-panel-shell__plugin-block">
                        <Text as="p" size="2" weight="medium" className="control-panel-shell__plugin-block-title">
                          运行态
                        </Text>
                        <div className="control-panel-shell__plugin-runtime-list">
                          {pluginDetail.runtimes.map((runtime) => (
                            <article key={`${runtime.kind}:${runtime.name}`} className="control-panel-shell__plugin-runtime-card">
                              <div className="control-panel-shell__plugin-card-heading">
                                <div>
                                  <Text as="p" size="2" weight="medium" className="control-panel-shell__plugin-card-title">
                                    {runtime.name}
                                  </Text>
                                  <Text as="p" size="1" className="control-panel-shell__plugin-card-subtitle">
                                    {formatPluginRuntimeKindLabel(runtime.kind)} · {runtime.transport}
                                  </Text>
                                </div>
                                <StatusPill tone={getPluginHealthTone(runtime.health)}>{formatPluginHealthLabel(runtime.health)}</StatusPill>
                              </div>
                              <Text as="p" size="1" className="control-panel-shell__plugin-card-copy">
                                status: {runtime.status} · last seen: {formatPluginTimestampLabel(runtime.last_seen_at)}
                              </Text>
                            </article>
                          ))}
                          {pluginDetail.runtimes.length === 0 ? (
                            <Text as="p" size="2" className="control-panel-shell__plugin-card-copy">
                              当前插件没有返回运行态实例。
                            </Text>
                          ) : null}
                        </div>
                      </section>

                      <section className="control-panel-shell__plugin-block">
                        <Text as="p" size="2" weight="medium" className="control-panel-shell__plugin-block-title">
                          Tools 合同
                        </Text>
                        <div className="control-panel-shell__plugin-tool-list">
                          {pluginDetail.tools.map((tool) => (
                            <article key={tool.tool_name} className="control-panel-shell__plugin-tool-card">
                              <div className="control-panel-shell__plugin-card-heading">
                                <div>
                                  <Text as="p" size="2" weight="medium" className="control-panel-shell__plugin-card-title">
                                    {tool.display_name}
                                  </Text>
                                  <Text as="p" size="1" className="control-panel-shell__plugin-card-subtitle">
                                    {tool.tool_name}
                                  </Text>
                                </div>
                                <StatusPill tone={tool.risk_hint === "red" ? "danger" : tool.risk_hint === "yellow" ? "warning" : "live"}>
                                  {formatPluginRiskHintLabel(tool.risk_hint)}
                                </StatusPill>
                              </div>
                              <Text as="p" size="2" className="control-panel-shell__plugin-card-copy">
                                {tool.description}
                              </Text>
                              <div className="control-panel-shell__plugin-stat-grid">
                                <span>{tool.input_contract.fields.length} input fields</span>
                                <span>{tool.output_contract.fields.length} output fields</span>
                                <span>{tool.supports_dry_run ? "dry-run" : "direct run"}</span>
                              </div>
                            </article>
                          ))}
                          {pluginDetail.tools.length === 0 ? (
                            <Text as="p" size="2" className="control-panel-shell__plugin-card-copy">
                              当前插件没有声明工具合同。
                            </Text>
                          ) : null}
                        </div>
                      </section>

                      <section className="control-panel-shell__plugin-block">
                        <Text as="p" size="2" weight="medium" className="control-panel-shell__plugin-block-title">
                          最近事件
                        </Text>
                        <div className="control-panel-shell__plugin-event-list">
                          {pluginDetail.recent_events.slice(0, 5).map((event, index) => (
                            <article key={`${event.event_type}:${event.created_at}:${index}`} className="control-panel-shell__plugin-event-card">
                              <Text as="p" size="2" weight="medium" className="control-panel-shell__plugin-card-title">
                                {event.event_type}
                              </Text>
                              <Text as="p" size="1" className="control-panel-shell__plugin-card-copy">
                                {formatPluginTimestampLabel(event.created_at)}
                              </Text>
                            </article>
                          ))}
                          {pluginDetail.recent_events.length === 0 ? (
                            <Text as="p" size="2" className="control-panel-shell__plugin-card-copy">
                              当前插件没有可展示的最近事件。
                            </Text>
                          ) : null}
                        </div>
                      </section>
                    </>
                  ) : (
                    <Text as="p" size="2" className="control-panel-shell__plugin-card-copy">
                      选择左侧插件卡片后，可在这里查看运行态、权限、工具合同与最近事件。
                    </Text>
                  )}
                </div>
              ) : (
                <Text as="p" size="2" className="control-panel-shell__plugin-card-copy">
                  当前没有已选插件。
                </Text>
              )}
            </SettingsCard>
          </>
        );

      case "models":
        return (
          <>
            <div>
              <SettingsCard title="模型路由" description="配置 provider、接口地址和默认模型。">
              <ControlLine label="Provider" hint="当前任务默认使用的模型提供商。">
                <TextField.Root
                  className="control-panel-shell__input"
                  value={draft.settings.models.provider}
                  onChange={(event) =>
                    updateSettings((current) => ({
                      ...current,
                      settings: {
                        ...current.settings,
                        models: { ...current.settings.models, provider: event.target.value },
                      },
                    }))
                  }
                />
              </ControlLine>

              <ControlLine label="Base URL" hint="用于接入托管服务或兼容接口。">
                <TextField.Root
                  className="control-panel-shell__input"
                  value={draft.settings.models.base_url}
                  onChange={(event) =>
                    updateSettings((current) => ({
                      ...current,
                      settings: {
                        ...current.settings,
                        models: { ...current.settings.models, base_url: event.target.value },
                      },
                    }))
                  }
                />
              </ControlLine>

              <ControlLine label="Model" hint="主链路默认优先选择的模型名。">
                <TextField.Root
                  className="control-panel-shell__input"
                  value={draft.settings.models.model}
                  onChange={(event) =>
                    updateSettings((current) => ({
                      ...current,
                      settings: {
                        ...current.settings,
                        models: { ...current.settings.models, model: event.target.value },
                      },
                    }))
                  }
                />
              </ControlLine>

              <ControlLine label="API Key" hint={providerApiKeyHint} className="control-panel-shell__row--stacked">
                <TextField.Root
                  className="control-panel-shell__input"
                  type="password"
                  value={draft.providerApiKeyInput}
                  placeholder={draft.settings.models.provider_api_key_configured ? "已配置，如需更换请重新输入" : "输入新的 provider API key"}
                  autoComplete="off"
                  onChange={(event) =>
                    updateSettings((current) => ({
                      ...current,
                      providerApiKeyInput: event.target.value,
                    }))
                  }
                />
              </ControlLine>

              <ToggleLine
                label="预算自动降级"
                description="预算接近上限时自动降级模型或交付强度。"
                checked={draft.settings.models.budget_auto_downgrade}
                onCheckedChange={(checked) =>
                  updateSettings((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      models: { ...current.settings.models, budget_auto_downgrade: checked },
                    },
                  }))
                }
              />
              <div className="control-panel-shell__model-actions">
                <Button
                  className="control-panel-shell__button control-panel-shell__button--ghost"
                  variant="soft"
                  color="gray"
                  onClick={() => void handleValidateModel()}
                  disabled={isSaving || isRunningInspection || isValidatingModel}
                >
                  {isValidatingModel ? "校验中…" : "测试连接"}
                </Button>
                {modelValidationFeedback ? (
                  <Text
                    as="p"
                    size="2"
                    color={modelValidationFeedback.tone === "warning" ? "amber" : undefined}
                    className="control-panel-shell__action-feedback control-panel-shell__model-feedback"
                    aria-live="polite"
                  >
                    {modelValidationFeedback.message}
                  </Text>
                ) : null}
              </div>
              </SettingsCard>
            </div>

            <SettingsCard title="模型与安全摘要" description="查看当前模型路由、API Key 状态与安全摘要。">
              <InfoRow label="当前模型" value={draft.settings.models.model} />
              <InfoRow label="API Key 状态" value={providerApiKeyStatus} />
              <InfoRow label="安全状态" value={hasRpcLoadError ? "暂不可用" : draft.securitySummary.security_status} />
              <InfoRow label="待确认授权" value={hasRpcLoadError ? "暂不可用" : draft.securitySummary.pending_authorizations} />
            </SettingsCard>

            <SettingsCard title="预算与成本展示" description="用于设置安全卫士中的预算阈值和当日成本展示。">
              <ControlLine label="当日 Tokens" hint="用于补充安全卫士中的当日 Token 展示。">
                <TextField.Root
                  className="control-panel-shell__input"
                  type="number"
                  min="0"
                  step="1"
                  value={formatBudgetDisplayInputValue(budgetDisplayDraft.today_tokens)}
                  placeholder="留空则沿用安全卫士当前值"
                  onChange={(event) => updateBudgetDisplay("today_tokens", event.target.value)}
                />
              </ControlLine>

              <ControlLine label="今日成本" hint="用于补充安全卫士中的当日成本展示。">
                <TextField.Root
                  className="control-panel-shell__input"
                  type="number"
                  min="0"
                  step="0.01"
                  value={formatBudgetDisplayInputValue(budgetDisplayDraft.today_cost)}
                  placeholder="留空则沿用安全卫士当前值"
                  onChange={(event) => updateBudgetDisplay("today_cost", event.target.value, true)}
                />
              </ControlLine>

              <ControlLine label="单任务上限" hint="用于设置安全卫士中的单任务预算阈值。">
                <TextField.Root
                  className="control-panel-shell__input"
                  type="number"
                  min="0"
                  step="1"
                  value={formatBudgetDisplayInputValue(budgetDisplayDraft.single_task_limit)}
                  placeholder="留空则显示为未配置"
                  onChange={(event) => updateBudgetDisplay("single_task_limit", event.target.value)}
                />
              </ControlLine>

              <ControlLine label="当日上限" hint="用于设置安全卫士中的当日预算阈值。">
                <TextField.Root
                  className="control-panel-shell__input"
                  type="number"
                  min="0"
                  step="1"
                  value={formatBudgetDisplayInputValue(budgetDisplayDraft.daily_limit)}
                  placeholder="留空则显示为未配置"
                  onChange={(event) => updateBudgetDisplay("daily_limit", event.target.value)}
                />
              </ControlLine>
            </SettingsCard>
          </>
        );

      case "about":
        return (
          <>
            <SettingsCard title="本地存储位置" description="这里展示桌面端当前用户目录下的正式 data 存储位置。">
              <InfoRow label="数据目录" value={<code className="control-panel-shell__about-link">{localDataPathLabel}</code>} />

              <ControlLine label="定位操作" hint="优先在系统资源管理器中打开 data 目录；目录不存在时会由宿主按需创建。" className="control-panel-shell__row--stacked">
                <div className="control-panel-shell__about-actions">
                  <Button
                    type="button"
                    variant="soft"
                    className="control-panel-shell__button control-panel-shell__button--secondary control-panel-shell__about-button"
                    onClick={() => void handleAboutAction("open_data_directory")}
                    disabled={localDataPath.length === 0}
                  >
                    打开目录
                  </Button>
                </div>
              </ControlLine>
            </SettingsCard>

            <SettingsCard title="帮助与反馈" description="集中展示应用内帮助入口与可扩展的反馈渠道。">
              <InfoRow label="帮助入口" value="应用内新手引导" />

              <ControlLine
                label="反馈渠道"
                hint="支持放置链接、二维码图片和预留位；后续只需要改 about 配置，不需要改 JSX 结构。"
                className="control-panel-shell__row--stacked"
              >
                <div className="control-panel-shell__feedback-grid">
                  {CONTROL_PANEL_ABOUT_FEEDBACK_CHANNELS.map((channel) => (
                    <FeedbackChannelCard key={channel.id} channel={channel} onCopyLink={handleAboutLinkCopy} />
                  ))}
                </div>
              </ControlLine>
            </SettingsCard>

            <SettingsCard title="分享 CialloClaw" description="复制项目地址，方便转发给协作者或朋友。">
              <InfoRow label="分享链接" value={<code className="control-panel-shell__about-link">https://github.com/1024XEngineer/CialloClaw</code>} />

              <ControlLine label="分享操作" hint="优先复制仓库地址；若当前环境不支持剪贴板，会直接显示链接。" className="control-panel-shell__row--stacked">
                <div className="control-panel-shell__about-actions">
                  <Button
                    type="button"
                    variant="soft"
                    className="control-panel-shell__button control-panel-shell__button--secondary control-panel-shell__about-button"
                    onClick={() => void handleAboutAction("share")}
                  >
                    复制链接
                  </Button>
                </div>
              </ControlLine>
            </SettingsCard>

            <SettingsCard title="版本信息" description="查看当前桌面端版本号。">
              <InfoRow label="产品名称" value={aboutSnapshot.appName} />
              <InfoRow label="应用版本" value={aboutSnapshot.appVersion} />
            </SettingsCard>

            <SettingsCard title="恢复默认设置" description="将桌面端可重置的设置恢复到默认值。">
              {isRestoreDefaultsConfirming ? (
                <div className="control-panel-shell__about-confirm">
                  <Text as="p" size="2" className="control-panel-shell__about-note">
                    会重置通用设置、悬浮球、记忆设置、任务巡检、预算自动降级，以及本地预算展示值。
                  </Text>
                  <Text as="p" size="2" className="control-panel-shell__about-note">
                    不会删除任务历史、记忆内容、本地文件，也不会改动当前已保存的 workspace 路径、任务来源、模型路由或已保存 API Key。
                  </Text>
                  <Text as="p" size="2" className="control-panel-shell__about-note">
                    确认后会立即提交默认设置；若存在需要延后生效的设置，仍按后端当前 `apply_mode` 规则生效。
                  </Text>
                  <div className="control-panel-shell__about-actions">
                    <Button
                      type="button"
                      className="control-panel-shell__button control-panel-shell__button--primary control-panel-shell__about-button"
                      onClick={() => void handleRestoreDefaults()}
                      disabled={restoreDefaultsDisabled}
                    >
                      确认恢复默认设置
                    </Button>
                    <Button
                      type="button"
                      variant="soft"
                      className="control-panel-shell__button control-panel-shell__button--ghost control-panel-shell__about-button"
                      onClick={handleCancelRestoreDefaults}
                      disabled={restoreDefaultsDisabled}
                    >
                      取消
                    </Button>
                  </div>
                </div>
              ) : (
                <ControlLine label="恢复操作" hint="先进入确认步骤，再查看本次会恢复哪些设置。" className="control-panel-shell__row--stacked">
                  <div className="control-panel-shell__about-actions">
                    <Button
                      type="button"
                      variant="soft"
                      className="control-panel-shell__button control-panel-shell__button--secondary control-panel-shell__about-button"
                      onClick={handlePrepareRestoreDefaults}
                      disabled={restoreDefaultsDisabled}
                    >
                      恢复默认设置
                    </Button>
                  </div>
                </ControlLine>
              )}
            </SettingsCard>

          </>
        );

    }
  };

  return (
    <main className="app-shell control-panel-shell" data-appearance={controlPanelAppearance}>
      <div className="control-panel-shell__titlebar" aria-label="控制面板窗口操作" onPointerDown={handleTopbarPointerDown}>
        <div className="control-panel-shell__titlebar-copy">
          <Heading size="5" className="control-panel-shell__titlebar-title">
            控制面板
          </Heading>
        </div>

        <div className="control-panel-shell__titlebar-actions">
          <button
            type="button"
            className="control-panel-shell__window-button control-panel-shell__window-button--drag"
            aria-label="拖动控制面板窗口"
            onPointerDown={handleWindowDragPointerDown}
          >
            <GripHorizontal size={16} strokeWidth={1.85} />
            <span>拖动窗口</span>
          </button>
          <button
            type="button"
            className="control-panel-shell__window-button control-panel-shell__window-button--close"
            aria-label="关闭控制面板"
            onClick={handleWindowCloseClick}
            onPointerDown={handleWindowClosePointerDown}
          >
            <X size={16} strokeWidth={1.9} />
          </button>
        </div>
      </div>

      <div className="control-panel-shell__workspace">
        <aside className="control-panel-shell__sidebar" aria-label="控制面板分组导航">
          <div className="control-panel-shell__nav-groups">
            {NAVIGATION_GROUPS.map((group) => (
              <div key={group.label} className="control-panel-shell__nav-group">
                <Text as="p" size="1" className="control-panel-shell__nav-group-label">
                  {group.label}
                </Text>
                <div className="control-panel-shell__nav-list">
                  {group.items.map((itemId) => (
                    <SidebarItem
                      key={itemId}
                      active={activeSection === itemId}
                      item={SECTION_META[itemId]}
                      onSelect={() => setActiveSection(itemId)}
                    />
                  ))}
                </div>
              </div>
            ))}
          </div>
        </aside>

        <section className="control-panel-shell__content">
          <header className="control-panel-shell__hero">
            {hasRpcLoadError ? (
              <section className="control-panel-shell__error-banner" aria-live="polite">
                <div className="control-panel-shell__error-banner-copy">
                  <Text as="p" size="2" weight="medium" className="control-panel-shell__error-banner-title">
                    设置服务连接失败
                  </Text>
                  <Text as="p" size="2" className="control-panel-shell__error-banner-text">
                    {loadError}
                  </Text>
                  <Text as="p" size="1" className="control-panel-shell__error-banner-note">
                    当前仍展示上一次成功同步的本地快照，重新连接后请刷新页面以回显正式设置。
                  </Text>
                </div>

                <Button
                  className="control-panel-shell__button control-panel-shell__button--secondary"
                  variant="soft"
                  onClick={() => void handleReload()}
                >
                  重新加载
                </Button>
              </section>
            ) : null}

            <div className="control-panel-shell__hero-heading">
              <Text as="p" size="1" className="control-panel-shell__eyebrow">
                {activeMeta.group}
              </Text>
              <Heading size="8" className="control-panel-shell__hero-title">
                {activeMeta.title}
              </Heading>
            </div>
          </header>

          <div className="control-panel-shell__cards">{renderSectionContent()}</div>

          <div className="control-panel-shell__action-bar">
            <div className="control-panel-shell__action-statuses">
              {saveStateValue}
              {isReplayingOnboarding ? (
                <Text as="p" size="2" className="control-panel-shell__action-feedback" aria-live="polite">
                  正在打开引导...
                </Text>
              ) : null}
              {draft.warnings && draft.warnings.length > 0 ? (
                <Text as="p" size="2" color="amber" className="control-panel-shell__action-feedback" aria-live="polite">
                  {draft.warnings[0]}
                </Text>
              ) : null}
              {saveFeedback ? (
                <Text as="p" size="2" className="control-panel-shell__action-feedback" aria-live="polite">
                  {saveFeedback}
                </Text>
              ) : null}
              {inspectionSummary ? (
                <Text as="p" size="2" className="control-panel-shell__action-feedback" aria-live="polite">
                  {inspectionSummary}
                </Text>
              ) : null}
              {pluginFeedback ? (
                <Text as="p" size="2" className="control-panel-shell__action-feedback" aria-live="polite">
                  {pluginFeedback}
                </Text>
              ) : null}
              {aboutActionFeedback ? (
                <Text as="p" size="2" className="control-panel-shell__action-feedback" aria-live="polite">
                  {aboutActionFeedback}
                </Text>
              ) : null}
            </div>

            <div className="control-panel-shell__action-buttons">
              <Button
                className="control-panel-shell__button control-panel-shell__button--ghost"
                variant="soft"
                color="gray"
                onClick={handleReplayOnboarding}
                disabled={onboardingReplayDisabled}
              >
                {isReplayingOnboarding ? "正在打开引导…" : "重新查看新手引导"}
              </Button>

              <Button
                className="control-panel-shell__button control-panel-shell__button--ghost"
                variant="soft"
                color="gray"
                onClick={handleReset}
                disabled={!hasChanges || isSaving || isRunningInspection}
              >
                撤销修改
              </Button>

              <Button
                className="control-panel-shell__button control-panel-shell__button--primary"
                onClick={() => void handleSave()}
                disabled={!hasChanges || isSaving || isRunningInspection}
              >
                {isSaving ? "保存中…" : "保存设置"}
              </Button>
            </div>
          </div>
        </section>
      </div>
    </main>
  );
}
