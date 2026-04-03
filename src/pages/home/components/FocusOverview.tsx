import { useEffect, useState } from 'react';

interface FocusOverviewProps {
  agentState: 'idle' | 'working' | 'highlight' | 'completing' | 'done' | 'error';
  visible: boolean;
}

const stateContent = {
  idle: {
    tag: '待命',
    tagColor: '#94a3b8',
    headline: '我在这里，随时准备好了',
    subline: '没有进行中的任务。你可以通过悬浮球发起新的指令。',
    accent: '#94a3b8',
  },
  working: {
    tag: '正在推进',
    tagColor: '#34d399',
    headline: '已完成资料检索，正在整理关键信息',
    subline: '从 47 个来源中筛选出 6 个高相关结果，正在提炼核心观点。',
    accent: '#34d399',
  },
  highlight: {
    tag: '值得关注',
    tagColor: '#fbbf24',
    headline: '发现 2 个你可能最关心的风险点',
    subline: '在竞品分析中，检测到定价策略差异与用户留存数据异常，已标记为优先项。',
    accent: '#fbbf24',
  },
  completing: {
    tag: '接近完成',
    tagColor: '#2dd4bf',
    headline: '草稿已就绪，下一步只需你确认',
    subline: '报告初稿已生成，建议先看结论部分。确认后可直接发送给相关方。',
    accent: '#2dd4bf',
  },
  done: {
    tag: '已完成',
    tagColor: '#e2e8f0',
    headline: '会议纪要已整理完成',
    subline: '本次任务全部完成。共处理 3 个议题，生成摘要 1 份，待你确认发送对象。',
    accent: '#e2e8f0',
  },
  error: {
    tag: '需要你介入',
    tagColor: '#fb7185',
    headline: '当前步骤被阻塞，无法继续推进',
    subline: '缺少访问权限：无法读取「Q3 财务数据」文件夹。请确认授权后继续。',
    accent: '#fb7185',
  },
};

export default function FocusOverview({ agentState, visible }: FocusOverviewProps) {
  const [mounted, setMounted] = useState(false);
  const [textVisible, setTextVisible] = useState(false);
  const content = stateContent[agentState] || stateContent.idle;

  useEffect(() => {
    if (visible) {
      const t1 = setTimeout(() => setMounted(true), 80);
      const t2 = setTimeout(() => setTextVisible(true), 320);
      return () => { clearTimeout(t1); clearTimeout(t2); };
    } else {
      setMounted(false);
      setTextVisible(false);
    }
  }, [visible, agentState]);

  return (
    <div
      className="flex flex-col"
      style={{
        opacity: mounted ? 1 : 0,
        transform: mounted ? 'translateY(0)' : 'translateY(12px)',
        transition: 'opacity 0.5s ease, transform 0.5s ease',
        maxWidth: 420,
      }}
    >
      {/* Tag */}
      <div className="flex items-center gap-2 mb-4">
        <div
          className="w-1.5 h-1.5 rounded-full"
          style={{
            background: content.accent,
            boxShadow: `0 0 6px ${content.accent}`,
            animation: agentState !== 'idle' && agentState !== 'done' ? 'pulse 2s ease-in-out infinite' : 'none',
          }}
        />
        <span
          className="text-xs font-semibold tracking-widest uppercase"
          style={{ color: content.tagColor, letterSpacing: '0.2em', fontSize: 10 }}
        >
          {content.tag}
        </span>
      </div>

      {/* Headline */}
      <h2
        className="font-light leading-snug mb-3"
        style={{
          fontSize: 22,
          color: 'rgba(241,245,249,0.95)',
          letterSpacing: '-0.01em',
          opacity: textVisible ? 1 : 0,
          transform: textVisible ? 'translateY(0)' : 'translateY(8px)',
          transition: 'opacity 0.45s ease 0.1s, transform 0.45s ease 0.1s',
          fontFamily: "'Inter', sans-serif",
          fontWeight: 300,
        }}
      >
        {content.headline}
      </h2>

      {/* Subline */}
      <p
        className="leading-relaxed"
        style={{
          fontSize: 13.5,
          color: 'rgba(148,163,184,0.85)',
          lineHeight: 1.7,
          opacity: textVisible ? 1 : 0,
          transform: textVisible ? 'translateY(0)' : 'translateY(6px)',
          transition: 'opacity 0.45s ease 0.22s, transform 0.45s ease 0.22s',
          fontWeight: 400,
        }}
      >
        {content.subline}
      </p>

      {/* Accent line */}
      <div
        className="mt-5 h-px"
        style={{
          background: `linear-gradient(to right, ${content.accent}40, transparent)`,
          width: textVisible ? '100%' : '0%',
          transition: 'width 0.7s ease 0.4s',
        }}
      />
    </div>
  );
}
