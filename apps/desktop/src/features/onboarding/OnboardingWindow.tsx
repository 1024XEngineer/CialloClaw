import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { Button, Heading, Text } from "@radix-ui/themes";
import { cn } from "@/utils/cn";
import { desktopOnboardingEvents } from "./onboarding.events";
import {
  advanceDesktopOnboarding,
  completeDesktopOnboarding,
  type DesktopOnboardingPresentation,
  type DesktopOnboardingSession,
  skipDesktopOnboarding,
  requestDesktopOnboardingAction,
} from "./onboardingService";
import { useDesktopOnboardingPresentation } from "./useDesktopOnboardingPresentation";
import { useDesktopOnboardingSession } from "./useDesktopOnboardingSession";
import "./onboarding.css";

function getOnboardingCopy(step: string) {
  switch (step) {
    case "welcome":
      return {
        body: "这是一段约 2 分钟的轻量引导。你可以随时跳过，之后也能在控制面板重新查看。",
        primaryLabel: "开始引导",
        secondaryLabel: null,
        stepLabel: null,
        title: "欢迎来到 CialloClaw",
      };
    case "shell_ball_intro":
      return {
        body: "这是桌面主入口。大多数核心操作都会从悬浮球开始。",
        primaryLabel: "下一步",
        secondaryLabel: "上一步",
        stepLabel: "第 1 步 / 6",
        title: "先认识悬浮球",
      };
    case "shell_ball_hold_voice":
      return {
        body: "请长按悬浮球试试语音。松开后结束输入；如果现在不想试，也可以直接继续。",
        primaryLabel: "下一步",
        secondaryLabel: "上一步",
        stepLabel: "第 2 步 / 6",
        title: "长按试试语音",
      };
    case "shell_ball_double_click":
      return {
        body: "请双击悬浮球打开主界面。检测到双击后会自动进入下一步。",
        primaryLabel: "下一步",
        secondaryLabel: "上一步",
        stepLabel: "第 3 步 / 6",
        title: "双击打开主界面",
      };
    case "dashboard_overview":
      return {
        body: "界面包含 4 个子页面，你可以从这里快速切换；中心的球长按可触发语音；当前也支持同时按下 Ctrl 和数字键 1/2/3/4/5 快速跳页。",
        primaryLabel: "下一步",
        secondaryLabel: "上一步",
        stepLabel: "第 4 步 / 6",
        title: "主界面可以快速切换",
      };
    case "tray_hint":
      return {
        body: "更多设置在系统托盘里。右键托盘图标可以打开控制面板；你也可以直接从这里继续。",
        primaryLabel: "打开控制面板",
        secondaryLabel: "上一步",
        stepLabel: "第 5 步 / 6",
        title: "更完整的设置入口在托盘",
      };
    case "control_panel_api_key":
      return {
        body: "首次使用建议先完成 API Key 配置。填写后点击“保存设置”，产品就会进入可用状态。",
        primaryLabel: "稍后配置",
        secondaryLabel: "上一步",
        stepLabel: "第 6 步 / 6",
        title: "完成 API Key 配置",
      };
    case "done":
      return {
        body: "你已经学会了最基本的使用方式。之后可以在控制面板末尾重新查看这份引导。",
        primaryLabel: "开始使用",
        secondaryLabel: null,
        stepLabel: "已完成",
        title: "准备好了",
      };
    default:
      return null;
  }
}

/**
 * Renders the dedicated onboarding window. The window only displays step copy,
 * highlights, and flow buttons; all real interactions still happen in the
 * underlying business windows.
 *
 * @returns The onboarding card window contents.
 */
