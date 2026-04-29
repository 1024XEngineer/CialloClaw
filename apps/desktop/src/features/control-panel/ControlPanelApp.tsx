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
  ControlPanelSaveError,
  loadControlPanelData,
  runControlPanelInspection,
  saveControlPanelData,
  validateControlPanelModel,
  type ControlPanelData,
  type ControlPanelModelValidationOptions,
  type ControlPanelSaveResult,
} from "@/services/controlPanelService";
import { loadHydratedSettings } from "@/services/settingsService";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { buildDesktopOnboardingPresentation } from "@/features/onboarding/onboardingGeometry";
import {
  advanceDesktopOnboarding,
  startManualDesktopOnboardingReplay,
  setDesktopOnboardingPresentation,
} from "@/features/onboarding/onboardingService";
import { useDesktopOnboardingActions } from "@/features/onboarding/useDesktopOnboardingActions";
import { useDesktopOnboardingSession } from "@/features/onboarding/useDesktopOnboardingSession";
import { requestCurrentDesktopWindowClose, startCurrentDesktopWindowDragging } from "@/platform/desktopWindowFrame";
import { ensureOnboardingWindow } from "@/platform/onboardingWindowController";
import "./controlPanel.css";

type ControlPanelSectionId = "general" | "desktop" | "memory" | "automation" | "models" | "about";
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
  tone: "live" | "pending" | "synced";
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

type ChoiceOption<T extends string = string> = {
  label: string;
  value: T;
};

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
    items: ["models", "about"],
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

  return {
    settings,
    inspector: buildLocalInspectorFallback(settings),
    providerApiKeyInput: "",
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
  const [panelData, setPanelData] = useState<ControlPanelData | null>(null);
  const [draft, setDraft] = useState<ControlPanelData | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saveFeedback, setSaveFeedback] = useState<string | null>(null);
  const [modelValidationFeedback, setModelValidationFeedback] = useState<ModelValidationFeedback | null>(null);
  const [inspectionSummary, setInspectionSummary] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [isValidatingModel, setIsValidatingModel] = useState(false);
  const [isRunningInspection, setIsRunningInspection] = useState(false);
  const [isReplayingOnboarding, setIsReplayingOnboarding] = useState(false);
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

        if (cancelled) {
          return;
        }

        setLoadError(null);
        setPanelData(nextData);
        setDraft(nextData);
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
      setLoadError(null);
      setPanelData(nextData);
      setDraft(nextData);
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

  if (!draft || !panelData) {
    return (
      <main className="app-shell control-panel-shell" data-appearance={systemAppearance}>
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
  const hasChanges = inspectorDirty || settingsDirty;
  const providerApiKeyStatus = draft.settings.models.provider_api_key_configured ? "已配置" : "未配置";
  const resolvedAppearance = resolveControlPanelAppearance(draft.settings.general.theme_mode, systemAppearance);
  const providerApiKeyHint = "通过 JSON-RPC `agent.settings.update` 提交；只写入后端 Stronghold，不会回显明文。";
  const hasRpcLoadError = loadError !== null;
  const onboardingReplayDisabled = isSaving || isRunningInspection || isReplayingOnboarding;

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
    setSaveFeedback("已恢复为上次载入的设置快照。");
    setModelValidationFeedback(null);
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
      const result = await saveControlPanelData(draft, {
        confirmedInspector: panelData.inspector,
        saveInspector: inspectorDirty,
        saveSettings: settingsDirty,
        validateModel: modelSettingsDirty,
      });
      const nextPanelData = applyControlPanelSaveResult(panelData, result);
      const nextDraft = applyControlPanelSaveResult(draft, result);
      setLoadError(null);
      setPanelData(nextPanelData);
      setDraft(nextDraft);
      setSaveFeedback(getApplyModeCopy(result.applyMode, result.needRestart));
      if (result.modelValidation) {
        setModelValidationFeedback({
          message: result.modelValidation.message,
          tone: result.modelValidation.ok ? "neutral" : "warning",
        });
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

            <SettingsCard title="工作区与下载" description="变更后仅影响后续新生成的文件位置。">
              <ControlLine label="工作区路径" hint="任务文档与生成文件默认写入的本地位置。">
                <TextField.Root
                  className="control-panel-shell__input"
                  value={draft.settings.general.download.workspace_path}
                  onChange={(event) =>
                    updateSettings((current) => ({
                      ...current,
                      settings: {
                        ...current.settings,
                        general: {
                          ...current.settings.general,
                          download: { ...current.settings.general.download, workspace_path: event.target.value },
                        },
                      },
                    }))
                  }
                />
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

            <SettingsCard title="安全与预算摘要" description="查看当前安全状态、授权数量与预算限制。">
              <InfoRow label="当前模型" value={draft.settings.models.model} />
              <InfoRow label="API Key 状态" value={providerApiKeyStatus} />
              <InfoRow label="安全状态" value={hasRpcLoadError ? "暂不可用" : draft.securitySummary.security_status} />
              <InfoRow label="待确认授权" value={hasRpcLoadError ? "暂不可用" : draft.securitySummary.pending_authorizations} />
              <InfoRow
                label="今日成本"
                value={hasRpcLoadError ? "暂不可用" : `¥${draft.securitySummary.token_cost_summary.today_cost.toFixed(2)}`}
              />
              <InfoRow
                label="单任务上限"
                value={
                  hasRpcLoadError
                    ? "暂不可用"
                    : `${draft.securitySummary.token_cost_summary.single_task_limit.toLocaleString("zh-CN")} tokens`
                }
              />
              <InfoRow
                label="当日上限"
                value={
                  hasRpcLoadError
                    ? "暂不可用"
                    : `${draft.securitySummary.token_cost_summary.daily_limit.toLocaleString("zh-CN")} tokens`
                }
              />
            </SettingsCard>
          </>
        );

      case "about":
        return (
          <>
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
                  <Button type="button" variant="soft" className="control-panel-shell__about-button" onClick={() => void handleAboutAction("share")}>
                    复制链接
                  </Button>
                </div>
              </ControlLine>
            </SettingsCard>

            <SettingsCard title="版本信息" description="查看当前桌面端版本号。">
              <InfoRow label="产品名称" value={aboutSnapshot.appName} />
              <InfoRow label="应用版本" value={aboutSnapshot.appVersion} />
            </SettingsCard>

          </>
        );

    }
  };

  return (
    <main className="app-shell control-panel-shell" data-appearance={resolvedAppearance}>
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
