/**
 * Control panel editing flow keeps a local draft so desktop settings can be
 * reviewed and persisted without mutating the last loaded snapshot in place.
 */
import "./controlPanel.css";
import { useEffect, useMemo, useState, type PointerEvent as ReactPointerEvent, type ReactNode } from "react";
import { GripHorizontal, X } from "lucide-react";
import { Button, Heading, SegmentedControl, Slider, Switch, Text, TextArea, TextField } from "@radix-ui/themes";
import isEqual from "react-fast-compare";
import {
  loadControlPanelData,
  runControlPanelInspection,
  saveControlPanelData,
  type ControlPanelData,
} from "@/services/controlPanelService";
import { requestCurrentDesktopWindowClose, startCurrentDesktopWindowDragging } from "@/platform/desktopWindowFrame";

type PanelTone = "blush" | "warm" | "mist" | "leaf";
type ControlPanelAppearance = "light" | "dark";

type SectionHeaderProps = {
  titleId: string;
  title: string;
};

type ControlLineProps = {
  label: string;
  hint?: string;
  children: ReactNode;
  tone?: PanelTone;
  className?: string;
  disabled?: boolean;
};

type ToggleLineProps = {
  label: string;
  description?: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  tone?: PanelTone;
  className?: string;
  disabled?: boolean;
};

type InfoRowProps = {
  label: string;
  value: ReactNode;
  tone?: PanelTone;
  className?: string;
};

/**
 * Maps the current data source into a presentational badge for the control
 * panel header.
 *
 * @param source Control-panel data source mode.
 * @returns Badge copy and color metadata for the UI.
 */
function getSourceCopy(source: ControlPanelData["source"]) {
  if (source === "rpc") {
    return {
      badge: "LIVE",
      label: "JSON-RPC",
      color: "green" as const,
    };
  }

  return {
    badge: "MOCK",
    label: "本地快照",
    color: "amber" as const,
  };
}

/**
 * Resolves the save feedback copy shown after settings are persisted.
 *
 * @param applyMode Backend apply mode returned by the settings snapshot.
 * @param needRestart Whether the current change set requires an app restart.
 * @returns User-facing save feedback copy.
 */
function getApplyModeCopy(applyMode: string, needRestart: boolean, source: ControlPanelData["source"]) {
  const localSnapshotSuffix = source === "mock" ? " 当前仍在使用本地快照，不会写入后端。": "";

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

/**
 * Renders the visual section header used by control panel setting groups.
 *
 * @param props Heading metadata for the current section.
 * @returns A stylized heading row for the section.
 */
function SectionHeader({ titleId, title }: SectionHeaderProps) {
  return (
    <div className="control-panel-page__section-header">
      <Heading id={titleId} size="4" className="control-panel-page__section-title">
        {title}
      </Heading>

      <div className="control-panel-page__section-ornament" aria-hidden="true">
        <span />
        <span />
        <span />
      </div>
    </div>
  );
}

function ControlLine({ label, hint, children, tone = "blush", className, disabled = false }: ControlLineProps) {
  const classes = [
    "control-panel-page__control-line",
    `control-panel-page__tone-surface--${tone}`,
    disabled ? "control-panel-page__surface--disabled" : null,
    className,
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={classes} aria-disabled={disabled}>
      <div className="control-panel-page__control-copy">
        <Text as="p" size="2" weight="medium" className="control-panel-page__field-label">
          {label}
        </Text>
        {hint ? (
          <Text as="p" size="1" className="control-panel-page__field-hint">
            {hint}
          </Text>
        ) : null}
      </div>
      <div className="control-panel-page__control-field">{children}</div>
    </div>
  );
}

function ToggleLine({ label, description, checked, onCheckedChange, tone = "warm", className, disabled = false }: ToggleLineProps) {
  const classes = [
    "control-panel-page__toggle-line",
    `control-panel-page__tone-surface--${tone}`,
    disabled ? "control-panel-page__surface--disabled" : null,
    className,
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={classes} aria-disabled={disabled}>
      <div className="control-panel-page__control-copy">
        <Text as="p" size="2" weight="medium" className="control-panel-page__field-label">
          {label}
        </Text>
        {description ? (
          <Text as="p" size="1" className="control-panel-page__field-hint">
            {description}
          </Text>
        ) : null}
      </div>
      <Switch checked={checked} onCheckedChange={onCheckedChange} disabled={disabled} />
    </div>
  );
}

function InfoRow({ label, value, tone = "mist", className }: InfoRowProps) {
  const classes = ["control-panel-page__info-row", `control-panel-page__tone-surface--${tone}`, className]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={classes}>
      <Text as="span" size="1" className="control-panel-page__info-label">
        {label}
      </Text>
      <Text as="span" size="2" weight="medium" className="control-panel-page__info-value">
        {value}
      </Text>
    </div>
  );
}

