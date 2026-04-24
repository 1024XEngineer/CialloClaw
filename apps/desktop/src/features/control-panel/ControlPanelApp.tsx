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
  ControlPanelSaveError,
  loadControlPanelData,
  runControlPanelInspection,
  saveControlPanelData,
  type ControlPanelData,
  type ControlPanelSaveResult,
} from "@/services/controlPanelService";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { buildDesktopOnboardingPresentation } from "@/features/onboarding/onboardingGeometry";
import {
  advanceDesktopOnboarding,
  setDesktopOnboardingPresentation,
  startDesktopOnboarding,
} from "@/features/onboarding/onboardingService";
import { useDesktopOnboardingActions } from "@/features/onboarding/useDesktopOnboardingActions";
import { useDesktopOnboardingSession } from "@/features/onboarding/useDesktopOnboardingSession";
import { requestCurrentDesktopWindowClose, startCurrentDesktopWindowDragging } from "@/platform/desktopWindowFrame";
import { showShellBallWindow } from "@/platform/shellBallWindowController";
import "./controlPanel.css";

type ControlPanelSectionId = "general" | "desktop" | "memory" | "automation" | "models";
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
  tone: "live" | "mock" | "pending" | "synced";
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
    items: ["models"],
  },
];

/**
 * Resolves the save feedback copy shown after settings are persisted.
 *
 * @param applyMode Backend apply mode returned by the settings snapshot.
 * @param needRestart Whether the current change set requires an app restart.
 * @param source Control-panel data source mode.
 * @returns User-facing save feedback copy.
 */
