import { useCallback, useEffect, useId, useMemo, useRef, useState } from "react";
import type { Task } from "@cialloclaw/protocol";
import { motion, AnimatePresence } from "motion/react";
import { LoaderCircle, Mic, RotateCcw, Sparkles, X } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { submitTextInput } from "@/services/agentInputService";
import type { DashboardVoiceSequence } from "@/features/dashboard/home/dashboardHome.types";
import { buildDashboardTaskDetailRouteState } from "@/features/dashboard/shared/dashboardTaskDetailNavigation";
import { resolveDashboardModuleRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import { getShellBallMotionConfig } from "@/features/shell-ball/shellBall.motion";
import { ShellBallMascot } from "@/features/shell-ball/components/ShellBallMascot";
import {
  collectShellBallSpeechTranscript,
  getShellBallSpeechRecognitionConstructor,
  getShellBallSpeechRecognitionLanguage,
  normalizeShellBallSpeechTranscript,
  type ShellBallSpeechRecognition,
} from "@/features/shell-ball/shellBall.speech";
import type { ShellBallVisualState } from "@/features/shell-ball/shellBall.types";
import "@/features/shell-ball/shellBall.css";
import "@/features/dashboard/home/dashboardHome.css";

type DashboardVoiceFieldProps = {
  isOpen: boolean;
  onClose: () => void;
  onRecommendationConfirm?: (recommendationId: string) => void;
  sequences: DashboardVoiceSequence[];
};

type DashboardVoiceStage = "ready" | "listening" | "submitting" | "completed" | "error";

type DashboardVoiceRecognitionStopReason = "none" | "finish" | "cancel";

const stageOrder: DashboardVoiceStage[] = ["ready", "listening", "submitting", "completed", "error"];

function getDashboardVoiceStatusLabel(stage: DashboardVoiceStage, taskStatus: Task["status"] | null) {
  if (stage === "ready") {
    return "正在准备语音场";
  }

  if (stage === "listening") {
    return "正在听…";
  }

  if (stage === "submitting") {
    return "正在提交到任务主链路";
  }

  if (stage === "error") {
    return "语音暂未完成";
  }

  switch (taskStatus) {
    case "confirming_intent":
      return "已进入意图确认";
    case "waiting_auth":
      return "已进入授权确认";
    case "waiting_input":
      return "正在等待补充信息";
    case "blocked":
      return "任务暂时被阻塞";
    case "failed":
      return "任务返回失败状态";
    case "completed":
      return "任务已完成";
    default:
      return "语音内容已提交";
  }
}

function getDashboardVoiceTaskRoute(status: Task["status"]) {
  return status === "waiting_auth" ? resolveDashboardModuleRoutePath("safety") : resolveDashboardModuleRoutePath("tasks");
}

function getSpeechRecognitionErrorMessage(error: string) {
  switch (error) {
    case "audio-capture":
      return "当前没有可用的麦克风设备。";
    case "network":
      return "语音转写网络暂时不可用，请稍后再试。";
    case "not-allowed":
    case "service-not-allowed":
      return "当前环境没有语音转写权限。";
    case "no-speech":
      return "没有检测到语音内容，请再试一次。";
    default:
      return "语音转写暂时不可用，请再试一次。";
  }
}

export function DashboardVoiceField({ isOpen, onClose, onRecommendationConfirm, sequences }: DashboardVoiceFieldProps) {
  const navigate = useNavigate();
  const [stage, setStage] = useState<DashboardVoiceStage>("ready");
  const [transcript, setTranscript] = useState("");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [submittedTaskStatus, setSubmittedTaskStatus] = useState<Task["status"] | null>(null);
  const [submittedTaskId, setSubmittedTaskId] = useState<string | null>(null);
  const [submittedMessage, setSubmittedMessage] = useState<string | null>(null);
  const panelRef = useRef<HTMLDivElement | null>(null);
  const closeButtonRef = useRef<HTMLButtonElement | null>(null);
  const previousFocusRef = useRef<HTMLElement | null>(null);
  const completionTimerRef = useRef<number | null>(null);
  const transcriptRef = useRef("");
  const recognitionRef = useRef<ShellBallSpeechRecognition | null>(null);
  const recognitionSessionIdRef = useRef(0);
  const recognitionStopReasonRef = useRef<DashboardVoiceRecognitionStopReason>("none");
  const recognitionErrorMessageRef = useRef<string | null>(null);
  const titleId = useId();
  const descriptionId = useId();

  const activeStageIndex = stageOrder.indexOf(stage);
  const hasTranscript = transcript.trim() !== "";
  const mascotState: ShellBallVisualState = stage === "listening" ? "voice_listening" : stage === "submitting" ? "processing" : stage === "completed" ? "voice_locked" : "hover_input";
  const motionConfig = useMemo(() => getShellBallMotionConfig(mascotState), [mascotState]);

  const clearCompletionTimer = useCallback(() => {
    if (completionTimerRef.current !== null) {
      window.clearTimeout(completionTimerRef.current);
      completionTimerRef.current = null;
    }
  }, []);

  const setLiveTranscript = useCallback((value: string) => {
    transcriptRef.current = value;
    setTranscript(value);
  }, []);

  const disposeVoiceRecognition = useCallback(() => {
    recognitionSessionIdRef.current += 1;
    recognitionStopReasonRef.current = "none";
    recognitionErrorMessageRef.current = null;
    const recognition = recognitionRef.current;
    recognitionRef.current = null;

    if (recognition === null) {
      return;
    }

    recognition.onresult = null;
    recognition.onerror = null;
    recognition.onend = null;

    try {
      recognition.abort();
    } catch {}
  }, []);

  const resetVoiceField = useCallback(() => {
    clearCompletionTimer();
    recognitionErrorMessageRef.current = null;
    setStage("ready");
    setErrorMessage(null);
    setSubmittedTaskStatus(null);
    setSubmittedTaskId(null);
    setSubmittedMessage(null);
    setLiveTranscript("");
  }, [clearCompletionTimer, setLiveTranscript]);

  const submitDashboardVoiceText = useCallback(async (text: string) => {
    const finalizedTranscript = normalizeShellBallSpeechTranscript(text);

    if (finalizedTranscript === "") {
      setStage("error");
      setErrorMessage("没有收到可提交的语音内容，请再试一次。");
      return;
    }

    setLiveTranscript(finalizedTranscript);
    setStage("submitting");
    setErrorMessage(null);

    try {
      const result = await submitTextInput({
        text: finalizedTranscript,
        source: "dashboard",
        trigger: "voice_commit",
        inputMode: "voice",
        includeForegroundBrowserPageContext: true,
      });

      if (result === null) {
        setStage("error");
        setErrorMessage("没有收到可提交的语音内容，请再试一次。");
        return;
      }

      setSubmittedMessage(result.bubble_message?.text?.trim() || null);
      setStage("completed");
      clearCompletionTimer();
      const task = result.task;
      if (!task) {
        setSubmittedTaskStatus(null);
        setSubmittedTaskId(null);
        completionTimerRef.current = window.setTimeout(() => {
          onClose();
        }, 720);
        return;
      }

      setSubmittedTaskStatus(task.status);
      setSubmittedTaskId(task.task_id);
      completionTimerRef.current = window.setTimeout(() => {
        navigate(getDashboardVoiceTaskRoute(task.status), {
          state: task.status === "waiting_auth"
            ? undefined
            : buildDashboardTaskDetailRouteState(task.task_id),
        });
        onClose();
      }, 720);
    } catch (error) {
      console.warn("dashboard voice submit failed", error);
      setStage("error");
      setErrorMessage("语音内容提交失败，请稍后重试。");
    }
  }, [clearCompletionTimer, navigate, onClose, setLiveTranscript]);

  const finalizeVoiceRecognition = useCallback(async (reason: Exclude<DashboardVoiceRecognitionStopReason, "none">) => {
    recognitionRef.current = null;
    recognitionStopReasonRef.current = "none";
    recognitionSessionIdRef.current += 1;
    const finalizedTranscript = normalizeShellBallSpeechTranscript(transcriptRef.current);

    if (reason === "cancel") {
      setStage("error");
      setErrorMessage(recognitionErrorMessageRef.current ?? "语音转写已取消，请重试。");
      return;
    }

    if (finalizedTranscript === "") {
      setStage("error");
      setErrorMessage(recognitionErrorMessageRef.current ?? "没有收到可提交的语音内容，请再试一次。");
      return;
    }

    await submitDashboardVoiceText(finalizedTranscript);
  }, [submitDashboardVoiceText]);

  const stopVoiceRecognition = useCallback((reason: Exclude<DashboardVoiceRecognitionStopReason, "none">) => {
    recognitionStopReasonRef.current = reason;
    const recognition = recognitionRef.current;

    if (recognition === null) {
      void finalizeVoiceRecognition(reason);
      return;
    }

    try {
      if (reason === "cancel") {
        recognition.abort();
        return;
      }

      recognition.stop();
    } catch {
      void finalizeVoiceRecognition(reason);
    }
  }, [finalizeVoiceRecognition]);

  const startVoiceRecognition = useCallback(() => {
    const Recognition = getShellBallSpeechRecognitionConstructor();

    if (Recognition === null) {
      setStage("error");
      setErrorMessage("当前环境不支持语音转写。\n请检查系统浏览器能力后重试。");
      return false;
    }

    disposeVoiceRecognition();
    clearCompletionTimer();
    setLiveTranscript("");
    setErrorMessage(null);
    setStage("listening");
    recognitionErrorMessageRef.current = null;
    recognitionSessionIdRef.current += 1;
    const sessionId = recognitionSessionIdRef.current;
    const recognition = new Recognition();
    recognitionRef.current = recognition;
    recognitionStopReasonRef.current = "none";
    recognition.continuous = true;
    recognition.interimResults = true;
    recognition.lang = getShellBallSpeechRecognitionLanguage();
    recognition.maxAlternatives = 1;

    recognition.onresult = (event) => {
      if (sessionId !== recognitionSessionIdRef.current) {
        return;
      }

      setLiveTranscript(collectShellBallSpeechTranscript(event.results));
    };

    recognition.onerror = (event) => {
      if (sessionId !== recognitionSessionIdRef.current) {
        return;
      }

      if (recognitionStopReasonRef.current === "cancel" && event.error === "aborted") {
        return;
      }

      recognitionErrorMessageRef.current = getSpeechRecognitionErrorMessage(event.error);
      recognitionStopReasonRef.current = "cancel";
    };

    recognition.onend = () => {
      if (sessionId !== recognitionSessionIdRef.current) {
        return;
      }

      const stopReason = recognitionStopReasonRef.current;

      if (stopReason === "finish" || stopReason === "cancel") {
        void finalizeVoiceRecognition(stopReason);
        return;
      }

      if (normalizeShellBallSpeechTranscript(transcriptRef.current) !== "") {
        void finalizeVoiceRecognition("finish");
        return;
      }

      setStage("error");
      setErrorMessage(recognitionErrorMessageRef.current ?? "没有检测到语音内容，请再试一次。");
    };

    try {
      recognition.start();
      return true;
    } catch (error) {
      console.warn("dashboard speech recognition start failed", error);
      recognitionRef.current = null;
      recognitionStopReasonRef.current = "none";
      recognitionSessionIdRef.current += 1;
      setStage("error");
      setErrorMessage("语音转写启动失败，请重新打开再试。");
      return false;
    }
  }, [clearCompletionTimer, disposeVoiceRecognition, finalizeVoiceRecognition, setLiveTranscript]);

  const handleClose = useCallback(() => {
    disposeVoiceRecognition();
    resetVoiceField();
    onClose();
  }, [disposeVoiceRecognition, onClose, resetVoiceField]);

  const handleRestart = useCallback(() => {
    resetVoiceField();
    startVoiceRecognition();
  }, [resetVoiceField, startVoiceRecognition]);

  const handleSubmitCurrentTranscript = useCallback(() => {
    if (hasTranscript) {
      void finalizeVoiceRecognition("finish");
      return;
    }

    handleRestart();
  }, [finalizeVoiceRecognition, handleRestart, hasTranscript]);

  const handleSuggestionSelect = useCallback((sequence: DashboardVoiceSequence) => {
    disposeVoiceRecognition();
    clearCompletionTimer();
    recognitionErrorMessageRef.current = null;
    if (sequence.recommendationId) {
      onRecommendationConfirm?.(sequence.recommendationId);
    }
    void submitDashboardVoiceText(sequence.suggestion);
  }, [clearCompletionTimer, disposeVoiceRecognition, onRecommendationConfirm, submitDashboardVoiceText]);

  useEffect(() => {
    if (!isOpen) {
      disposeVoiceRecognition();
      resetVoiceField();
      return;
    }

    resetVoiceField();
    startVoiceRecognition();

    return () => {
      clearCompletionTimer();
      disposeVoiceRecognition();
    };
  }, [clearCompletionTimer, disposeVoiceRecognition, isOpen, resetVoiceField, startVoiceRecognition]);

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

  function handlePanelKeyDown(event: React.KeyboardEvent<HTMLDivElement>) {
    if (event.key === "Escape") {
      event.preventDefault();
      handleClose();
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
        onClick={handleClose}
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
            <Button className="dashboard-orbit-panel__close" onClick={handleClose} ref={closeButtonRef} size="icon-sm" variant="ghost">
              <X className="h-4 w-4" />
              <span className="sr-only">关闭语音场</span>
            </Button>
          </div>

          <div className="dashboard-voice-field__stage-row">
            {stageOrder.map((item, index) => (
              <span key={item} className="dashboard-voice-field__stage-dot" data-active={index <= activeStageIndex ? "true" : "false"} />
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
            <p className="dashboard-voice-field__status" data-stage={stage}>
              {stage === "submitting" ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Mic className="h-4 w-4" />}
              {getDashboardVoiceStatusLabel(stage, submittedTaskStatus)}
            </p>
            <p className="dashboard-voice-field__subline" id={descriptionId}>
              {stage === "submitting"
                ? "我正在把最终转写通过 agent.input.submit 送入任务入口。"
                : stage === "completed"
                  ? submittedTaskStatus === "waiting_auth"
                    ? (submittedMessage ?? "语音内容已经进入正式任务链路，接下来会打开安全页继续处理授权。")
                    : submittedTaskId
                      ? (submittedMessage ?? `语音内容已经进入任务 ${submittedTaskId}，接下来会打开任务详情继续承接。`)
                      : "语音内容已经进入正式任务链路，接下来会打开任务页继续承接。"
                  : stage === "error"
                    ? errorMessage ?? "这次语音没有形成可提交内容。"
                    : "直接说出你的想法，停顿结束后会自动提交；也可以手动结束收音。"}
            </p>

            <div aria-live="polite" className="dashboard-voice-field__transcript-shell">
              <p className="dashboard-voice-field__transcript-eyebrow">live transcript</p>
              <p className="dashboard-voice-field__transcript" data-empty={hasTranscript ? "false" : "true"}>
                {hasTranscript ? transcript : "……正在等待你的语音内容"}
              </p>
            </div>
          </div>

          {stage === "submitting" ? (
            <div className="dashboard-voice-field__progress-bar">
              <motion.div animate={{ width: "100%" }} className="dashboard-voice-field__progress-fill" initial={{ width: "18%" }} transition={{ duration: 0.34, ease: [0.22, 1, 0.36, 1] }} />
            </div>
          ) : null}

          {stage !== "submitting" && stage !== "completed" ? (
            <>
              <div className="dashboard-voice-field__actions">
                <Button className="dashboard-orbit-panel__primary-button" onClick={stage === "listening" ? () => stopVoiceRecognition("finish") : handleSubmitCurrentTranscript}>
                  {stage === "listening" ? "结束并提交" : hasTranscript ? "提交当前转写" : "重新开始"}
                  <Sparkles className="h-4 w-4" />
                </Button>
                <Button className="dashboard-orbit-panel__action-button dashboard-orbit-panel__action-button--mute" onClick={handleRestart} variant="ghost">
                  <RotateCcw className="h-4 w-4" />
                  重新开始
                </Button>
                <Button className="dashboard-orbit-panel__action-button dashboard-orbit-panel__action-button--mute" onClick={handleClose} variant="ghost">
                  取消
                </Button>
              </div>

              <div className="dashboard-voice-field__suggestions" role="list" aria-label="语音快捷建议">
                {sequences.length > 0 ? (
                  sequences.map((sequence) => (
                    <button
                      key={sequence.suggestion}
                      className="dashboard-voice-field__suggestion-chip"
                      onClick={() => handleSuggestionSelect(sequence)}
                      type="button"
                    >
                      {sequence.suggestion}
                    </button>
                  ))
                ) : (
                  <span className="dashboard-voice-field__suggestion-chip" data-active="false">
                    暂无可用建议
                  </span>
                )}
              </div>
            </>
          ) : null}
        </motion.div>
      </motion.div>
    </AnimatePresence>
  );
}