export function ControlPanelApp() {
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

  const sourceCopy = useMemo(() => (draft ? getSourceCopy(draft.source) : null), [draft]);

  const hasChanges = !isEqual(draft, panelData);
  const pendingFloatingBallHint = "悬浮球联调中：当前仅展示快照，不从控制面板写回。";

  if (!draft || !panelData || !sourceCopy) {
    return (
      <main className="app-shell control-panel-page" data-appearance={systemAppearance}>
        <div className="control-panel-page__loading">
          <Text size="2" className="control-panel-page__loading-copy">
            正在载入控制面板…
          </Text>
        </div>
      </main>
    );
  }

  const latestRestorePoint = draft.securitySummary.latest_restore_point?.summary ?? "暂无恢复点";
  const inspectionInterval = `${draft.inspector.inspection_interval.value}${draft.inspector.inspection_interval.unit}`;
  const workSummaryCadence = `${draft.settings.memory.work_summary_interval.value}${draft.settings.memory.work_summary_interval.unit}`;
  const profileCadence = `${draft.settings.memory.profile_refresh_interval.value}${draft.settings.memory.profile_refresh_interval.unit}`;
  const providerApiKeyStatus = draft.settings.models.provider_api_key_configured ? "已配置" : "未配置";
  const resolvedAppearance = resolveControlPanelAppearance(draft.settings.general.theme_mode, systemAppearance);
  const providerApiKeyHint =
    draft.source === "rpc"
      ? "通过 JSON-RPC `agent.settings.update` 提交；只写入后端 Stronghold，不会回显明文。"
      : "当前为本地快照模式：不会写入后端 Stronghold，也不会在桌面端保存明文 API key。";
  const sourceValue = (
    <span className="control-panel-page__value-cluster">
      <span
        className={`control-panel-page__value-badge ${
          draft.source === "rpc"
            ? "control-panel-page__value-badge--live"
            : "control-panel-page__value-badge--mock"
        }`}
      >
        {sourceCopy.badge}
      </span>
      <span className="control-panel-page__value-text">{sourceCopy.label}</span>
    </span>
  );
  const saveStateValue = (
    <span
      className={`control-panel-page__value-badge ${
        hasChanges ? "control-panel-page__value-badge--pending" : "control-panel-page__value-badge--synced"
      }`}
    >
      {hasChanges ? "待保存" : "已同步"}
    </span>
  );

  const updateSettings = (updater: (current: ControlPanelData) => ControlPanelData) => {
    setDraft((current) => (current ? updater(current) : current));
  };

  // The top bar doubles as the drag region, but interactive controls inside it
  // must keep their own pointer behavior instead of starting a window move.
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

  // Stop the close button press from bubbling into the drag region before
  // the click handler requests the native close flow.
  const handleWindowClosePointerDown = (event: ReactPointerEvent<HTMLButtonElement>) => {
    event.stopPropagation();
  };

  const handleWindowCloseClick = () => {
    void requestCurrentDesktopWindowClose();
  };

  const handleSave = async () => {
    setIsSaving(true);
    try {
      const result = await saveControlPanelData(draft);
      const nextData: ControlPanelData = {
        ...draft,
        inspector: result.effectiveInspector,
        providerApiKeyInput: "",
        settings: {
          ...draft.settings,
          ...result.effectiveSettings,
          task_automation: {
            ...draft.settings.task_automation,
            ...(result.effectiveSettings.task_automation ?? {}),
          },
        },
      };
      setPanelData(nextData);
      setDraft(nextData);
      setSaveFeedback(getApplyModeCopy(result.applyMode, result.needRestart, result.source));
    } catch (error) {
      setSaveFeedback(error instanceof Error ? error.message : "保存控制面板设置失败。");
    } finally {
      setIsSaving(false);
    }
  };

  const handleRunInspection = async () => {
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

  return (
    <main className="app-shell control-panel-page" data-appearance={resolvedAppearance}>
      <div className="control-panel-page__frame">
        <section
          className="control-panel-page__topbar control-panel-page__tone-surface--warm"
          aria-label="控制面板窗口操作"
          onPointerDown={handleTopbarPointerDown}
        >
          <div className="control-panel-page__page-copy">
            <Text as="p" size="1" className="control-panel-page__meta-label">
              Desktop Control Surface
            </Text>
            <Heading size="6" className="control-panel-page__page-title">
              控制面板
            </Heading>
            <Text as="p" size="2" className="control-panel-page__meta-text">
              拖动顶部区域可以移动窗口，按 Esc 或右上角按钮可以关闭。
            </Text>
          </div>

          <div className="control-panel-page__topbar-side">
            <div className="control-panel-page__meta-grid">
              <div className="control-panel-page__meta-item">
                <Text as="p" size="1" className="control-panel-page__meta-label">
                  数据来源
                </Text>
                <div className="control-panel-page__meta-value">{sourceValue}</div>
              </div>

              <div className="control-panel-page__meta-item">
                <Text as="p" size="1" className="control-panel-page__meta-label">
                  保存状态
                </Text>
                <div className="control-panel-page__meta-value">{saveStateValue}</div>
              </div>

              <div className="control-panel-page__meta-item">
                <Text as="p" size="1" className="control-panel-page__meta-label">
                  最近恢复点
                </Text>
                <Text as="p" size="2" className="control-panel-page__meta-text">
                  {latestRestorePoint}
                </Text>
              </div>
            </div>

            <div className="control-panel-page__window-actions">
              <button
                type="button"
                className="control-panel-page__frame-button control-panel-page__frame-button--drag"
                aria-label="拖动控制面板窗口"
                onPointerDown={handleWindowDragPointerDown}
              >
                <GripHorizontal className="control-panel-page__frame-button-icon" />
                <span>拖动窗口</span>
              </button>

              <button
                type="button"
                className="control-panel-page__frame-button control-panel-page__frame-button--close"
                aria-label="关闭控制面板"
                onClick={handleWindowCloseClick}
                onPointerDown={handleWindowClosePointerDown}
              >
                <X className="control-panel-page__frame-button-icon" />
              </button>
            </div>
          </div>
        </section>
        <div className="control-panel-page__columns" aria-label="控制面板设置分组">
          <div className="control-panel-page__column">
            <section className="control-panel-page__panel control-panel-page__tone-surface--blush" aria-labelledby="control-panel-general-title">
              <SectionHeader titleId="control-panel-general-title" title="通用" />

              <div className="control-panel-page__stack">
                <ControlLine label="语言" tone="blush">
                  <TextField.Root
                    className="control-panel-page__input"
                    value={draft.settings.general.language}
                    onChange={(event) =>
                      updateSettings((current) => ({
                        ...current,
                        settings: {
                          ...current.settings,
                          general: { ...current.settings.general, language: event.target.value },
                        },
                      }))
                    }
                  />
                </ControlLine>

                <ControlLine label="工作区路径" tone="blush">
                  <TextField.Root
                    className="control-panel-page__input"
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
                  label="开机自启"
                  checked={draft.settings.general.auto_launch}
                  tone="blush"
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
                  checked={draft.settings.general.voice_notification_enabled}
                  tone="blush"
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

                <ControlLine label="主题" tone="blush">
                  <SegmentedControl.Root
                    value={draft.settings.general.theme_mode}
                    onValueChange={(value) =>
                      updateSettings((current) => ({
                        ...current,
                        settings: {
                          ...current.settings,
                          general: { ...current.settings.general, theme_mode: value as typeof current.settings.general.theme_mode },
                        },
                      }))
                    }
                    className="control-panel-page__selector"
                  >
                    <SegmentedControl.Item value="follow_system">跟随系统</SegmentedControl.Item>
                    <SegmentedControl.Item value="light">浅色</SegmentedControl.Item>
                    <SegmentedControl.Item value="dark">深色</SegmentedControl.Item>
                  </SegmentedControl.Root>
                </ControlLine>
              </div>
            </section>

            <section className="control-panel-page__panel control-panel-page__tone-surface--mist" aria-labelledby="control-panel-memory-title">
              <SectionHeader titleId="control-panel-memory-title" title="记忆" />

              <div className="control-panel-page__stack">
                <div className="control-panel-page__info-list">
                  <InfoRow label="工作总结间隔" value={workSummaryCadence} tone="mist" />
                  <InfoRow label="画像刷新间隔" value={profileCadence} tone="mist" />
                </div>

                <ToggleLine
                  label="启用记忆"
                  checked={draft.settings.memory.enabled}
                  tone="mist"
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

                <ControlLine label="生命周期" tone="mist">
                  <TextField.Root
                    className="control-panel-page__input"
                    value={draft.settings.memory.lifecycle}
                    onChange={(event) =>
                      updateSettings((current) => ({
                        ...current,
                        settings: {
                          ...current.settings,
                          memory: { ...current.settings.memory, lifecycle: event.target.value },
                        },
                      }))
                    }
                  />
                </ControlLine>
              </div>
            </section>

            <section className="control-panel-page__panel control-panel-page__tone-surface--warm" aria-labelledby="control-panel-floating-title">
              <SectionHeader titleId="control-panel-floating-title" title="悬浮球" />

              <div className="control-panel-page__stack">
                <ToggleLine
                  label="自动贴边"
                  description={pendingFloatingBallHint}
                  checked={draft.settings.floating_ball.auto_snap}
                  tone="warm"
                  disabled
                  onCheckedChange={() => {}}
                />

                <ToggleLine
                  label="空闲半透明"
                  checked={draft.settings.floating_ball.idle_translucent}
                  tone="warm"
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

                <ControlLine label="尺寸" tone="warm">
                  <div className="control-panel-page__slider-stack">
                    <Text as="p" size="1" className="control-panel-page__field-hint">
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

                    <div className="control-panel-page__slider-legend">
                      <span>小</span>
                      <span>中</span>
                      <span>大</span>
                    </div>
                  </div>
                </ControlLine>

                <ControlLine label="停靠方式" tone="warm">
                  <SegmentedControl.Root
                    value={draft.settings.floating_ball.position_mode}
                    onValueChange={(value) =>
                      updateSettings((current) => ({
                        ...current,
                        settings: {
                          ...current.settings,
                          floating_ball: {
                            ...current.settings.floating_ball,
                            position_mode: value as typeof current.settings.floating_ball.position_mode,
                          },
                        },
                      }))
                    }
                    className="control-panel-page__selector"
                  >
                    <SegmentedControl.Item value="fixed">固定</SegmentedControl.Item>
                    <SegmentedControl.Item value="draggable">拖动</SegmentedControl.Item>
                  </SegmentedControl.Root>
                </ControlLine>
              </div>
            </section>
          </div>

          <div className="control-panel-page__column">
            <section className="control-panel-page__panel control-panel-page__tone-surface--leaf" aria-labelledby="control-panel-inspection-title">
              <SectionHeader titleId="control-panel-inspection-title" title="巡检" />

              <div className="control-panel-page__stack">
                <div className="control-panel-page__info-list">
                  <InfoRow label="任务来源" value={`${draft.inspector.task_sources.length} 项`} tone="leaf" />
                  <InfoRow label="巡检频率" value={inspectionInterval} tone="leaf" />
                </div>

                <ToggleLine
                  label="开机巡检"
                  checked={draft.inspector.inspect_on_startup}
                  tone="leaf"
                  onCheckedChange={(checked) =>
                    updateSettings((current) => ({
                      ...current,
                      inspector: { ...current.inspector, inspect_on_startup: checked },
                    }))
                  }
                />

                <ToggleLine
                  label="文件变化时巡检"
                  checked={draft.inspector.inspect_on_file_change}
                  tone="leaf"
                  onCheckedChange={(checked) =>
                    updateSettings((current) => ({
                      ...current,
                      inspector: { ...current.inspector, inspect_on_file_change: checked },
                    }))
                  }
                />

                <ToggleLine
                  label="截止前提醒"
                  checked={draft.inspector.remind_before_deadline}
                  tone="leaf"
                  onCheckedChange={(checked) =>
                    updateSettings((current) => ({
                      ...current,
                      inspector: { ...current.inspector, remind_before_deadline: checked },
                    }))
                  }
                />

                <ToggleLine
                  label="陈旧任务提醒"
                  checked={draft.inspector.remind_when_stale}
                  tone="leaf"
                  onCheckedChange={(checked) =>
                    updateSettings((current) => ({
                      ...current,
                      inspector: { ...current.inspector, remind_when_stale: checked },
                    }))
                  }
                />

                <ControlLine
                  label="任务来源列表"
                  hint="每行一个路径或标签。"
                  tone="leaf"
                  className="control-panel-page__control-line--textarea"
                >
                  <TextArea
                    className="control-panel-page__textarea"
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
              </div>
            </section>

            <section className="control-panel-page__panel control-panel-page__tone-surface--warm" aria-labelledby="control-panel-budget-title">
              <SectionHeader titleId="control-panel-budget-title" title="模型与路由" />

              <div className="control-panel-page__stack">
                <ControlLine label="Provider" tone="warm">
                  <TextField.Root
                    className="control-panel-page__input"
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

                <ControlLine label="Base URL" tone="warm">
                  <TextField.Root
                    className="control-panel-page__input"
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

                <ControlLine label="Model" tone="warm">
                  <TextField.Root
                    className="control-panel-page__input"
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

                <ControlLine
                  label="API Key"
                  hint={providerApiKeyHint}
                  tone="warm"
                >
                  <TextField.Root
                    className="control-panel-page__input"
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
                  checked={draft.settings.models.budget_auto_downgrade}
                  tone="warm"
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

                <div className="control-panel-page__info-list">
                  <InfoRow label="当前模型" value={draft.settings.models.model} tone="warm" />
                  <InfoRow label="API Key 状态" value={providerApiKeyStatus} tone="warm" />
                  <InfoRow label="安全状态" value={draft.securitySummary.security_status} tone="warm" />
                  <InfoRow label="待确认授权" value={draft.securitySummary.pending_authorizations} tone="warm" />
                  <InfoRow
                    label="今日成本"
                    value={`¥${draft.securitySummary.token_cost_summary.today_cost.toFixed(2)}`}
                    tone="warm"
                  />
                  <InfoRow
                    label="单任务上限"
                    value={`${draft.securitySummary.token_cost_summary.single_task_limit.toLocaleString("zh-CN")} tokens`}
                    tone="warm"
                  />
                  <InfoRow
                    label="当日上限"
                    value={`${draft.securitySummary.token_cost_summary.daily_limit.toLocaleString("zh-CN")} tokens`}
                    tone="warm"
                  />
                </div>
              </div>
            </section>

            <section className="control-panel-page__panel control-panel-page__tone-surface--blush" aria-labelledby="control-panel-actions-title">
              <SectionHeader titleId="control-panel-actions-title" title="操作" />

              <div className="control-panel-page__stack">
                <div className="control-panel-page__info-list">
                  <InfoRow label="数据来源" value={sourceValue} tone="blush" />
                  <InfoRow label="保存状态" value={saveStateValue} tone="blush" />
                  <InfoRow label="巡检频率" value={inspectionInterval} tone="blush" />
                  <InfoRow label="恢复点" value={latestRestorePoint} tone="blush" />
                </div>

                <div className="control-panel-page__button-row">
                  <Button
                    className="control-panel-page__button control-panel-page__button--secondary"
                    variant="soft"
                    onClick={() => void handleRunInspection()}
                    disabled={isRunningInspection}
                  >
                    {isRunningInspection ? "巡检执行中…" : "立即巡检"}
                  </Button>

                  <Button
                    className="control-panel-page__button control-panel-page__button--ghost"
                    variant="soft"
                    color="gray"
                    onClick={() => setDraft(panelData)}
                    disabled={!hasChanges || isSaving}
                  >
                    撤销修改
                  </Button>

                  <Button
                    className="control-panel-page__button control-panel-page__button--primary"
                    onClick={() => void handleSave()}
                    disabled={!hasChanges || isSaving}
                  >
                    {isSaving ? "保存中…" : "保存设置"}
                  </Button>
                </div>

                <div className="control-panel-page__feedback-list">
                  <div className="control-panel-page__message control-panel-page__tone-surface--warm">
                    <Text as="p" size="1" className="control-panel-page__meta-label">
                      巡检结果
                    </Text>
                    <Text as="p" size="2" className="control-panel-page__message-text" aria-live="polite">
                      {inspectionSummary ?? "手动巡检后显示结果。"}
                    </Text>
                  </div>

                  <div className="control-panel-page__message control-panel-page__tone-surface--blush">
                    <Text as="p" size="1" className="control-panel-page__meta-label">
                      保存结果
                    </Text>
                    <Text as="p" size="2" className="control-panel-page__message-text" aria-live="polite">
                      {saveFeedback ?? "保存后显示结果。"}
                    </Text>
                  </div>
                </div>
              </div>
            </section>
          </div>
        </div>
      </div>
    </main>
  );
}
