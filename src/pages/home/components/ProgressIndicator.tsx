import { useEffect, useState } from 'react';

interface ProgressIndicatorProps {
  agentState: 'idle' | 'working' | 'highlight' | 'completing' | 'done' | 'error';
  visible: boolean;
}

const stateProgress = {
  idle: { value: 0, label: '等待指令', steps: ['准备', '执行', '完成'], activeStep: -1 },
  working: { value: 42, label: '正在执行', steps: ['已启动', '检索中', '整理中', '待确认'], activeStep: 2 },
  highlight: { value: 58, label: '发现重点', steps: ['已启动', '检索完成', '分析中', '待确认'], activeStep: 2 },
  completing: { value: 85, label: '接近完成', steps: ['已启动', '执行完成', '草稿就绪', '待确认'], activeStep: 3 },
  done: { value: 100, label: '全部完成', steps: ['已启动', '执行完成', '草稿就绪', '已完成'], activeStep: 4 },
  error: { value: 58, label: '推进受阻', steps: ['已启动', '检索完成', '⚠ 权限缺失', '待解决'], activeStep: 2 },
};

const stateColors = {
  idle: '#64748b',
  working: '#34d399',
  highlight: '#fbbf24',
  completing: '#2dd4bf',
  done: '#e2e8f0',
  error: '#fb7185',
};

export default function ProgressIndicator({ agentState, visible }: ProgressIndicatorProps) {
  const [animatedValue, setAnimatedValue] = useState(0);
  const [mounted, setMounted] = useState(false);
  const config = stateProgress[agentState] || stateProgress.idle;
  const color = stateColors[agentState] || stateColors.idle;

  useEffect(() => {
    if (visible) {
      const t = setTimeout(() => setMounted(true), 200);
      return () => clearTimeout(t);
    } else {
      setMounted(false);
      setAnimatedValue(0);
    }
  }, [visible]);

  useEffect(() => {
    if (!mounted) return;
    const target = config.value;
    let current = 0;
    const step = target / 40;
    const interval = setInterval(() => {
      current = Math.min(current + step, target);
      setAnimatedValue(current);
      if (current >= target) clearInterval(interval);
    }, 18);
    return () => clearInterval(interval);
  }, [mounted, config.value]);

  const circumference = 2 * Math.PI * 38;
  const strokeDashoffset = circumference - (animatedValue / 100) * circumference;

  return (
    <div
      style={{
        opacity: mounted ? 1 : 0,
        transform: mounted ? 'translateY(0)' : 'translateY(10px)',
        transition: 'opacity 0.5s ease 0.15s, transform 0.5s ease 0.15s',
      }}
    >
      <div className="flex items-center gap-6">
        {/* Arc progress */}
        <div className="relative flex items-center justify-center" style={{ width: 88, height: 88 }}>
          <svg width="88" height="88" style={{ transform: 'rotate(-90deg)' }}>
            {/* Track */}
            <circle
              cx="44" cy="44" r="38"
              fill="none"
              stroke="rgba(255,255,255,0.06)"
              strokeWidth="2"
            />
            {/* Progress */}
            <circle
              cx="44" cy="44" r="38"
              fill="none"
              stroke={color}
              strokeWidth="2"
              strokeLinecap="round"
              strokeDasharray={circumference}
              strokeDashoffset={strokeDashoffset}
              style={{
                transition: 'stroke-dashoffset 0.6s ease, stroke 0.4s ease',
                filter: `drop-shadow(0 0 4px ${color}88)`,
              }}
            />
          </svg>
          {/* Center value */}
          <div className="absolute flex flex-col items-center">
            <span
              className="font-light"
              style={{ fontSize: 18, color: 'rgba(241,245,249,0.9)', lineHeight: 1 }}
            >
              {Math.round(animatedValue)}
              <span style={{ fontSize: 10, color: 'rgba(148,163,184,0.7)' }}>%</span>
            </span>
          </div>
        </div>

        {/* Step indicators */}
        <div className="flex flex-col gap-2">
          {config.steps.map((step, i) => {
            const isActive = i === config.activeStep - 1;
            const isDone = i < config.activeStep - 1 || config.value === 100;
            const isError = agentState === 'error' && i === config.activeStep - 1;
            return (
              <div key={i} className="flex items-center gap-2.5">
                <div
                  className="rounded-full flex-shrink-0"
                  style={{
                    width: 6,
                    height: 6,
                    background: isError
                      ? '#fb7185'
                      : isDone
                      ? color
                      : isActive
                      ? color
                      : 'rgba(255,255,255,0.12)',
                    boxShadow: isActive || isDone ? `0 0 6px ${color}88` : 'none',
                    opacity: isActive ? 1 : isDone ? 0.7 : 0.3,
                    animation: isActive && agentState !== 'done' ? 'pulse 1.8s ease-in-out infinite' : 'none',
                  }}
                />
                <span
                  style={{
                    fontSize: 12,
                    color: isActive
                      ? 'rgba(241,245,249,0.9)'
                      : isDone
                      ? 'rgba(148,163,184,0.7)'
                      : 'rgba(100,116,139,0.5)',
                    fontWeight: isActive ? 500 : 400,
                    transition: 'color 0.3s ease',
                  }}
                >
                  {step}
                </span>
              </div>
            );
          })}
        </div>
      </div>

      {/* Label */}
      <div className="mt-3 flex items-center gap-2">
        <span style={{ fontSize: 11, color: 'rgba(100,116,139,0.7)', letterSpacing: '0.12em' }}>
          {config.label}
        </span>
      </div>
    </div>
  );
}
