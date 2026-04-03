import { useEffect, useState, useRef, useCallback } from 'react';
import type { ModuleKey } from '@/mocks/agentStates';

interface VoiceInterfaceProps {
  isOpen: boolean;
  onClose: () => void;
  onCommand: (command: string, module?: ModuleKey) => void;
  orbColor: string;
  orbGlow: string;
}

type VoiceStage = 'ready' | 'listening' | 'understanding' | 'confirming' | 'executing';

interface IntentFragment {
  id: number;
  text: string;
}

interface EchoWord {
  id: number;
  text: string;
  x: number;
  y: number;
  opacity: number;
  scale: number;
  born: number;
}

interface ConfirmAction {
  label: string;
  primary?: boolean;
  module?: ModuleKey;
  cancel?: boolean;
}

interface Particle {
  id: number;
  x: number;
  y: number;
  radius: number;
  opacity: number;
  speed: number;
  size: number;
  angle: number;
}

const VOICE_SUGGESTIONS = [
  '帮我看今天的重点任务',
  '总结我刚才说的内容',
  '我现在还有什么没做完',
  '帮我记一下这个想法',
  '最近系统状态怎么样',
];

// Echo words that drift up during listening — simulates real-time keyword capture
const ECHO_WORD_POOLS = [
  ['今天', '重点', '任务', '截止', '周报', '整理'],
  ['便签', '记录', '想法', '备忘', '提醒'],
  ['系统', '状态', '硬件', '感知', '负载'],
  ['总结', '分析', '洞察', '本周', '模式'],
];

const RECOGNITION_SEQUENCES = [
  {
    echoPool: ECHO_WORD_POOLS[0],
    fragments: ['查看今天重点任务', '整理周报草稿'],
    summary: '我会先看今天的重点任务，再整理周报草稿',
    executingSteps: ['正在读取任务列表…', '发现 3 项今日任务', '周报草稿已定位', '准备就绪'],
    actions: [
      { label: '开始执行', primary: true, module: 'task' as ModuleKey },
      { label: '说得更详细一点' },
      { label: '取消', cancel: true },
    ] as ConfirmAction[],
  },
  {
    echoPool: ECHO_WORD_POOLS[1],
    fragments: ['记录便签', '设置提醒'],
    summary: '我来帮你记录这个想法，并设置提醒',
    executingSteps: ['正在打开便签…', '已创建新便签', '提醒已设置', '完成'],
    actions: [
      { label: '开始记录', primary: true, module: 'notepad' as ModuleKey },
      { label: '取消', cancel: true },
    ] as ConfirmAction[],
  },
  {
    echoPool: ECHO_WORD_POOLS[2],
    fragments: ['查看系统状态'],
    summary: '我来看看当前系统和硬件运行状态',
    executingSteps: ['正在采集系统数据…', 'CPU / 内存已读取', '网络状态正常', '报告生成完毕'],
    actions: [
      { label: '查看详情', primary: true, module: 'sense' as ModuleKey },
      { label: '取消', cancel: true },
    ] as ConfirmAction[],
  },
];

