import { useEffect, useState } from 'react';

interface Shortcut {
  keys: string[];
  label: string;
  category: string;
}

const SHORTCUTS: Shortcut[] = [
  { keys: ['Esc'], label: '关闭当前面板 / 退出语音场', category: '导航' },
  { keys: ['↑', '↓'], label: '切换模块（任务/便签/镜子/感知）', category: '导航' },
  { keys: ['←', '→'], label: '切换当前模块的子状态', category: '导航' },
  { keys: ['1'], label: '打开任务状态', category: '快速打开' },
  { keys: ['2'], label: '打开便签协作', category: '快速打开' },
  { keys: ['3'], label: '打开镜子', category: '快速打开' },
  { keys: ['4'], label: '打开硬件感知', category: '快速打开' },
  { keys: ['5'], label: '打开信任与边界', category: '快速打开' },
  { keys: ['长按', '中心球'], label: '唤起语音场（持续 0.65s）', category: '语音场' },
  { keys: ['点击', '建议词'], label: '快速触发语音意图识别', category: '语音场' },
  { keys: ['Esc'], label: '退出语音场 / 取消当前阶段', category: '语音场' },
  { keys: ['悬停', '标签'], label: '显示关闭按钮（× 隐藏标签）', category: '关注标签' },
  { keys: ['拖拽', '标签'], label: '自由移动标签位置', category: '关注标签' },
  { keys: ['撤销', '按钮'], label: '4 秒内可恢复已隐藏的标签', category: '关注标签' },
  { keys: ['F'], label: '切换专注模式（隐藏所有外围标签和球）', category: '其他' },
  { keys: ['?'], label: '显示/隐藏快捷键帮助', category: '其他' },
];

interface ShortcutPanelProps {
  visible: boolean;
  onClose: () => void;
}