export function OnboardingWindow() {
  const cardRef = useRef<HTMLElement | null>(null);
  const session = useDesktopOnboardingSession();
  const presentation = useDesktopOnboardingPresentation();
  const [stagedPresentation, setStagedPresentation] = useState<DesktopOnboardingPresentation | null>(null);
  const currentWindowRef = useRef(getCurrentWindow());
  const cardReadyRef = useRef(false);
  const copy = useMemo(() => (session ? getOnboardingCopy(session.step) : null), [session]);

  useEffect(() => {
    const currentWindow = currentWindowRef.current;
    let disposed = false;
    let disposeReadyRequestListener: (() => void) | null = null;

    const announceReady = () => {
      if (disposed) {
        return;
      }

      void currentWindow.emit(desktopOnboardingEvents.ready);
    };

    announceReady();

    void currentWindow
      .listen(desktopOnboardingEvents.readyRequested, () => {
        announceReady();
      })
      .then((unlisten) => {
        if (disposed) {
          unlisten();
          return;
        }

        disposeReadyRequestListener = unlisten;
      });

    return () => {
      disposed = true;
      disposeReadyRequestListener?.();
    };
  }, []);

  useEffect(() => {
    if (presentation !== null) {
      setStagedPresentation(presentation);
      return;
    }

    if (session === null) {
      setStagedPresentation(null);
    }
  }, [presentation, session]);

  const activePresentation = useMemo(() => {
    if (session === null) {
      return null;
    }

    if (presentation?.step === session.step) {
      return presentation;
    }

    if (stagedPresentation?.step === session.step) {
      return stagedPresentation;
    }

    if (stagedPresentation !== null) {
      return {
        ...stagedPresentation,
        highlights: session.step === "welcome" || session.step === "done" ? [] : stagedPresentation.highlights,
        placement: session.step === "welcome" || session.step === "done" ? "center" : stagedPresentation.placement,
        step: session.step,
      } satisfies DesktopOnboardingPresentation;
    }

    return null;
  }, [presentation, session, stagedPresentation]);

  useLayoutEffect(() => {
    if (!cardRef.current || !session || !activePresentation) {
      cardReadyRef.current = false;
      return;
    }

    let disposed = false;
    let disposeCardReadyRequestListener: (() => void) | null = null;

    const announceCardReady = () => {
      if (disposed || !cardRef.current) {
        return;
      }

      cardReadyRef.current = true;
      void currentWindowRef.current.emit(desktopOnboardingEvents.cardReady);
    };

    announceCardReady();

    void currentWindowRef.current
      .listen(desktopOnboardingEvents.cardReadyRequested, () => {
        if (!cardReadyRef.current) {
          return;
        }

        announceCardReady();
      })
      .then((unlisten) => {
        if (disposed) {
          unlisten();
          return;
        }

        disposeCardReadyRequestListener = unlisten;
      });

    return () => {
      disposed = true;
      cardReadyRef.current = false;
      disposeCardReadyRequestListener?.();
    };
  }, [activePresentation, session]);

  if (!session || !copy || !activePresentation) {
    return <main className="desktop-onboarding-window" />;
  }

  const advanceToStep = (step: DesktopOnboardingSession["step"]) => {
    void advanceDesktopOnboarding(step);
  };

  const handlePrimary = () => {
    switch (session.step) {
      case "welcome":
        advanceToStep("shell_ball_intro");
        return;
      case "shell_ball_intro":
        advanceToStep("shell_ball_hold_voice");
        return;
      case "shell_ball_hold_voice":
        advanceToStep("shell_ball_double_click");
        return;
      case "shell_ball_double_click":
        void requestDesktopOnboardingAction({
          targetWindow: "shell-ball",
          type: "open_dashboard",
        });
        advanceToStep("dashboard_overview");
        return;
      case "dashboard_overview":
        void requestDesktopOnboardingAction({
          targetWindow: "dashboard",
          type: "close_dashboard",
        });
        advanceToStep("tray_hint");
        return;
      case "tray_hint":
        void requestDesktopOnboardingAction({
          targetWindow: "dashboard",
          type: "close_dashboard",
        });
        void requestDesktopOnboardingAction({
          targetWindow: "dashboard",
          type: "open_control_panel",
        });
        advanceToStep("control_panel_api_key");
        return;
      case "control_panel_api_key":
        advanceToStep("done");
        return;
      case "done":
        void completeDesktopOnboarding();
        return;
      default:
        return;
    }
  };

  const handleSecondary = () => {
    switch (session.step) {
      case "welcome":
        void skipDesktopOnboarding();
        return;
      case "shell_ball_intro":
        advanceToStep("welcome");
        return;
      case "shell_ball_hold_voice":
        advanceToStep("shell_ball_intro");
        return;
      case "shell_ball_double_click":
        advanceToStep("shell_ball_hold_voice");
        return;
      case "dashboard_overview":
        void requestDesktopOnboardingAction({
          targetWindow: "dashboard",
          type: "close_dashboard",
        });
        void requestDesktopOnboardingAction({
          targetWindow: "shell-ball",
          type: "show_shell_ball",
        });
        advanceToStep("shell_ball_double_click");
        return;
      case "tray_hint":
        advanceToStep("dashboard_overview");
        return;
      case "control_panel_api_key":
        void requestDesktopOnboardingAction({
          targetWindow: "control-panel",
          type: "close_control_panel",
        });
        void requestDesktopOnboardingAction({
          targetWindow: "dashboard",
          type: "open_dashboard",
        });
        advanceToStep("tray_hint");
        return;
      default:
        return;
      }
  };

  return (
    <main className="desktop-onboarding-window">
      <section
        key={`${activePresentation.step}-${activePresentation.placement}`}
        ref={cardRef}
        className={cn("desktop-onboarding-window__card", `desktop-onboarding-window__card--${activePresentation.placement}`)}
        aria-label={copy.title}
      >
        {copy.stepLabel ? (
          <Text as="p" size="1" className="desktop-onboarding-window__step-label">
            {copy.stepLabel}
          </Text>
        ) : null}
        <Heading size={session.step === "welcome" ? "7" : "5"} className="desktop-onboarding-window__title">
          {copy.title}
        </Heading>
        <Text as="p" size="2" className="desktop-onboarding-window__body">
          {copy.body}
        </Text>
        <div className="desktop-onboarding-window__actions">
          {copy.secondaryLabel ? (
            <Button className="desktop-onboarding-window__button desktop-onboarding-window__button--secondary" variant="soft" onClick={handleSecondary}>
              {copy.secondaryLabel}
            </Button>
          ) : null}
          {session.step !== "done" ? (
            <Button
              className="desktop-onboarding-window__button desktop-onboarding-window__button--ghost"
              variant="soft"
              color="gray"
              onClick={() => {
                void skipDesktopOnboarding();
              }}
            >
              结束引导
            </Button>
          ) : null}
          <Button className="desktop-onboarding-window__button desktop-onboarding-window__button--primary" onClick={handlePrimary}>
            {copy.primaryLabel}
          </Button>
        </div>
      </section>
    </main>
  );
}