function getApplyModeCopy(applyMode: string, needRestart: boolean, source: ControlPanelData["source"]) {
  const localSnapshotSuffix = source === "mock" ? " 当前仍在使用本地快照，不会写入后端。" : "";

  if (needRestart) {
    return `部分设置需要重启桌面端后生效。${localSnapshotSuffix}`;
  }

  if (applyMode === "next_task_effective") {
    return `设置已保存，将在下一个任务周期生效。${localSnapshotSuffix}`;
  }

  return `设置已即时生效。${localSnapshotSuffix}`;
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
        <CircleHelp size={14} strokeWidth={1.9} />
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
  const [panelData, setPanelData] = useState<ControlPanelData | null>(null);
  const [draft, setDraft] = useState<ControlPanelData | null>(null);
  const [saveFeedback, setSaveFeedback] = useState<string | null>(null);
  const [inspectionSummary, setInspectionSummary] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [isRunningInspection, setIsRunningInspection] = useState(false);
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
    void loadControlPanelData().then((nextData) => {
      setPanelData(nextData);
      setDraft(nextData);
    });
  }, []);

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
          <Text size="2" className="control-panel-shell__loading-copy">
            正在载入控制面板…
          </Text>
        </div>
      </main>
    );
  }

  const activeMeta = SECTION_META[activeSection];
  const inspectorDirty = !isEqual(draft.inspector, panelData.inspector);
  const settingsDirty = !isEqual(draft.settings, panelData.settings) || draft.providerApiKeyInput.trim() !== "";
  const hasChanges = inspectorDirty || settingsDirty;
  const pendingFloatingBallHint = "悬浮球联调中：当前仅展示快照，不从控制面板写回。";
  const workSummaryCadence = `${draft.settings.memory.work_summary_interval.value}${draft.settings.memory.work_summary_interval.unit}`;
  const profileCadence = `${draft.settings.memory.profile_refresh_interval.value}${draft.settings.memory.profile_refresh_interval.unit}`;
  const providerApiKeyStatus = draft.settings.models.provider_api_key_configured ? "已配置" : "未配置";
  const resolvedAppearance = resolveControlPanelAppearance(draft.settings.general.theme_mode, systemAppearance);
  const providerApiKeyHint =
    draft.source === "rpc"
      ? "通过 JSON-RPC `agent.settings.update` 提交；只写入后端 Stronghold，不会回显明文。"
      : "当前为本地快照模式：不会写入后端 Stronghold，也不会在桌面端保存明文 API key。";

  const saveStateValue = hasChanges ? <StatusPill tone="pending">待保存</StatusPill> : <StatusPill tone="synced">已同步</StatusPill>;

  const updateSettings = (updater: (current: ControlPanelData) => ControlPanelData) => {
    setDraft((current) => (current ? updater(current) : current));
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
  };

  const handleReplayOnboarding = () => {
    void (async () => {
      await showShellBallWindow("ball");
      await startDesktopOnboarding("manual", "control-panel");
      await requestCurrentDesktopWindowClose();
    })();
  };

  const handleSave = async () => {
    if (!hasChanges || isRunningInspection) {
      return;
    }

    setIsSaving(true);
    try {
      const result = await saveControlPanelData(draft, {
        saveInspector: inspectorDirty,
        saveSettings: settingsDirty,
      });
      const nextPanelData = applyControlPanelSaveResult(panelData, result);
      const nextDraft = applyControlPanelSaveResult(draft, result);
      setPanelData(nextPanelData);
      setDraft(nextDraft);
      setSaveFeedback(getApplyModeCopy(result.applyMode, result.needRestart, result.source));

    } catch (error) {
      if (error instanceof ControlPanelSaveError && error.partialResult) {
        const nextPanelData = applyControlPanelSaveResult(panelData, error.partialResult);
        const nextDraft = applyControlPanelSaveResult(draft, error.partialResult);
        setPanelData(nextPanelData);
        setDraft(nextDraft);
      }

      setSaveFeedback(error instanceof Error ? error.message : "保存控制面板设置失败。");
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
            </SettingsCard>
          </>
        );

      case "desktop":
        return (
          <>
            <SettingsCard title="悬浮球状态" description="控制悬浮球在桌面上的默认表现。">
              <ToggleLine
                label="自动贴边"
                description={pendingFloatingBallHint}
                checked={draft.settings.floating_ball.auto_snap}
                disabled
                onCheckedChange={() => {}}
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
              <ControlLine label="尺寸" hint="在多窗口协作时决定悬浮球的可发现程度。" disabled>
                <div className="control-panel-shell__slider-stack">
                  <Text as="p" size="1" className="control-panel-shell__field-note">
                    {pendingFloatingBallHint}
                  </Text>
                  <Slider
                    min={0}
                    max={2}
                    step={1}
                    disabled
                    value={[draft.settings.floating_ball.size === "small" ? 0 : draft.settings.floating_ball.size === "medium" ? 1 : 2]}
                    onValueChange={() => {}}
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

            <SettingsCard title="记忆节奏" description="显示工作总结与画像刷新的默认频率。">
              <InfoRow label="工作总结间隔" value={workSummaryCadence} />
              <InfoRow label="画像刷新间隔" value={profileCadence} />
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
              </SettingsCard>
            </div>

            <SettingsCard title="安全与预算摘要" description="查看当前安全状态、授权数量与预算限制。">
              <InfoRow label="当前模型" value={draft.settings.models.model} />
              <InfoRow label="API Key 状态" value={providerApiKeyStatus} />
              <InfoRow label="安全状态" value={draft.securitySummary.security_status} />
              <InfoRow label="待确认授权" value={draft.securitySummary.pending_authorizations} />
              <InfoRow label="今日成本" value={`¥${draft.securitySummary.token_cost_summary.today_cost.toFixed(2)}`} />
              <InfoRow
                label="单任务上限"
                value={`${draft.securitySummary.token_cost_summary.single_task_limit.toLocaleString("zh-CN")} tokens`}
              />
              <InfoRow
                label="当日上限"
                value={`${draft.securitySummary.token_cost_summary.daily_limit.toLocaleString("zh-CN")} tokens`}
              />
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
            </div>

            <div className="control-panel-shell__action-buttons">
              <Button
                className="control-panel-shell__button control-panel-shell__button--ghost"
                variant="soft"
                color="gray"
                onClick={handleReplayOnboarding}
                disabled={isSaving || isRunningInspection}
              >
                重新查看新手引导
              </Button>
              <Button
                className="control-panel-shell__button control-panel-shell__button--secondary"
                variant="soft"
                onClick={() => void handleRunInspection()}
                disabled={isRunningInspection || isSaving}
              >
                {isRunningInspection ? "巡检执行中…" : "立即巡检"}
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