export default function VoiceInterface({
  isOpen,
  onClose,
  onCommand,
  orbColor,
  orbGlow,
}: VoiceInterfaceProps) {
  const [stage, setStage] = useState<VoiceStage>('ready');
  const [audioLevel, setAudioLevel] = useState(0);
  const [intentFragments, setIntentFragments] = useState<IntentFragment[]>([]);
  const [confirmSummary, setConfirmSummary] = useState('');
  const [confirmActions, setConfirmActions] = useState<ConfirmAction[]>([]);
  const [particles, setParticles] = useState<Particle[]>([]);
  const [wavePhase, setWavePhase] = useState(0);
  const [mounted, setMounted] = useState(false);
  const [closing, setClosing] = useState(false);
  const [suggestionVisible, setSuggestionVisible] = useState(false);

  // Echo words — drift up during listening
  const [echoWords, setEchoWords] = useState<EchoWord[]>([]);

  // Executing stage
  const [executingStep, setExecutingStep] = useState('');
  const [executingProgress, setExecutingProgress] = useState(0);
  const [executingDone, setExecutingDone] = useState(false);
  const [executingModule, setExecutingModule] = useState<ModuleKey | undefined>(undefined);

  const animRef = useRef<number>(0);
  const stageTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const particleIdRef = useRef(0);
  const echoIdRef = useRef(0);
  const seqRef = useRef(0);
  const echoTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const clearTimer = useCallback(() => {
    if (stageTimerRef.current) clearTimeout(stageTimerRef.current);
  }, []);

  // Mount / unmount
  useEffect(() => {
    if (isOpen) {
      setStage('ready');
      setIntentFragments([]);
      setConfirmSummary('');
      setConfirmActions([]);
      setEchoWords([]);
      setExecutingStep('');
      setExecutingProgress(0);
      setExecutingDone(false);
      setExecutingModule(undefined);
      setClosing(false);
      setMounted(false);
      setSuggestionVisible(false);
      const t1 = setTimeout(() => setMounted(true), 30);
      const t2 = setTimeout(() => setSuggestionVisible(true), 700);
      return () => { clearTimeout(t1); clearTimeout(t2); };
    } else {
      setMounted(false);
    }
  }, [isOpen]);

  // Main animation loop
  useEffect(() => {
    if (!isOpen) return;
    const animate = () => {
      setWavePhase(Date.now() / 1000);
      if (stage === 'listening') {
        setAudioLevel(0.3 + Math.sin(Date.now() / 180) * 0.25 + Math.random() * 0.2);
      } else if (stage === 'understanding') {
        setAudioLevel(0.15 + Math.sin(Date.now() / 300) * 0.08);
      } else {
        setAudioLevel(0.06 + Math.sin(Date.now() / 900) * 0.03);
      }

      // Particles during understanding
      if (stage === 'understanding') {
        setParticles(prev => {
          const moved = prev.map(p => ({
            ...p,
            radius: p.radius - p.speed * 1.4,
            opacity: p.radius < 35 ? p.opacity * 0.88 : p.opacity * 0.992,
          })).filter(p => p.radius > 4 && p.opacity > 0.03);

          if (moved.length < 20 && Math.random() < 0.35) {
            const angle = Math.random() * Math.PI * 2;
            const radius = 110 + Math.random() * 55;
            moved.push({
              id: ++particleIdRef.current,
              x: Math.cos(angle) * radius,
              y: Math.sin(angle) * radius,
              angle,
              radius,
              opacity: 0.5 + Math.random() * 0.35,
              speed: 0.7 + Math.random() * 0.7,
              size: 1.5 + Math.random() * 2,
            });
          }
          return moved;
        });
      }

      // Fade echo words
      setEchoWords(prev =>
        prev
          .map(w => ({
            ...w,
            y: w.y - 0.35,
            opacity: w.opacity - 0.004,
            scale: Math.min(w.scale + 0.008, 1),
          }))
          .filter(w => w.opacity > 0.02)
      );

      animRef.current = requestAnimationFrame(animate);
    };
    animRef.current = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(animRef.current);
  }, [isOpen, stage]);

  // Echo word emitter — fires during listening
  useEffect(() => {
    if (stage !== 'listening') {
      if (echoTimerRef.current) clearInterval(echoTimerRef.current);
      return;
    }
    const seq = RECOGNITION_SEQUENCES[seqRef.current % RECOGNITION_SEQUENCES.length];
    const pool = seq.echoPool;
    let poolIdx = 0;

    echoTimerRef.current = setInterval(() => {
      const word = pool[poolIdx % pool.length];
      poolIdx++;
      // Scatter around the orb area
      const angle = Math.random() * Math.PI * 2;
      const r = 55 + Math.random() * 50;
      setEchoWords(prev => {
        if (prev.length > 12) return prev;
        return [...prev, {
          id: ++echoIdRef.current,
          text: word,
          x: 120 + Math.cos(angle) * r,
          y: 120 + Math.sin(angle) * r,
          opacity: 0.55 + Math.random() * 0.3,
          scale: 0.7,
          born: Date.now(),
        }];
      });
    }, 420);

    return () => {
      if (echoTimerRef.current) clearInterval(echoTimerRef.current);
    };
  }, [stage]);

  // Stage progression
  useEffect(() => {
    if (!isOpen || stage !== 'ready') return;
    clearTimer();
    stageTimerRef.current = setTimeout(() => setStage('listening'), 1000);
    return clearTimer;
  }, [isOpen, stage, clearTimer]);

  useEffect(() => {
    if (stage !== 'listening') return;
    clearTimer();
    const seq = RECOGNITION_SEQUENCES[seqRef.current % RECOGNITION_SEQUENCES.length];

    stageTimerRef.current = setTimeout(() => {
      setStage('understanding');
      setParticles([]);
      setEchoWords([]);

      seq.fragments.forEach((frag, i) => {
        setTimeout(() => {
          setIntentFragments(prev => [...prev, { id: Date.now() + i, text: frag }]);
        }, i * 650);
      });

      const confirmDelay = seq.fragments.length * 650 + 900;
      stageTimerRef.current = setTimeout(() => {
        setStage('confirming');
        setConfirmSummary(seq.summary);
        setConfirmActions(seq.actions);
        seqRef.current += 1;
      }, confirmDelay);
    }, 2800);

    return clearTimer;
  }, [stage, clearTimer]);

  // Executing stage — step through progress
  const runExecuting = useCallback((steps: string[], module?: ModuleKey) => {
    setExecutingModule(module);
    setExecutingProgress(0);
    setExecutingDone(false);

    steps.forEach((step, i) => {
      setTimeout(() => {
        setExecutingStep(step);
        setExecutingProgress(Math.round(((i + 1) / steps.length) * 100));
        if (i === steps.length - 1) {
          setTimeout(() => {
            setExecutingDone(true);
            // After showing done, open the module and close
            setTimeout(() => {
              if (module) onCommand('voice', module);
              handleClose();
            }, 1200);
          }, 600);
        }
      }, i * 700);
    });
  }, [onCommand]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleClose = useCallback(() => {
    setClosing(true);
    clearTimer();
    if (echoTimerRef.current) clearInterval(echoTimerRef.current);
    setTimeout(onClose, 350);
  }, [onClose, clearTimer]);

  const handleAction = useCallback((action: ConfirmAction) => {
    if (action.cancel) { handleClose(); return; }
    if (action.primary) {
      const seq = RECOGNITION_SEQUENCES[(seqRef.current - 1 + RECOGNITION_SEQUENCES.length) % RECOGNITION_SEQUENCES.length];
      setStage('executing');
      setIntentFragments([]);
      runExecuting(seq.executingSteps, action.module);
    } else {
      // "说得更详细一点" — go back to listening
      setStage('listening');
      setIntentFragments([]);
      setConfirmSummary('');
      setConfirmActions([]);
    }
  }, [handleClose, runExecuting]);

  const handleSuggestionClick = useCallback((suggestion: string) => {
    setStage('listening');
    setIntentFragments([]);
    setEchoWords([]);
    clearTimer();
    // Immediately emit the suggestion as an echo word
    setEchoWords([{
      id: ++echoIdRef.current,
      text: suggestion,
      x: 90 + Math.random() * 60,
      y: 100 + Math.random() * 40,
      opacity: 0.8,
      scale: 0.85,
      born: Date.now(),
    }]);
    setTimeout(() => {
      const seq = RECOGNITION_SEQUENCES[seqRef.current % RECOGNITION_SEQUENCES.length];
      setStage('understanding');
      setIntentFragments([{ id: Date.now(), text: suggestion }]);
      setTimeout(() => {
        setStage('confirming');
        setConfirmSummary(seq.summary);
        setConfirmActions(seq.actions);
        seqRef.current += 1;
      }, 1100);
    }, 900);
  }, [clearTimer]);

  if (!isOpen) return null;

  const stageConfig: Record<VoiceStage, { statusLine: string; subLine: string; waveIntensity: number; coreScale: number }> = {
    ready: { statusLine: '直接说出你的想法', subLine: '我在听', waveIntensity: 0.25, coreScale: 1 },
    listening: { statusLine: '正在听…', subLine: '继续说，我会跟上', waveIntensity: 1, coreScale: 1.06 },
    understanding: { statusLine: '我在整理你的意思', subLine: '我大概明白了，让我确认一下', waveIntensity: 0.45, coreScale: 1.02 },
    confirming: { statusLine: '你是想让我…', subLine: confirmSummary, waveIntensity: 0.18, coreScale: 1 },
    executing: { statusLine: '正在处理…', subLine: executingStep, waveIntensity: 0.35, coreScale: 1.04 },
  };
  const cfg = stageConfig[stage];

  // Organic wave rings
  const waveRings = Array.from({ length: 5 }, (_, i) => {
    const baseR = 66 + i * 17;
    const intensity = cfg.waveIntensity;
    const t = wavePhase;
    const distortion = intensity * (7 + i * 3);
    const phase = t * (1.2 - i * 0.15) + i * 0.8;
    const points = Array.from({ length: 64 }, (__, j) => {
      const a = (j / 64) * Math.PI * 2;
      const noise = Math.sin(a * 3 + phase) * distortion * 0.4
        + Math.sin(a * 5 - phase * 1.3) * distortion * 0.25
        + Math.sin(a * 7 + phase * 0.7) * distortion * 0.15
        + (stage === 'listening' ? Math.sin(a * 11 + phase * 2.1) * audioLevel * distortion * 0.35 : 0);
      const r = baseR + noise;
      return `${120 + Math.cos(a) * r},${120 + Math.sin(a) * r}`;
    });
    return { points: points.join(' '), opacity: (0.17 - i * 0.022) * (0.35 + intensity * 0.65) };
  });

  // Executing progress arc
  const progressArc = (() => {
    const r = 52;
    const cx = 120; const cy = 120;
    const angle = (executingProgress / 100) * Math.PI * 2 - Math.PI / 2;
    const x = cx + Math.cos(angle) * r;
    const y = cy + Math.sin(angle) * r;
    const large = executingProgress > 50 ? 1 : 0;
    return executingProgress === 0
      ? ''
      : executingProgress >= 100
      ? `M ${cx} ${cy - r} A ${r} ${r} 0 1 1 ${cx - 0.001} ${cy - r}`
      : `M ${cx} ${cy - r} A ${r} ${r} 0 ${large} 1 ${x} ${y}`;
  })();

  return (
    <div
      className="fixed inset-0 flex items-center justify-center"
      style={{
        zIndex: 200,
        background: `rgba(3,5,12,${closing ? 0 : mounted ? 0.9 : 0})`,
        backdropFilter: 'blur(14px)',
        transition: 'background 0.35s ease',
        pointerEvents: 'auto',
      }}
      onClick={handleClose}
    >
      <div
        className="relative flex flex-col items-center"
        style={{
          opacity: closing ? 0 : mounted ? 1 : 0,
          transform: closing ? 'scale(0.94)' : mounted ? 'scale(1)' : 'scale(0.92)',
          transition: 'opacity 0.35s cubic-bezier(0.16,1,0.3,1), transform 0.35s cubic-bezier(0.16,1,0.3,1)',
        }}
        onClick={e => e.stopPropagation()}
      >

        {/* ── Stage progress bar — top ── */}
        <div
          className="flex items-center gap-2 mb-10"
          style={{ opacity: mounted ? 0.5 : 0, transition: 'opacity 0.6s ease 0.3s' }}
        >
          {(['ready', 'listening', 'understanding', 'confirming', 'executing'] as VoiceStage[]).map((s, i) => (
            <div key={s} className="flex items-center gap-2">
              <div
                className="rounded-full transition-all duration-500"
                style={{
                  width: s === stage ? 22 : 5,
                  height: 5,
                  background: s === stage
                    ? orbColor
                    : (['ready', 'listening', 'understanding', 'confirming', 'executing'] as VoiceStage[]).indexOf(s)
                      < (['ready', 'listening', 'understanding', 'confirming', 'executing'] as VoiceStage[]).indexOf(stage)
                      ? `${orbColor}50`
                      : 'rgba(255,255,255,0.1)',
                  boxShadow: s === stage ? `0 0 8px ${orbColor}` : 'none',
                }}
              />
              {i < 4 && <div style={{ width: 10, height: 1, background: 'rgba(255,255,255,0.06)' }} />}
            </div>
          ))}
        </div>

        {/* ── Core consciousness orb ── */}
        <div className="relative mb-8" style={{ width: 240, height: 240 }}>

          {/* Semantic absorption particles */}
          {particles.map(p => (
            <div
              key={p.id}
              className="absolute rounded-full pointer-events-none"
              style={{
                width: p.size, height: p.size,
                left: 120 + p.x - p.size / 2,
                top: 120 + p.y - p.size / 2,
                background: orbColor,
                opacity: p.opacity * 0.65,
                boxShadow: `0 0 ${p.size * 2}px ${orbColor}`,
              }}
            />
          ))}

          {/* Echo words — drift up during listening */}
          {echoWords.map(w => (
            <div
              key={w.id}
              className="absolute pointer-events-none"
              style={{
                left: w.x,
                top: w.y,
                transform: `translate(-50%, -50%) scale(${w.scale})`,
                opacity: w.opacity,
                fontSize: 10,
                color: orbColor,
                letterSpacing: '0.08em',
                fontWeight: 500,
                whiteSpace: 'nowrap',
                textShadow: `0 0 8px ${orbColor}`,
                transition: 'none',
              }}
            >
              {w.text}
            </div>
          ))}

          {/* Organic wave field SVG */}
          <svg width="240" height="240" className="absolute inset-0" style={{ pointerEvents: 'none' }}>
            <defs>
              <radialGradient id="vglow2" cx="50%" cy="50%" r="50%">
                <stop offset="0%" stopColor={orbColor} stopOpacity="0.12" />
                <stop offset="100%" stopColor={orbColor} stopOpacity="0" />
              </radialGradient>
            </defs>
            <circle cx="120" cy="120" r="112" fill="url(#vglow2)" />

            {waveRings.map((ring, i) => (
              <polygon
                key={i}
                points={ring.points}
                fill="none"
                stroke={orbColor}
                strokeWidth={1.1 - i * 0.12}
                opacity={ring.opacity}
                strokeLinejoin="round"
              />
            ))}

            {/* Understanding: convergence lines */}
            {stage === 'understanding' && Array.from({ length: 8 }, (_, i) => {
              const a = (i / 8) * Math.PI * 2 + wavePhase * 0.25;
              return (
                <line
                  key={i}
                  x1={120 + Math.cos(a) * 92}
                  y1={120 + Math.sin(a) * 92}
                  x2={120 + Math.cos(a) * 44}
                  y2={120 + Math.sin(a) * 44}
                  stroke={orbColor}
                  strokeWidth="0.5"
                  opacity={0.1 + Math.sin(wavePhase * 2 + i) * 0.05}
                />
              );
            })}

            {/* Executing: progress arc */}
            {stage === 'executing' && progressArc && (
              <path
                d={progressArc}
                fill="none"
                stroke={orbColor}
                strokeWidth="2"
                strokeLinecap="round"
                opacity={0.7}
              />
            )}
            {stage === 'executing' && (
              <circle cx="120" cy="120" r="52" fill="none" stroke={`${orbColor}15`} strokeWidth="1.5" />
            )}
          </svg>

          {/* Pulsing rings — listening */}
          {stage === 'listening' && (
            <>
              <div className="absolute rounded-full pointer-events-none" style={{
                width: 200, height: 200, left: 20, top: 20,
                border: `1px solid ${orbColor}22`,
                animation: 'vPulse 1.9s ease-out infinite',
              }} />
              <div className="absolute rounded-full pointer-events-none" style={{
                width: 232, height: 232, left: 4, top: 4,
                border: `1px solid ${orbColor}10`,
                animation: 'vPulse 1.9s ease-out infinite 0.65s',
              }} />
            </>
          )}

          {/* Convergence ring — understanding */}
          {stage === 'understanding' && (
            <div className="absolute rounded-full pointer-events-none" style={{
              width: 178, height: 178, left: 31, top: 31,
              border: `1px solid ${orbColor}28`,
              animation: 'vConverge 1.3s ease-in-out infinite',
            }} />
          )}

          {/* Core orb */}
          <div
            className="absolute rounded-full flex items-center justify-center"
            style={{
              width: 80, height: 80, left: 80, top: 80,
              background: `radial-gradient(circle at 38% 32%, ${orbColor}cc 0%, rgba(3,5,12,0.95) 65%)`,
              boxShadow: `0 0 ${28 + audioLevel * 38}px ${orbGlow}, 0 0 ${12 + audioLevel * 18}px ${orbGlow} inset`,
              transform: `scale(${cfg.coreScale + audioLevel * 0.035})`,
              transition: 'transform 0.15s ease, box-shadow 0.15s ease',
            }}
          >
            {stage === 'ready' && <i className="ri-mic-line" style={{ fontSize: 26, color: orbColor, opacity: 0.65 }} />}
            {stage === 'listening' && <i className="ri-mic-fill" style={{ fontSize: 28, color: orbColor, animation: 'vMicPulse 0.85s ease-in-out infinite' }} />}
            {stage === 'understanding' && <i className="ri-brain-line" style={{ fontSize: 26, color: orbColor, animation: 'vSpin 3s linear infinite' }} />}
            {stage === 'confirming' && <i className="ri-check-double-line" style={{ fontSize: 26, color: orbColor }} />}
            {stage === 'executing' && (
              executingDone
                ? <i className="ri-check-line" style={{ fontSize: 28, color: orbColor, animation: 'vFragIn 0.3s ease' }} />
                : <i className="ri-loader-4-line" style={{ fontSize: 26, color: orbColor, animation: 'vSpin 1s linear infinite' }} />
            )}
          </div>

          {/* Intent fragments — float around core during understanding */}
          {stage === 'understanding' && intentFragments.map((frag, i) => {
            const a = (i / Math.max(intentFragments.length, 1)) * Math.PI * 2 - Math.PI / 2;
            const r = 96;
            return (
              <div
                key={frag.id}
                className="absolute pointer-events-none"
                style={{
                  left: 120 + Math.cos(a) * r,
                  top: 120 + Math.sin(a) * r,
                  transform: 'translate(-50%, -50%)',
                  padding: '3px 10px',
                  background: `${orbColor}10`,
                  border: `1px solid ${orbColor}28`,
                  borderRadius: 20,
                  fontSize: 10,
                  color: orbColor,
                  letterSpacing: '0.06em',
                  whiteSpace: 'nowrap',
                  animation: 'vFragIn 0.4s cubic-bezier(0.16,1,0.3,1)',
                }}
              >
                {frag.text}
              </div>
            );
          })}

          {/* Executing: progress % label */}
          {stage === 'executing' && !executingDone && (
            <div
              className="absolute pointer-events-none"
              style={{
                left: 120, top: 148,
                transform: 'translateX(-50%)',
                fontSize: 9,
                color: `${orbColor}80`,
                letterSpacing: '0.12em',
              }}
            >
              {executingProgress}%
            </div>
          )}
        </div>

        {/* ── Status text ── */}
        <div className="text-center mb-5" style={{ minHeight: 54 }}>
          <div
            key={stage + '-s'}
            style={{
              fontSize: 15,
              color: stage === 'ready' ? `${orbColor}70` : stage === 'executing' && executingDone ? orbColor : orbColor,
              letterSpacing: '0.08em',
              fontWeight: 500,
              marginBottom: 8,
              animation: 'vTextIn 0.35s ease',
            }}
          >
            {executingDone ? '已完成，正在打开…' : cfg.statusLine}
          </div>
          <div
            key={stage + '-sub-' + executingStep}
            style={{
              fontSize: 11,
              color: 'rgba(148,163,184,0.48)',
              letterSpacing: '0.04em',
              maxWidth: 320,
              lineHeight: 1.6,
              animation: 'vTextIn 0.3s ease 0.08s both',
            }}
          >
            {cfg.subLine}
          </div>
        </div>

        {/* ── Confirming: summary + actions ── */}
        {stage === 'confirming' && (
          <div
            className="flex flex-col items-center gap-3 mb-5"
            style={{ animation: 'vTextIn 0.4s cubic-bezier(0.16,1,0.3,1)' }}
          >
            <div
              className="rounded"
              style={{
                padding: '10px 22px',
                background: `${orbColor}08`,
                border: `1px solid ${orbColor}1e`,
                fontSize: 12,
                color: 'rgba(226,232,240,0.72)',
                letterSpacing: '0.04em',
                maxWidth: 340,
                textAlign: 'center',
                lineHeight: 1.65,
              }}
            >
              {confirmSummary}
            </div>
            <div className="flex items-center gap-2 flex-wrap justify-center">
              {confirmActions.map((action, i) => (
                <button
                  key={i}
                  onClick={() => handleAction(action)}
                  className="px-4 py-2 rounded cursor-pointer transition-all duration-200 whitespace-nowrap"
                  style={{
                    background: action.primary ? orbColor : 'rgba(255,255,255,0.04)',
                    border: `1px solid ${action.primary ? orbColor : 'rgba(255,255,255,0.09)'}`,
                    color: action.primary ? '#03050c' : action.cancel ? 'rgba(100,116,139,0.5)' : 'rgba(148,163,184,0.65)',
                    fontSize: 11,
                    fontWeight: action.primary ? 600 : 400,
                    letterSpacing: '0.06em',
                  }}
                  onMouseEnter={e => {
                    if (!action.primary) {
                      (e.currentTarget as HTMLButtonElement).style.background = `${orbColor}12`;
                      (e.currentTarget as HTMLButtonElement).style.color = orbColor;
                    }
                  }}
                  onMouseLeave={e => {
                    if (!action.primary) {
                      (e.currentTarget as HTMLButtonElement).style.background = 'rgba(255,255,255,0.04)';
                      (e.currentTarget as HTMLButtonElement).style.color = action.cancel ? 'rgba(100,116,139,0.5)' : 'rgba(148,163,184,0.65)';
                    }
                  }}
                >
                  {action.label}
                </button>
              ))}
            </div>
          </div>
        )}

        {/* ── Executing: step list ── */}
        {stage === 'executing' && (
          <div
            className="flex flex-col items-center gap-2 mb-5"
            style={{ animation: 'vTextIn 0.4s ease', minWidth: 260 }}
          >
            {/* Progress bar */}
            <div
              className="rounded-full overflow-hidden"
              style={{ width: 220, height: 2, background: 'rgba(255,255,255,0.06)', marginBottom: 8 }}
            >
              <div
                className="h-full rounded-full transition-all duration-700"
                style={{
                  width: `${executingProgress}%`,
                  background: executingDone
                    ? `linear-gradient(to right, ${orbColor}, ${orbColor})`
                    : `linear-gradient(to right, ${orbColor}80, ${orbColor})`,
                  boxShadow: `0 0 8px ${orbColor}`,
                }}
              />
            </div>
            {/* Current step */}
            <div
              key={executingStep}
              style={{
                fontSize: 11,
                color: executingDone ? orbColor : 'rgba(148,163,184,0.55)',
                letterSpacing: '0.06em',
                animation: 'vTextIn 0.3s ease',
              }}
            >
              {executingDone ? '✓ 全部完成' : executingStep}
            </div>
          </div>
        )}

        {/* ── Voice suggestions ── */}
        {(stage === 'ready' || stage === 'listening') && (
          <div
            className="flex flex-col items-center gap-2"
            style={{
              opacity: suggestionVisible ? 1 : 0,
              transform: suggestionVisible ? 'translateY(0)' : 'translateY(8px)',
              transition: 'opacity 0.5s ease, transform 0.5s ease',
              maxWidth: 380,
            }}
          >
            <div style={{ fontSize: 9, color: 'rgba(71,85,105,0.45)', letterSpacing: '0.2em', marginBottom: 4 }}>
              你现在可以说
            </div>
            <div className="flex flex-wrap justify-center gap-2">
              {VOICE_SUGGESTIONS.map((s, i) => (
                <button
                  key={i}
                  onClick={() => handleSuggestionClick(s)}
                  className="px-3 py-1.5 rounded-full cursor-pointer transition-all duration-200 whitespace-nowrap"
                  style={{
                    background: 'rgba(255,255,255,0.03)',
                    border: '1px solid rgba(255,255,255,0.07)',
                    color: 'rgba(148,163,184,0.42)',
                    fontSize: 10,
                    letterSpacing: '0.04em',
                  }}
                  onMouseEnter={e => {
                    (e.currentTarget as HTMLButtonElement).style.background = `${orbColor}0e`;
                    (e.currentTarget as HTMLButtonElement).style.borderColor = `${orbColor}22`;
                    (e.currentTarget as HTMLButtonElement).style.color = `${orbColor}cc`;
                  }}
                  onMouseLeave={e => {
                    (e.currentTarget as HTMLButtonElement).style.background = 'rgba(255,255,255,0.03)';
                    (e.currentTarget as HTMLButtonElement).style.borderColor = 'rgba(255,255,255,0.07)';
                    (e.currentTarget as HTMLButtonElement).style.color = 'rgba(148,163,184,0.42)';
                  }}
                >
                  {s}
                </button>
              ))}
            </div>
          </div>
        )}

        {/* ── Close hint ── */}
        <div
          className="absolute"
          style={{ bottom: -50, left: '50%', transform: 'translateX(-50%)', opacity: 0.22, whiteSpace: 'nowrap' }}
        >
          <span style={{ fontSize: 9.5, color: 'rgba(100,116,139,0.5)', letterSpacing: '0.14em' }}>
            点击空白处退出语音场
          </span>
        </div>
      </div>

      <style>{`
        @keyframes vPulse {
          0% { transform: scale(1); opacity: 0.65; }
          100% { transform: scale(1.28); opacity: 0; }
        }
        @keyframes vConverge {
          0%, 100% { transform: scale(1); opacity: 0.28; }
          50% { transform: scale(0.91); opacity: 0.48; }
        }
        @keyframes vMicPulse {
          0%, 100% { opacity: 1; transform: scale(1); }
          50% { opacity: 0.55; transform: scale(0.88); }
        }
        @keyframes vSpin {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }
        @keyframes vTextIn {
          from { opacity: 0; transform: translateY(5px); }
          to { opacity: 1; transform: translateY(0); }
        }
        @keyframes vFragIn {
          from { opacity: 0; transform: translate(-50%, -50%) scale(0.65); }
          to { opacity: 1; transform: translate(-50%, -50%) scale(1); }
        }
      `}</style>
    </div>
  );
}
