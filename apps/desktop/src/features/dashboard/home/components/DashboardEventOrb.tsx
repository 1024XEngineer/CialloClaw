import { useEffect, useState, useRef, useCallback } from "react";
import type { DashboardHomeStateData, DashboardHomeSummonEvent } from "../dashboardHome.types";
import { BrainCircuit, FileText, NotebookPen, ShieldAlert, X } from "lucide-react";

type DashboardEventOrbProps = {
  event: DashboardHomeSummonEvent;
  stateMap: Record<string, DashboardHomeStateData>;
  onDismiss: (id: string) => void;
  onExpand: (event: DashboardHomeSummonEvent) => void;
};

type Phase = "dormant" | "emerging" | "present" | "receding" | "gone";

const SUMMON_RADIUS = 155;
const DORMANT_RADIUS = 260;

const icons = {
  notes: NotebookPen,
  memory: BrainCircuit,
  safety: ShieldAlert,
  tasks: FileText,
} as const;

export function DashboardEventOrb({ event, stateMap, onDismiss, onExpand }: DashboardEventOrbProps) {
  const stateData = stateMap[event.stateKey];
  const [phase, setPhase] = useState<Phase>("dormant");
  const [textVisible, setTextVisible] = useState(false);
  const [hovered, setHovered] = useState(false);
  const [orbitAngle, setOrbitAngle] = useState(0);
  const [dismissed, setDismissed] = useState(false);
  const [dragOffset, setDragOffset] = useState({ x: 0, y: 0 });
  const [isDragging, setIsDragging] = useState(false);
  const isDraggingRef = useRef(false);
  const dragStartRef = useRef({ mouseX: 0, mouseY: 0, offX: 0, offY: 0 });
  const animRef = useRef<number>(0);
  const startAngleRef = useRef(Math.random() * 360);
  const phaseRef = useRef<Phase>("dormant");
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const enterTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const settleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const recedeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const interactionRef = useRef(false);

  const { duration = 5200 } = event;
  const Icon = icons[stateData.module];
  useEffect(() => {
    const speed = 1.8;
    let last = 0;

    const animate = (timestamp: number) => {
      const dt = last ? (timestamp - last) / 1000 : 0;
      last = timestamp;

      if (phaseRef.current === "present" || phaseRef.current === "emerging") {
        startAngleRef.current = (startAngleRef.current + speed * dt) % 360;
        setOrbitAngle(startAngleRef.current);
      }

      animRef.current = requestAnimationFrame(animate);
    };

    animRef.current = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(animRef.current);
  }, []);

  const clearDismissTimer = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  const clearPhaseTimers = useCallback(() => {
    if (enterTimerRef.current) {
      clearTimeout(enterTimerRef.current);
      enterTimerRef.current = null;
    }
    if (settleTimerRef.current) {
      clearTimeout(settleTimerRef.current);
      settleTimerRef.current = null;
    }
    if (recedeTimerRef.current) {
      clearTimeout(recedeTimerRef.current);
      recedeTimerRef.current = null;
    }
    clearDismissTimer();
  }, [clearDismissTimer]);

  const startRecede = useCallback(() => {
    if (phaseRef.current === "receding" || phaseRef.current === "gone") {
      return;
    }

    clearPhaseTimers();
    phaseRef.current = "receding";
    setPhase("receding");
    setTextVisible(false);

    recedeTimerRef.current = setTimeout(() => {
      phaseRef.current = "gone";
      setPhase("gone");
      onDismiss(event.id);
    }, 800);
  }, [clearPhaseTimers, event.id, onDismiss]);

  const scheduleDismiss = useCallback(
    (nextDuration: number) => {
      clearDismissTimer();
      timerRef.current = setTimeout(() => {
        if (!interactionRef.current) {
          startRecede();
        }
      }, nextDuration);
    },
    [clearDismissTimer, startRecede],
  );

  useEffect(() => {
    enterTimerRef.current = setTimeout(() => {
      phaseRef.current = "emerging";
      setPhase("emerging");

      settleTimerRef.current = setTimeout(() => {
        setTextVisible(true);
        phaseRef.current = "present";
        setPhase("present");
        scheduleDismiss(duration);
      }, 700);
    }, 80);

    return clearPhaseTimers;
  }, [clearPhaseTimers, duration, scheduleDismiss]);

  useEffect(() => {
    const handleMove = (nativeEvent: MouseEvent) => {
      if (!isDraggingRef.current) {
        return;
      }

      const dx = nativeEvent.clientX - dragStartRef.current.mouseX;
      const dy = nativeEvent.clientY - dragStartRef.current.mouseY;
      setDragOffset({ x: dragStartRef.current.offX + dx, y: dragStartRef.current.offY + dy });
    };

    const handleUp = () => {
      if (!isDraggingRef.current) {
        return;
      }

      isDraggingRef.current = false;
      setIsDragging(false);
    };

    window.addEventListener("mousemove", handleMove);
    window.addEventListener("mouseup", handleUp);
    return () => {
      window.removeEventListener("mousemove", handleMove);
      window.removeEventListener("mouseup", handleUp);
    };
  }, []);

  const handleMouseDown = useCallback(
    (eventObject: React.MouseEvent<HTMLDivElement>) => {
      eventObject.preventDefault();
      eventObject.stopPropagation();
      isDraggingRef.current = true;
      setIsDragging(true);
      dragStartRef.current = {
        mouseX: eventObject.clientX,
        mouseY: eventObject.clientY,
        offX: dragOffset.x,
        offY: dragOffset.y,
      };

      if (timerRef.current) {
        clearDismissTimer();
      }
    },
    [clearDismissTimer, dragOffset],
  );

  const handleMouseUp = useCallback(() => {
    if (isDragging) {
      return;
    }

    if (phase === "present" && !interactionRef.current) {
      scheduleDismiss(2500);
    }
  }, [isDragging, phase, scheduleDismiss]);

  const setInteracting = useCallback(
    (nextInteracting: boolean) => {
      interactionRef.current = nextInteracting;

      if (nextInteracting) {
        clearDismissTimer();
        return;
      }

      if (phaseRef.current === "present" && !isDraggingRef.current) {
        scheduleDismiss(1800);
      }
    },
    [clearDismissTimer, scheduleDismiss],
  );

  const handleMouseEnter = useCallback(() => {
    setHovered(true);
    setInteracting(true);
  }, [setInteracting]);

  const handleMouseLeave = useCallback(() => {
    setHovered(false);
    setInteracting(false);
  }, [setInteracting]);

  const handleFocus = useCallback(() => {
    setInteracting(true);
  }, [setInteracting]);

  const handleBlur = useCallback(() => {
    setInteracting(false);
  }, [setInteracting]);

  const handleDismiss = useCallback(
    (eventObject: React.MouseEvent<HTMLButtonElement>) => {
      eventObject.stopPropagation();
      setDismissed(true);
      startRecede();
    },
    [startRecede],
  );

  const handleClick = useCallback(() => {
    if (isDragging) {
      return;
    }

    onExpand(event);
    startRecede();
  }, [event, isDragging, onExpand, startRecede]);

  if (phase === "gone") {
    return null;
  }

  const rad = (orbitAngle * Math.PI) / 180;
  const currentRadius =
    phase === "dormant"
      ? DORMANT_RADIUS
      : phase === "emerging"
        ? SUMMON_RADIUS + (DORMANT_RADIUS - SUMMON_RADIUS) * 0.15
        : phase === "present"
          ? SUMMON_RADIUS
          : DORMANT_RADIUS * 0.85;

  const x = Math.cos(rad) * currentRadius + dragOffset.x;
  const y = Math.sin(rad) * currentRadius + dragOffset.y;

  const opacity = phase === "dormant" ? 0 : phase === "emerging" ? 0.92 : phase === "present" ? 1 : 0;
  const orbScale = phase === "emerging" ? 1.05 : phase === "present" ? (hovered || isDragging ? 1.18 : 1.08) : 0.85;
  const priorityColors = {
    urgent: { dot: "#fb7185", label: "URGENT", ring: "#fb7185" },
    normal: { dot: stateData.accentColor, label: "AGENT", ring: stateData.accentColor },
    low: { dot: `${stateData.accentColor}80`, label: "", ring: `${stateData.accentColor}80` },
  } as const;
  const priorityColor = priorityColors[event.priority];

  return (
    <div
      className="absolute"
      onClick={handleClick}
      onKeyDown={(eventObject) => {
        if (eventObject.key === "Enter" || eventObject.key === " ") {
          eventObject.preventDefault();
          handleClick();
        }
      }}
      onMouseDown={handleMouseDown}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
      onMouseUp={handleMouseUp}
      onFocus={handleFocus}
      onBlur={handleBlur}
      role="button"
      tabIndex={0}
      style={{
        cursor: isDragging ? "grabbing" : "pointer",
        left: "50%",
        opacity,
        pointerEvents: phase === "dormant" ? "none" : "auto",
        top: "50%",
        transform: `translate(calc(-50% + ${x}px), calc(-50% + ${y}px))`,
        transition: isDragging
          ? "opacity 0.15s ease"
          : phase === "emerging"
            ? "transform 0.7s cubic-bezier(0.16,1,0.3,1), opacity 0.5s ease"
            : phase === "receding"
              ? "transform 0.8s cubic-bezier(0.4,0,1,1), opacity 0.6s ease"
              : "opacity 0.3s ease",
        zIndex: isDragging ? 50 : 25,
      }}
    >
      <div
        className="absolute rounded-full pointer-events-none"
        style={{
          width: 88,
          height: 88,
          left: "50%",
          top: "50%",
          transform: "translate(-50%, -50%)",
          background: `radial-gradient(circle, ${stateData.orbGlow} 0%, transparent 60%)`,
          opacity: phase === "present" ? (hovered || isDragging ? 1 : 0.75) : 0.4,
          transition: "opacity 0.4s ease",
        }}
      />

      {phase === "present" ? (
        <div
          className="absolute rounded-full pointer-events-none"
          style={{
            width: 54,
            height: 54,
            left: "50%",
            top: "50%",
            transform: "translate(-50%, -50%)",
            border: `1px solid ${priorityColor.ring}45`,
            animation: "summonRingPulse 2.2s ease-in-out infinite",
          }}
        />
      ) : null}

      {phase === "present" && !isDragging && dragOffset.x === 0 && dragOffset.y === 0 ? (
        <div
          className="absolute pointer-events-none"
          style={{
            left: "50%",
            top: "50%",
            width: currentRadius,
            height: 1,
            transformOrigin: "0 50%",
            transform: `rotate(${orbitAngle + 180}deg)`,
            background: `linear-gradient(to left, ${stateData.accentColor}25, transparent)`,
            opacity: hovered ? 0.6 : 0.25,
            transition: "opacity 0.3s ease",
          }}
        />
      ) : null}

      {isDragging ? (
        <div
          className="absolute rounded-full pointer-events-none"
          style={{
            width: 66,
            height: 66,
            left: "50%",
            top: "50%",
            transform: "translate(-50%, -50%)",
            border: `1px dashed ${stateData.accentColor}40`,
            animation: "dragSpin 3s linear infinite",
          }}
        />
      ) : null}

      <div
        className="relative rounded-full flex items-center justify-center"
        style={{
          width: 38,
          height: 38,
          transform: `scale(${orbScale})`,
          background: `radial-gradient(circle at 35% 30%, ${stateData.orbColor}dd 0%, ${stateData.orbColor}55 50%, ${stateData.orbColor}22 100%)`,
          border: `1.5px solid ${stateData.orbColor}${hovered || isDragging ? "80" : "55"}`,
          boxShadow: `
            0 0 ${hovered || isDragging ? 50 : 32}px ${stateData.orbGlow},
            0 0 ${hovered || isDragging ? 18 : 10}px ${stateData.orbGlow} inset,
            0 0 ${hovered || isDragging ? 80 : 55}px ${stateData.orbGlow}
          `,
          transition: isDragging ? "transform 0.05s ease" : "transform 0.35s cubic-bezier(0.34,1.56,0.64,1), box-shadow 0.3s ease",
        }}
      >
        <div
          className="absolute inset-0 rounded-full pointer-events-none"
          style={{
            background: "radial-gradient(circle at 30% 25%, rgba(255,255,255,0.28) 0%, transparent 55%)",
          }}
        />

        <Icon
          style={{
            fontSize: 15,
            color: "rgba(255,255,255,0.95)",
            position: "relative",
            zIndex: 1,
          }}
        />

        {event.priority === "urgent" ? (
          <div
            className="absolute rounded-full"
            style={{
              width: 9,
              height: 9,
              background: "#fb7185",
              border: "2px solid rgba(255,250,244,0.82)",
              top: 2,
              right: 2,
              boxShadow: "0 0 8px #fb7185",
              animation: "notifPulse 1.2s ease-in-out infinite",
            }}
          />
        ) : null}
      </div>

      {isDragging ? (
        <div
          className="absolute whitespace-nowrap pointer-events-none"
          style={{
            top: "100%",
            left: "50%",
            transform: "translateX(-50%)",
            marginTop: 8,
            fontSize: 9,
            color: `${stateData.accentColor}99`,
            letterSpacing: "0.12em",
            animation: "fadeInUp 0.2s ease",
          }}
        >
          松手固定位置
        </div>
      ) : null}

      <div
        style={{
          position: "absolute",
          left: "50%",
          top: "100%",
          transform: textVisible ? "translateX(-50%) translateY(0px)" : "translateX(-50%) translateY(6px)",
          marginTop: 10,
          width: 200,
          opacity: textVisible && !isDragging ? 1 : 0,
          transition: "opacity 0.45s cubic-bezier(0.16,1,0.3,1), transform 0.45s cubic-bezier(0.16,1,0.3,1)",
          pointerEvents: "none",
        }}
      >
        <div
          style={{
            background: "rgba(255,250,244,0.92)",
            border: `1px solid ${stateData.accentColor}30`,
            borderRadius: 12,
            padding: "8px 11px",
            backdropFilter: "blur(16px)",
            boxShadow: `0 8px 32px rgba(168,145,120,0.16), 0 0 20px ${stateData.orbGlow}`,
          }}
        >
          {priorityColor.label ? (
            <div className="flex items-center gap-1.5 mb-1.5">
              <div
                className="rounded-full"
                style={{
                  width: 4,
                  height: 4,
                  background: priorityColor.dot,
                  boxShadow: `0 0 5px ${priorityColor.dot}`,
                  animation: event.priority === "urgent" ? "notifPulse 1.2s ease-in-out infinite" : "notifPulse 2s ease-in-out infinite",
                }}
              />
              <span style={{ fontSize: 7.5, color: priorityColor.dot, letterSpacing: "0.2em", fontWeight: 600 }}>{priorityColor.label}</span>
            </div>
          ) : null}

          <div
            style={{
              fontSize: 11,
              color: "rgba(95,84,75,0.88)",
              lineHeight: 1.5,
              letterSpacing: "0.02em",
              marginBottom: 4,
              fontWeight: 500,
            }}
          >
            {event.message}
          </div>

          <div
            style={{
              fontSize: 9.5,
              color: "rgba(95,84,75,0.58)",
              lineHeight: 1.4,
              letterSpacing: "0.02em",
              marginBottom: event.nextStep ? 6 : 0,
            }}
          >
            {event.reason}
          </div>

          {event.nextStep ? (
            <div className="flex items-center gap-1.5 pt-1.5" style={{ borderTop: `1px solid ${stateData.accentColor}15` }}>
              <span style={{ fontSize: 9, color: stateData.accentColor, letterSpacing: "0.06em" }}>{event.nextStep}</span>
            </div>
          ) : null}
        </div>
        <div className="flex justify-center" style={{ marginTop: -1 }}>
          <div style={{ width: 1, height: 8, background: `linear-gradient(to bottom, ${stateData.accentColor}40, transparent)` }} />
        </div>
      </div>

      {hovered && !dismissed && !isDragging ? (
        <button
          className="absolute flex items-center justify-center rounded-full cursor-pointer"
          onClick={handleDismiss}
          onMouseDown={(eventObject) => eventObject.stopPropagation()}
          style={{
            width: 18,
            height: 18,
            top: -4,
            right: -4,
            background: "rgba(255,250,244,0.95)",
            border: "1px solid rgba(156,133,113,0.22)",
            color: "rgba(95,84,75,0.58)",
            zIndex: 10,
            animation: "fadeInScale 0.15s ease",
          }}
          type="button"
        >
          <X className="h-3 w-3" />
        </button>
      ) : null}

      <style>{`
        @keyframes summonRingPulse {
          0%, 100% { opacity: 0.5; transform: translate(-50%, -50%) scale(1); }
          50% { opacity: 0.15; transform: translate(-50%, -50%) scale(1.1); }
        }
        @keyframes fadeInScale {
          from { opacity: 0; transform: scale(0.7); }
          to { opacity: 1; transform: scale(1); }
        }
        @keyframes dragSpin {
          from { transform: translate(-50%, -50%) rotate(0deg); }
          to { transform: translate(-50%, -50%) rotate(360deg); }
        }
        @keyframes fadeInUp {
          from { opacity: 0; transform: translateX(-50%) translateY(4px); }
          to { opacity: 1; transform: translateX(-50%) translateY(0); }
        }
      `}</style>
    </div>
  );
}