export default function ShortcutPanel({ visible, onClose }: ShortcutPanelProps) {
  const [show, setShow] = useState(false);

  useEffect(() => {
    if (visible) {
      const t = setTimeout(() => setShow(true), 30);
      return () => clearTimeout(t);
    } else {
      setShow(false);
    }
  }, [visible]);

  if (!visible) return null;

  const categories = Array.from(new Set(SHORTCUTS.map(s => s.category)));

  return (
    <div
      className="fixed inset-0 flex items-center justify-center pointer-events-auto"
      style={{ zIndex: 300, background: 'rgba(3,5,12,0.88)', backdropFilter: 'blur(16px)' }}
      onClick={onClose}
    >
      <div
        className="relative"
        style={{
          width: 540,
          opacity: show ? 1 : 0,
          transform: show ? 'scale(1) translateY(0)' : 'scale(0.95) translateY(16px)',
          transition: 'all 0.35s cubic-bezier(0.16,1,0.3,1)',
        }}
        onClick={e => e.stopPropagation()}
      >
        {/* Corner brackets */}
        {(['tl','tr','bl','br'] as const).map(c => (
          <div key={c} className="absolute" style={{
            top: c.startsWith('t') ? -2 : undefined,
            bottom: c.startsWith('b') ? -2 : undefined,
            left: c.endsWith('l') ? -2 : undefined,
            right: c.endsWith('r') ? -2 : undefined,
            zIndex: 10,
          }}>
            {c === 'tl' && <><div style={{ width: 18, height: 2, background: '#64748b' }} /><div style={{ width: 2, height: 18, background: '#64748b' }} /></>}
            {c === 'tr' && <><div style={{ width: 18, height: 2, background: '#64748b', marginLeft: 'auto' }} /><div style={{ width: 2, height: 18, background: '#64748b', marginLeft: 'auto' }} /></>}
            {c === 'bl' && <><div style={{ width: 2, height: 18, background: '#64748b' }} /><div style={{ width: 18, height: 2, background: '#64748b' }} /></>}
            {c === 'br' && <><div style={{ width: 2, height: 18, background: '#64748b', marginLeft: 'auto' }} /><div style={{ width: 18, height: 2, background: '#64748b', marginLeft: 'auto' }} /></>}
          </div>
        ))}

        <div style={{
          background: 'rgba(3,5,12,0.98)',
          border: '1px solid rgba(100,116,139,0.22)',
          borderRadius: 4,
          overflow: 'hidden',
          boxShadow: '0 0 0 1px rgba(100,116,139,0.08) inset, 0 40px 80px rgba(0,0,0,0.9)',
        }}>
          {/* Header */}
          <div className="flex items-center justify-between px-5 py-4" style={{ borderBottom: '1px solid rgba(100,116,139,0.14)' }}>
            <div className="flex items-center gap-3">
              <div className="flex items-center justify-center rounded-full" style={{
                width: 32, height: 32,
                background: 'rgba(100,116,139,0.14)',
                border: '1px solid rgba(100,116,139,0.28)',
              }}>
                <i className="ri-keyboard-line" style={{ fontSize: 14, color: '#94a3b8' }} />
              </div>
              <div>
                <div style={{ fontSize: 13, color: '#e2e8f0', letterSpacing: '0.15em', fontWeight: 600 }}>快捷键</div>
                <div style={{ fontSize: 9.5, color: 'rgba(148,163,184,0.45)', marginTop: 2 }}>KEYBOARD SHORTCUTS</div>
              </div>
            </div>
            <button
              onClick={onClose}
              className="flex items-center justify-center rounded cursor-pointer transition-all duration-200"
              style={{ width: 28, height: 28, background: 'rgba(255,255,255,0.04)', border: '1px solid rgba(100,116,139,0.22)', color: 'rgba(148,163,184,0.7)' }}
              onMouseEnter={e => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(251,113,133,0.12)'; (e.currentTarget as HTMLButtonElement).style.color = '#fb7185'; }}
              onMouseLeave={e => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(255,255,255,0.04)'; (e.currentTarget as HTMLButtonElement).style.color = 'rgba(148,163,184,0.7)'; }}
            >
              <i className="ri-close-line" style={{ fontSize: 14 }} />
            </button>
          </div>

          {/* Shortcuts */}
          <div className="p-5 flex flex-col gap-5">
            {categories.map(cat => (
              <div key={cat}>
                <div style={{ fontSize: 9.5, color: 'rgba(148,163,184,0.45)', letterSpacing: '0.2em', marginBottom: 10 }}>
                  {cat.toUpperCase()}
                </div>
                <div className="flex flex-col gap-1.5">
                  {SHORTCUTS.filter(s => s.category === cat).map((s, i) => (
                    <div
                      key={i}
                      className="flex items-center justify-between px-3.5 py-2.5 rounded"
                      style={{ background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.05)' }}
                    >
                      <span style={{ fontSize: 12.5, color: 'rgba(226,232,240,0.85)' }}>{s.label}</span>
                      <div className="flex items-center gap-1.5">
                        {s.keys.map((k, ki) => {
                          const isAction = ['长按', '点击', '悬停', '拖拽', '撤销'].includes(k);
                          return (
                            <span
                              key={ki}
                              className="px-2.5 py-0.5 rounded"
                              style={{
                                fontSize: 11,
                                color: isAction ? 'rgba(148,163,184,0.75)' : '#cbd5e1',
                                background: isAction ? 'rgba(255,255,255,0.04)' : 'rgba(100,116,139,0.18)',
                                border: isAction ? '1px solid rgba(255,255,255,0.08)' : '1px solid rgba(100,116,139,0.32)',
                                fontFamily: isAction ? 'inherit' : 'monospace',
                                letterSpacing: isAction ? '0.04em' : '0.05em',
                              }}
                            >
                              {k}
                            </span>
                          );
                        })}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>

          {/* Footer */}
          <div className="px-5 py-3 flex items-center gap-2" style={{ borderTop: '1px solid rgba(100,116,139,0.1)' }}>
            <i className="ri-information-line" style={{ fontSize: 10, color: 'rgba(148,163,184,0.4)' }} />
            <span style={{ fontSize: 9.5, color: 'rgba(148,163,184,0.45)', letterSpacing: '0.08em' }}>
              按 <span style={{ color: 'rgba(203,213,225,0.7)', fontFamily: 'monospace' }}>?</span> 随时打开此面板 · 按 <span style={{ color: 'rgba(203,213,225,0.7)', fontFamily: 'monospace' }}>Esc</span> 关闭
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
