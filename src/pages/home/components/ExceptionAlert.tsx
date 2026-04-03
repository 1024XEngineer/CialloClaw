import { useEffect, useState } from 'react';

interface ExceptionAlertProps {
  agentState: 'idle' | 'working' | 'highlight' | 'completing' | 'done' | 'error';
  visible: boolean;
  onAction?: () => void;
  onDismiss?: () => void;
}

export default function ExceptionAlert({ agentState, visible, onAction, onDismiss }: ExceptionAlertProps) {
  const [mounted, setMounted] = useState(false);
  const [pulse, setPulse] = useState(0);
  const isError = agentState === 'error';
  const isHighlight = agentState === 'highlight';
  const isCompleting = agentState === 'completing';

  useEffect(() => {
    if (visible && (isError || isHighlight || isCompleting)) {
      const t = setTimeout(() => setMounted(true), 500);
      return () => clearTimeout(t);
    } else {
      setMounted(false);
    }
  }, [visible, agentState, isError, isHighlight, isCompleting]);

  useEffect(() => {
    if (!mounted) return;
    let frame: number;
    const start = Date.now();
    const animate = () => {
      const elapsed = (Date.now() - start) / 1000;
      setPulse(Math.sin(elapsed * 2.5) * 0.5 + 0.5);
      frame = requestAnimationFrame(animate);
    };
    frame = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(frame);
  }, [mounted]);

  if (!isError && !isHighlight && !isCompleting) return null;

  const config = isError
    ? {
        color: '#fb7185',
        bg: 'rgba(251,113,133,0.08)',
        border: 'rgba(251,113,133,0.25)',
        icon: 'ri-error-warning-line',
        title: '需要你介入',
        desc: '缺少「Q3 财务数据」访问权限，当前步骤无法继续。',
        action: '前往授权',
        dismiss: '稍后处理',
      }
    : isCompleting
    ? {
        color: '#2dd4bf',
        bg: 'rgba(45,212,191,0.07)',
        border: 'rgba(45,212,191,0.2)',
        icon: 'ri-checkbox-circle-line',
        title: '草稿已就绪',
        desc: '确认发送对象后，我可以立即帮你发出。',
        action: '确认发送',
        dismiss: '再看看',
      }
    : {
        color: '#fbbf24',
        bg: 'rgba(251,191,36,0.07)',
        border: 'rgba(251,191,36,0.2)',
        icon: 'ri-lightbulb-flash-line',
        title: '发现值得关注的重点',
        desc: '检测到 2 个风险点，建议在继续推进前先确认。',
        action: '查看详情',
        dismiss: '继续推进',
      };

  return (
    <div
      className="rounded-2xl px-5 py-4"
      style={{
        opacity: mounted ? 1 : 0,
        transform: mounted ? 'translateY(0) scale(1)' : 'translateY(6px) scale(0.98)',
        transition: 'opacity 0.45s ease, transform 0.45s ease',
        background: config.bg,
        border: `1px solid ${config.border}`,
        backdropFilter: 'blur(16px)',
        boxShadow: isError
          ? `0 0 ${20 + pulse * 10}px rgba(251,113,133,${0.08 + pulse * 0.06})`
          : 'none',
      }}
    >
      <div className="flex items-start gap-3">
        <div
          className="w-8 h-8 flex items-center justify-center rounded-full flex-shrink-0 mt-0.5"
          style={{
            background: `${config.color}18`,
            boxShadow: isError ? `0 0 ${8 + pulse * 6}px ${config.color}44` : 'none',
          }}
        >
          <i className={config.icon} style={{ fontSize: 15, color: config.color }} />
        </div>
        <div className="flex-1 min-w-0">
          <div
            className="font-medium mb-1"
            style={{ fontSize: 13, color: config.color }}
          >
            {config.title}
          </div>
          <div style={{ fontSize: 12, color: 'rgba(148,163,184,0.8)', lineHeight: 1.6 }}>
            {config.desc}
          </div>
          <div className="flex items-center gap-3 mt-3">
            <button
              onClick={onAction}
              className="px-4 py-1.5 rounded-full text-xs font-medium whitespace-nowrap cursor-pointer transition-all duration-200"
              style={{
                background: config.color,
                color: '#0f172a',
                fontSize: 11.5,
                letterSpacing: '0.02em',
              }}
              onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.opacity = '0.85'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.opacity = '1'; }}
            >
              {config.action}
            </button>
            <button
              onClick={onDismiss}
              className="text-xs cursor-pointer transition-all duration-200 whitespace-nowrap"
              style={{ color: 'rgba(100,116,139,0.7)', fontSize: 11.5 }}
              onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'rgba(148,163,184,0.9)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'rgba(100,116,139,0.7)'; }}
            >
              {config.dismiss}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
