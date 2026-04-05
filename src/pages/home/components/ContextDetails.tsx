import { useEffect, useState } from 'react';

interface ContextDetailsProps {
  agentState: 'idle' | 'working' | 'highlight' | 'completing' | 'done' | 'error';
  visible: boolean;
}

const stateContext = {
  idle: [],
  working: [
    { icon: 'ri-search-line', text: '已扫描 47 个来源', time: '刚刚' },
    { icon: 'ri-filter-line', text: '筛选出 6 个高相关结果', time: '1 分钟前' },
    { icon: 'ri-loader-4-line', text: '正在提炼核心观点...', time: '进行中', active: true },
  ],
  highlight: [
    { icon: 'ri-search-line', text: '竞品分析完成', time: '2 分钟前' },
    { icon: 'ri-alert-line', text: '定价策略差异 — 已标记', time: '1 分钟前', warn: true },
    { icon: 'ri-alert-line', text: '用户留存数据异常 — 已标记', time: '刚刚', warn: true },
  ],
  completing: [
    { icon: 'ri-file-text-line', text: '报告初稿已生成', time: '刚刚' },
    { icon: 'ri-eye-line', text: '建议先看结论部分', time: '' },
    { icon: 'ri-send-plane-line', text: '确认后可直接发送', time: '', hint: true },
  ],
  done: [
    { icon: 'ri-check-line', text: '3 个议题全部处理完成', time: '完成' },
    { icon: 'ri-file-text-line', text: '摘要已生成，共 1 份', time: '' },
    { icon: 'ri-user-line', text: '待确认发送对象', time: '', hint: true },
  ],
  error: [
    { icon: 'ri-check-line', text: '检索阶段已完成', time: '2 分钟前' },
    { icon: 'ri-lock-line', text: '无法读取「Q3 财务数据」', time: '刚刚', error: true },
    { icon: 'ri-question-line', text: '等待你授权后继续', time: '', hint: true },
  ],
};

export default function ContextDetails({ agentState, visible }: ContextDetailsProps) {
  const [mounted, setMounted] = useState(false);
  const items = stateContext[agentState] || [];

  useEffect(() => {
    if (visible && items.length > 0) {
      const t = setTimeout(() => setMounted(true), 350);
      return () => clearTimeout(t);
    } else {
      setMounted(false);
    }
  }, [visible, agentState, items.length]);

  if (items.length === 0) return null;

  return (
    <div
      style={{
        opacity: mounted ? 1 : 0,
        transform: mounted ? 'translateY(0)' : 'translateY(8px)',
        transition: 'opacity 0.5s ease 0.2s, transform 0.5s ease 0.2s',
      }}
    >
      <div
        className="rounded-2xl px-5 py-4"
        style={{
          background: 'rgba(255,255,255,0.03)',
          border: '1px solid rgba(255,255,255,0.06)',
          backdropFilter: 'blur(12px)',
        }}
      >
        <div className="flex flex-col gap-3">
          {items.map((item, i) => (
            <div
              key={i}
              className="flex items-center gap-3"
              style={{
                opacity: mounted ? 1 : 0,
                transform: mounted ? 'translateX(0)' : 'translateX(-8px)',
                transition: `opacity 0.4s ease ${0.1 + i * 0.1}s, transform 0.4s ease ${0.1 + i * 0.1}s`,
              }}
            >
              <div
                className="w-6 h-6 flex items-center justify-center flex-shrink-0 rounded-full"
                style={{
                  background: (item as { error?: boolean }).error
                    ? 'rgba(251,113,133,0.12)'
                    : (item as { warn?: boolean }).warn
                    ? 'rgba(251,191,36,0.1)'
                    : (item as { hint?: boolean }).hint
                    ? 'rgba(45,212,191,0.1)'
                    : 'rgba(255,255,255,0.05)',
                }}
              >
                <i
                  className={item.icon}
                  style={{
                    fontSize: 11,
                    color: (item as { error?: boolean }).error
                      ? '#fb7185'
                      : (item as { warn?: boolean }).warn
                      ? '#fbbf24'
                      : (item as { hint?: boolean }).hint
                      ? '#2dd4bf'
                      : (item as { active?: boolean }).active
                      ? '#34d399'
                      : 'rgba(148,163,184,0.7)',
                  }}
                />
              </div>
              <span
                style={{
                  fontSize: 12.5,
                  color: (item as { error?: boolean }).error
                    ? 'rgba(251,113,133,0.9)'
                    : (item as { warn?: boolean }).warn
                    ? 'rgba(251,191,36,0.85)'
                    : (item as { hint?: boolean }).hint
                    ? 'rgba(45,212,191,0.8)'
                    : (item as { active?: boolean }).active
                    ? 'rgba(52,211,153,0.9)'
                    : 'rgba(148,163,184,0.75)',
                  flex: 1,
                }}
              >
                {item.text}
              </span>
              {item.time && (
                <span style={{ fontSize: 10.5, color: 'rgba(100,116,139,0.5)', flexShrink: 0 }}>
                  {item.time}
                </span>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
