import { useEffect, useState } from 'react';

interface InteractionCueProps {
  agentState: 'idle' | 'working' | 'highlight' | 'completing' | 'done' | 'error';
  visible: boolean;
}

export default function InteractionCue({ agentState, visible }: InteractionCueProps) {
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    if (visible) {
      const t = setTimeout(() => setMounted(true), 600);
      return () => clearTimeout(t);
    } else {
      setMounted(false);
    }
  }, [visible]);

  const hints = {
    idle: '悬停查看 Agent 状态 · 右键进入仪表盘',
    working: '任务推进中，无需操作 · 点击查看详情',
    highlight: '有新进展值得关注 · 点击了解更多',
    completing: '草稿已就绪 · 点击确认发送',
    done: '任务已完成 · 点击查看摘要',
    error: '需要你介入 · 点击处理异常',
  };

  return (
    <div
      className="flex items-center justify-center gap-2"
      style={{
        opacity: mounted ? 0.45 : 0,
        transition: 'opacity 0.6s ease',
      }}
    >
      <div className="w-3 h-px" style={{ background: 'rgba(148,163,184,0.4)' }} />
      <span style={{ fontSize: 10.5, color: 'rgba(148,163,184,0.8)', letterSpacing: '0.08em' }}>
        {hints[agentState] || hints.idle}
      </span>
      <div className="w-3 h-px" style={{ background: 'rgba(148,163,184,0.4)' }} />
    </div>
  );
}
