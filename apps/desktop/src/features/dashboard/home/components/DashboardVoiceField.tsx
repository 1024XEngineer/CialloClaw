import { useCallback, useEffect, useId, useMemo, useRef, useState } from "react";
import { motion, AnimatePresence } from "motion/react";
import { Mic, Sparkles, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { getShellBallMotionConfig } from "@/features/shell-ball/shellBall.motion";
import { ShellBallMascot } from "@/features/shell-ball/components/ShellBallMascot";
import type { ShellBallVisualState } from "@/features/shell-ball/shellBall.types";
import type { DashboardHomeModuleKey, DashboardVoiceSequence, DashboardVoiceStage } from "../dashboardHome.types";
import "@/features/shell-ball/shellBall.css";
import "../dashboardHome.css";

type DashboardVoiceFieldProps = {
  isOpen: boolean;
  onClose: () => void;
  onCommand: (module: DashboardHomeModuleKey) => void;
  sequences: DashboardVoiceSequence[];
};

const stageOrder: DashboardVoiceStage[] = ["ready", "listening", "understanding", "confirming", "executing"];

export function DashboardVoiceField({ isOpen, onClose, onCommand, sequences }: DashboardVoiceFieldProps) {
  const [stage, setStage] = useState<DashboardVoiceStage>("ready");
  const [summary, setSummary] = useState("");
  const [fragments, setFragments] = useState<string[]>([]);
  const [executingStep, setExecutingStep] = useState("");
  const [executingProgress, setExecutingProgress] = useState(0);
  const [currentSequenceIndex, setCurrentSequenceIndex] = useState(0);
  const timerRef = useRef<number | null>(null);
  const executionTimersRef = useRef<number[]>([]);
  const panelRef = useRef<HTMLDivElement | null>(null);
  const closeButtonRef = useRef<HTMLButtonElement | null>(null);
  const previousFocusRef = useRef<HTMLElement | null>(null);
  const titleId = useId();
  const descriptionId = useId();

  const activeSequence = sequences[currentSequenceIndex % sequences.length];
  const mascotState: ShellBallVisualState = stage === "listening" ? "voice_listening" : stage === "executing" ? "voice_locked" : stage === "understanding" ? "processing" : "hover_input";
  const motionConfig = useMemo(() => getShellBallMotionConfig(mascotState), [mascotState]);

  const clearTimer = useCallback(() => {
    if (timerRef.current) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }

    executionTimersRef.current.forEach((timer) => {
      window.clearTimeout(timer);
    });
    executionTimersRef.current = [];
  }, []);

  useEffect(() => {
    if (!isOpen) {
      clearTimer();
      setStage("ready");
      setFragments([]);
      setSummary("");
      setExecutingStep("");
      setExecutingProgress(0);
      return;
    }

    setStage("ready");
    setFragments([]);
    setSummary("");
    setExecutingStep("");
    setExecutingProgress(0);

    timerRef.current = window.setTimeout(() => setStage("listening"), 720);

    return clearTimer;
  }, [clearTimer, isOpen]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    previousFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const focusFrame = window.requestAnimationFrame(() => {
      closeButtonRef.current?.focus();
    });

    return () => {
      window.cancelAnimationFrame(focusFrame);
      if (previousFocusRef.current && document.contains(previousFocusRef.current)) {
        previousFocusRef.current.focus();
      }
    };
  }, [isOpen]);

  useEffect(() => {
    clearTimer();

    if (!isOpen) {
      return;
    }

    if (stage === "listening") {
      timerRef.current = window.setTimeout(() => {
        setFragments(activeSequence.fragments);
        setStage("understanding");
      }, 2100);
      return;
    }

    if (stage === "understanding") {
      timerRef.current = window.setTimeout(() => {
        setSummary(activeSequence.summary);
        setStage("confirming");
      }, 1200);
      return;
    }

    return;
  }, [activeSequence.fragments, activeSequence.summary, clearTimer, isOpen, stage]);

  useEffect(() => {
    if (!isOpen || stage !== "executing") {
      return;
    }

    activeSequence.executingSteps.forEach((step, index) => {
      const stepTimer = window.setTimeout(() => {
        setExecutingStep(step);
        setExecutingProgress(Math.round(((index + 1) / activeSequence.executingSteps.length) * 100));
        if (index === activeSequence.executingSteps.length - 1) {
          const closeTimer = window.setTimeout(() => {
            onCommand(activeSequence.module);
            onClose();
            setCurrentSequenceIndex((current) => current + 1);
          }, 900);
          executionTimersRef.current.push(closeTimer);
        }
      }, index * 680);

      executionTimersRef.current.push(stepTimer);
    });

    return clearTimer;
  }, [activeSequence.executingSteps, activeSequence.module, clearTimer, isOpen, onClose, onCommand, stage]);

  function handleConfirm() {
    setStage("executing");
  }

  function handleRetry() {
    setFragments([]);
    setSummary("");
    setStage("listening");
  }

  function handlePanelKeyDown(event: React.KeyboardEvent<HTMLDivElement>) {
    if (event.key === "Escape") {
      event.preventDefault();
      onClose();
      return;
    }

    if (event.key !== "Tab") {
      return;
    }

    const focusable = panelRef.current?.querySelectorAll<HTMLElement>(
      'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
    );

    if (!focusable || focusable.length === 0) {
      event.preventDefault();
      panelRef.current?.focus();
      return;
    }

    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    const activeElement = document.activeElement;

    if (event.shiftKey && activeElement === first) {
      event.preventDefault();
      last.focus();
      return;
    }

    if (!event.shiftKey && activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }

  if (!isOpen) {
    return null;
  }

  return (
    <AnimatePresence>
      <motion.div
        animate={{ opacity: 1 }}
        aria-hidden="false"
        className="dashboard-voice-field"
        exit={{ opacity: 0 }}
        initial={{ opacity: 0 }}
        onClick={onClose}
      >
        <motion.div
          animate={{ opacity: 1, scale: 1, y: 0 }}
          aria-describedby={descriptionId}
          aria-labelledby={titleId}
          aria-modal="true"
          className="dashboard-voice-field__panel"
          exit={{ opacity: 0, scale: 0.96, y: 16 }}
          initial={{ opacity: 0, scale: 0.96, y: 12 }}
          onClick={(event) => event.stopPropagation()}
          onKeyDown={handlePanelKeyDown}
          ref={panelRef}
          role="dialog"
          tabIndex={-1}
          transition={{ duration: 0.32, ease: [0.22, 1, 0.36, 1] }}
        >
          <div className="dashboard-voice-field__header">
            <div>
              <p className="dashboard-orbit-panel__eyebrow">voice field</p>
              <h2 className="dashboard-voice-field__title" id={titleId}>
                语音陪伴场
              </h2>
            </div>
            <Button className="dashboard-orbit-panel__close" onClick={onClose} ref={closeButtonRef} size="icon-sm" variant="ghost">
              <X className="h-4 w-4" />
              <span className="sr-only">关闭语音场</span>
            </Button>
          </div>

          <div className="dashboard-voice-field__stage-row">
            {stageOrder.map((item) => (
              <span key={item} className="dashboard-voice-field__stage-dot" data-active={item === stage ? "true" : "false"} />
            ))}
          </div>

          <div className="dashboard-voice-field__orb-shell">
            <div className="dashboard-voice-field__wave dashboard-voice-field__wave--outer" data-stage={stage} />
            <div className="dashboard-voice-field__wave dashboard-voice-field__wave--middle" data-stage={stage} />
            <div className="dashboard-voice-field__wave dashboard-voice-field__wave--inner" data-stage={stage} />
            <div className="dashboard-voice-field__mascot-shell">
              <ShellBallMascot
                motionConfig={motionConfig}
                onPressEnd={() => false}
                onPressMove={() => {}}
                onPressStart={() => {}}
                onPrimaryClick={() => {}}
                showVoiceHints={false}
                visualState={mascotState}
                voicePreview={null}
              />
            </div>
          </div>

          <div className="dashboard-voice-field__copy">
            <p className="dashboard-voice-field__status">
              <Mic className="h-4 w-4" />
              {stage === "ready"
                ? "直接说出你的想法"
                : stage === "listening"
                  ? "正在听…"
                  : stage === "understanding"
                    ? "我在整理你的意思"
                    : stage === "confirming"
                      ? "你是想让我…"
                      : "正在处理…"}
            </p>
            <p className="dashboard-voice-field__subline" id={descriptionId}>
              {stage === "confirming"
                ? summary
                : stage === "executing"
                  ? executingStep
                  : fragments.length > 0
                    ? fragments.join(" · ")
                    : activeSequence.suggestion}
            </p>
          </div>

          {stage === "confirming" ? (
            <div className="dashboard-voice-field__actions">
              <Button className="dashboard-orbit-panel__primary-button" onClick={handleConfirm}>
                开始执行
                <Sparkles className="h-4 w-4" />
              </Button>
              <Button className="dashboard-orbit-panel__action-button dashboard-orbit-panel__action-button--mute" onClick={handleRetry} variant="ghost">
                再详细一点
              </Button>
              <Button className="dashboard-orbit-panel__action-button dashboard-orbit-panel__action-button--mute" onClick={onClose} variant="ghost">
                取消
              </Button>
            </div>
          ) : null}

          {stage === "executing" ? (
            <div className="dashboard-voice-field__progress-bar">
              <motion.div animate={{ width: `${executingProgress}%` }} className="dashboard-voice-field__progress-fill" initial={{ width: 0 }} transition={{ duration: 0.34, ease: [0.22, 1, 0.36, 1] }} />
            </div>
          ) : (
            <div className="dashboard-voice-field__suggestions">
              {sequences.map((sequence, index) => (
                <button
                  key={sequence.suggestion}
                  className="dashboard-voice-field__suggestion-chip"
                  data-active={index === currentSequenceIndex % sequences.length ? "true" : "false"}
                  onClick={() => {
                    setCurrentSequenceIndex(index);
                    setStage("listening");
                    setFragments([]);
                    setSummary("");
                  }}
                  type="button"
                >
                  {sequence.suggestion}
                </button>
              ))}
            </div>
          )}
        </motion.div>
      </motion.div>
    </AnimatePresence>
  );
}
