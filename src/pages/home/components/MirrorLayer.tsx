import { useEffect, useState, useRef } from 'react';
import type { AgentStateData } from '@/mocks/agentStates';

interface MirrorLayerProps {
  stateData: AgentStateData;
  visible: boolean;
}

// Radar chart for user profile
function RadarChart({ color, visible }: { color: string; visible: boolean }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const animRef = useRef<number>(0);
  const startRef = useRef<number>(0);

  const axes = [
    { label: '专注力', value: 0.82 },
    { label: '执行力', value: 0.74 },
    { label: '创造力', value: 0.68 },
    { label: '协作力', value: 0.55 },
    { label: '规律性', value: 0.88 },
    { label: '决策力', value: 0.71 },
  ];

  useEffect(() => {
    if (!visible) return;
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const W = canvas.width;
    const H = canvas.height;
    const cx = W / 2;
    const cy = H / 2;
    const R = Math.min(W, H) * 0.38;
    const N = axes.length;

    startRef.current = Date.now();

    const draw = () => {
      const progress = Math.min((Date.now() - startRef.current) / 900, 1);
      const ease = 1 - Math.pow(1 - progress, 3);

      ctx.clearRect(0, 0, W, H);

      // Background rings
      for (let r = 1; r <= 4; r++) {
        ctx.beginPath();
        for (let i = 0; i < N; i++) {
          const angle = (i / N) * Math.PI * 2 - Math.PI / 2;
          const x = cx + Math.cos(angle) * R * (r / 4);
          const y = cy + Math.sin(angle) * R * (r / 4);
          if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
        }
        ctx.closePath();
        ctx.strokeStyle = `${color}18`;
        ctx.lineWidth = 0.8;
        ctx.stroke();
      }

      // Axis lines
      for (let i = 0; i < N; i++) {
        const angle = (i / N) * Math.PI * 2 - Math.PI / 2;
        ctx.beginPath();
        ctx.moveTo(cx, cy);
        ctx.lineTo(cx + Math.cos(angle) * R, cy + Math.sin(angle) * R);
        ctx.strokeStyle = `${color}14`;
        ctx.lineWidth = 0.8;
        ctx.stroke();
      }

      // Data polygon
      ctx.beginPath();
      for (let i = 0; i < N; i++) {
        const angle = (i / N) * Math.PI * 2 - Math.PI / 2;
        const v = axes[i].value * ease;
        const x = cx + Math.cos(angle) * R * v;
        const y = cy + Math.sin(angle) * R * v;
        if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
      }
      ctx.closePath();
      ctx.fillStyle = `${color}18`;
      ctx.fill();
      ctx.strokeStyle = `${color}80`;
      ctx.lineWidth = 1.5;
      ctx.stroke();

      // Data points
      for (let i = 0; i < N; i++) {
        const angle = (i / N) * Math.PI * 2 - Math.PI / 2;
        const v = axes[i].value * ease;
        const x = cx + Math.cos(angle) * R * v;
        const y = cy + Math.sin(angle) * R * v;
        ctx.beginPath();
        ctx.arc(x, y, 2.5, 0, Math.PI * 2);
        ctx.fillStyle = color;
        ctx.fill();
      }

      // Labels
      ctx.font = '9px Inter, sans-serif';
      ctx.fillStyle = `${color}90`;
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      for (let i = 0; i < N; i++) {
        const angle = (i / N) * Math.PI * 2 - Math.PI / 2;
        const labelR = R + 16;
        const x = cx + Math.cos(angle) * labelR;
        const y = cy + Math.sin(angle) * labelR;
        ctx.fillText(axes[i].label, x, y);
      }

      if (progress < 1) {
        animRef.current = requestAnimationFrame(draw);
      }
    };

    animRef.current = requestAnimationFrame(draw);
    return () => cancelAnimationFrame(animRef.current);
  }, [visible, color]);

  return (
    <canvas
      ref={canvasRef}
      width={180}
      height={180}
      style={{ display: 'block' }}
    />
  );
}

// Timeline bar for period summary
function TimelineBar({ label, value, color, delay, visible }: {
  label: string; value: number; color: string; delay: number; visible: boolean;
}) {
  const [w, setW] = useState(0);
  useEffect(() => {
    if (visible) {
      const t = setTimeout(() => setW(value), delay);
      return () => clearTimeout(t);
    }
    setW(0);
  }, [visible, value, delay]);

  return (
    <div className="flex items-center gap-2.5">
      <span style={{ fontSize: 9.5, color: 'rgba(100,116,139,0.7)', width: 52, flexShrink: 0, letterSpacing: '0.05em' }}>{label}</span>
      <div className="flex-1 relative" style={{ height: 4, background: 'rgba(255,255,255,0.05)', borderRadius: 2 }}>
        <div
          style={{
            position: 'absolute', left: 0, top: 0, height: '100%',
            width: `${w * 100}%`,
            background: `linear-gradient(to right, ${color}60, ${color})`,
            borderRadius: 2,
            transition: 'width 0.8s cubic-bezier(0.34,1.56,0.64,1)',
            boxShadow: `0 0 6px ${color}60`,
          }}
        />
      </div>
      <span style={{ fontSize: 9, color, width: 28, textAlign: 'right', letterSpacing: '0.05em' }}>
        {Math.round(value * 100)}%
      </span>
    </div>
  );
}

// Heatmap for activity pattern
function ActivityHeatmap({ color, visible }: { color: string; visible: boolean }) {
  const days = ['一', '二', '三', '四', '五', '六', '日'];
  const hours = [9, 10, 11, 12, 14, 15, 16, 17, 18, 20];
  const data: Record<string, number> = {
    '一-10': 0.9, '一-11': 0.85, '一-14': 0.6, '一-15': 0.5,
    '二-10': 0.7, '二-11': 0.8, '二-15': 0.65, '二-16': 0.55,
    '三-9': 0.4, '三-10': 0.75, '三-11': 0.9, '三-14': 0.7,
    '四-10': 0.95, '四-11': 0.88, '四-14': 0.92, '四-15': 0.85, '四-16': 0.78,
    '五-10': 0.6, '五-11': 0.7, '五-14': 0.5, '五-15': 0.45,
    '六-11': 0.3, '六-14': 0.25,
    '日-20': 0.2,
  };

  const [show, setShow] = useState(false);
  useEffect(() => {
    if (visible) {
      const t = setTimeout(() => setShow(true), 200);
      return () => clearTimeout(t);
    }
    setShow(false);
  }, [visible]);

  return (
    <div>
      <div className="flex items-center gap-1 mb-1.5">
        {hours.map(h => (
          <div key={h} style={{ width: 14, fontSize: 7, color: 'rgba(71,85,105,0.5)', textAlign: 'center', letterSpacing: 0 }}>
            {h}
          </div>
        ))}
      </div>
      {days.map((day, di) => (
        <div key={day} className="flex items-center gap-1 mb-0.5">
          <span style={{ fontSize: 8, color: 'rgba(71,85,105,0.5)', width: 10, flexShrink: 0 }}>{day}</span>
          {hours.map(h => {
            const v = data[`${day}-${h}`] ?? 0;
            return (
              <div
                key={h}
                style={{
                  width: 14, height: 10,
                  borderRadius: 2,
                  background: v > 0 ? `${color}` : 'rgba(255,255,255,0.04)',
                  opacity: show ? (v > 0 ? v * 0.9 + 0.1 : 1) : 0,
                  transition: `opacity 0.4s ease ${(di * 7 + hours.indexOf(h)) * 0.015}s`,
                  boxShadow: v > 0.7 ? `0 0 4px ${color}60` : 'none',
                }}
              />
            );
          })}
        </div>
      ))}
      <div className="flex items-center gap-1.5 mt-2" style={{ opacity: 0.4 }}>
        <span style={{ fontSize: 8, color: 'rgba(71,85,105,0.6)' }}>低</span>
        {[0.15, 0.35, 0.55, 0.75, 0.95].map(v => (
          <div key={v} style={{ width: 10, height: 8, borderRadius: 1.5, background: color, opacity: v }} />
        ))}
        <span style={{ fontSize: 8, color: 'rgba(71,85,105,0.6)' }}>高</span>
      </div>
    </div>
  );
}

export default function MirrorLayer({ stateData, visible }: MirrorLayerProps) {
  const [show, setShow] = useState(false);
  const [insightsShow, setInsightsShow] = useState(false);
  const [activeTab, setActiveTab] = useState<'profile' | 'period'>('profile');
  const insights = stateData.mirrorInsights ?? [];
  const color = stateData.accentColor;
  const isSummary = stateData.key === 'mirror_summary';

  useEffect(() => {
    if (visible) {
      const t1 = setTimeout(() => setShow(true), 80);
      const t2 = setTimeout(() => setInsightsShow(true), 350);
      return () => { clearTimeout(t1); clearTimeout(t2); };
    }
    setShow(false);
    setInsightsShow(false);
  }, [visible, stateData.key]);

  if (!insights.length) return null;

  const periodData = [
    { label: '产品策略', value: 0.88 },
    { label: '竞品分析', value: 0.76 },
    { label: '周报整理', value: 0.92 },
    { label: '数据分析', value: 0.61 },
    { label: '团队沟通', value: 0.54 },
  ];

  return (
    <div
      style={{
        opacity: show ? 1 : 0,
        transform: show ? 'translateY(0)' : 'translateY(10px)',
        transition: 'opacity 0.45s ease, transform 0.45s ease',
      }}
    >
      {/* Section header */}
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <div className="w-4 h-px" style={{ background: `linear-gradient(to right, ${color}60, transparent)` }} />
          <span style={{ fontSize: 9.5, color, letterSpacing: '0.2em', fontWeight: 600, opacity: 0.85 }}>
            镜子 · 我观察到的
          </span>
          {stateData.mirrorPeriod && (
            <span style={{ fontSize: 8.5, color: 'rgba(71,85,105,0.55)', marginLeft: 2 }}>
              {stateData.mirrorPeriod}
            </span>
          )}
        </div>
      </div>

      {/* Tab switcher */}
      {isSummary && (
        <div
          className="flex items-center gap-1 mb-4 p-1 rounded-full"
          style={{ background: 'rgba(255,255,255,0.04)', border: `1px solid ${color}15`, width: 'fit-content' }}
        >
          {(['profile', 'period'] as const).map(tab => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className="px-3 py-1 rounded-full cursor-pointer transition-all duration-200 whitespace-nowrap"
              style={{
                fontSize: 9.5,
                background: activeTab === tab ? `${color}20` : 'transparent',
                color: activeTab === tab ? color : 'rgba(71,85,105,0.55)',
                border: activeTab === tab ? `1px solid ${color}35` : '1px solid transparent',
                letterSpacing: '0.08em',
              }}
            >
              {tab === 'profile' ? '用户画像' : '周期总结'}
            </button>
          ))}
        </div>
      )}

      {/* ── USER PROFILE TAB ── */}
      {(!isSummary || activeTab === 'profile') && (
        <div
          style={{
            opacity: insightsShow ? 1 : 0,
            transform: insightsShow ? 'translateY(0)' : 'translateY(6px)',
            transition: 'opacity 0.4s ease, transform 0.4s ease',
          }}
        >
          {/* Radar + stats side by side */}
          <div className="flex items-start gap-4 mb-4">
            {/* Radar chart */}
            <div
              className="flex-shrink-0 flex items-center justify-center rounded-lg"
              style={{
                background: `${color}06`,
                border: `1px solid ${color}15`,
                padding: 8,
              }}
            >
              <RadarChart color={color} visible={insightsShow} />
            </div>

            {/* Right stats */}
            <div className="flex-1 flex flex-col gap-2.5 pt-1">
              {/* Identity card */}
              <div
                className="px-3 py-2.5 rounded-lg"
                style={{ background: `${color}08`, border: `1px solid ${color}18` }}
              >
                <div style={{ fontSize: 8.5, color: 'rgba(71,85,105,0.5)', letterSpacing: '0.15em', marginBottom: 4 }}>PROFILE · ID</div>
                <div style={{ fontSize: 13, color: 'rgba(226,232,240,0.9)', fontWeight: 500, lineHeight: 1.4 }}>
                  深度执行者
                </div>
                <div style={{ fontSize: 10, color: 'rgba(148,163,184,0.6)', marginTop: 2, lineHeight: 1.5 }}>
                  规律性强 · 偏好结论优先 · 周四深度工作
                </div>
              </div>

              {/* Key metrics */}
              <div className="grid grid-cols-2 gap-1.5">
                {[
                  { label: '高效时段', value: '10-12h', icon: 'ri-time-line' },
                  { label: '任务完成率', value: '94%', icon: 'ri-check-double-line' },
                  { label: '习惯稳定性', value: '极高', icon: 'ri-repeat-2-line' },
                  { label: '决策速度', value: '快', icon: 'ri-flashlight-line' },
                ].map((m, i) => (
                  <div
                    key={i}
                    className="px-2 py-1.5 rounded"
                    style={{
                      background: 'rgba(255,255,255,0.03)',
                      border: `1px solid ${color}10`,
                      opacity: insightsShow ? 1 : 0,
                      transition: `opacity 0.3s ease ${i * 0.08}s`,
                    }}
                  >
                    <div className="flex items-center gap-1 mb-0.5">
                      <i className={m.icon} style={{ fontSize: 8, color: `${color}80` }} />
                      <span style={{ fontSize: 8, color: 'rgba(71,85,105,0.5)', letterSpacing: '0.08em' }}>{m.label}</span>
                    </div>
                    <div style={{ fontSize: 11, color: color, fontWeight: 500 }}>{m.value}</div>
                  </div>
                ))}
              </div>
            </div>
          </div>

          {/* Activity heatmap */}
          <div
            className="px-3 py-3 rounded-lg mb-3"
            style={{ background: `${color}05`, border: `1px solid ${color}12` }}
          >
            <div className="flex items-center gap-1.5 mb-2.5">
              <i className="ri-calendar-2-line" style={{ fontSize: 9, color: `${color}80` }} />
              <span style={{ fontSize: 9, color: 'rgba(71,85,105,0.55)', letterSpacing: '0.12em' }}>活跃时段热力图 · 近两周</span>
            </div>
            <ActivityHeatmap color={color} visible={insightsShow} />
          </div>

          {/* Insights list */}
          <div className="flex flex-col gap-2">
            {insights.map((insight, i) => (
              <div
                key={i}
                className="flex items-start gap-2.5"
                style={{
                  opacity: insightsShow ? 1 : 0,
                  transform: insightsShow ? 'translateX(0)' : 'translateX(-6px)',
                  transition: `opacity 0.4s ease ${i * 0.1 + 0.1}s, transform 0.4s ease ${i * 0.1 + 0.1}s`,
                }}
              >
                <div
                  className="flex items-center justify-center rounded-full flex-shrink-0 mt-0.5"
                  style={{
                    width: 20, height: 20,
                    background: insight.emphasis ? `${color}20` : 'rgba(255,255,255,0.04)',
                    border: insight.emphasis ? `1px solid ${color}35` : '1px solid rgba(255,255,255,0.06)',
                  }}
                >
                  <i className={insight.icon} style={{ fontSize: 9, color: insight.emphasis ? color : 'rgba(100,116,139,0.6)' }} />
                </div>
                <p style={{
                  fontSize: 12,
                  color: insight.emphasis ? 'rgba(226,232,240,0.88)' : 'rgba(148,163,184,0.65)',
                  lineHeight: 1.65,
                  fontWeight: insight.emphasis ? 400 : 300,
                }}>
                  {insight.text}
                </p>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* ── PERIOD SUMMARY TAB ── */}
      {isSummary && activeTab === 'period' && (
        <div
          style={{
            opacity: insightsShow ? 1 : 0,
            transform: insightsShow ? 'translateY(0)' : 'translateY(6px)',
            transition: 'opacity 0.4s ease, transform 0.4s ease',
          }}
        >
          {/* Period header */}
          <div
            className="flex items-center justify-between px-3 py-2.5 rounded-lg mb-3"
            style={{ background: `${color}08`, border: `1px solid ${color}18` }}
          >
            <div>
              <div style={{ fontSize: 8.5, color: 'rgba(71,85,105,0.5)', letterSpacing: '0.15em', marginBottom: 3 }}>PERIOD SUMMARY</div>
              <div style={{ fontSize: 12, color: 'rgba(226,232,240,0.85)', fontWeight: 500 }}>
                {stateData.mirrorPeriod ?? '近两周'}
              </div>
            </div>
            <div className="flex flex-col items-end gap-1">
              <div style={{ fontSize: 22, color, fontWeight: 700, lineHeight: 1 }}>14</div>
              <div style={{ fontSize: 8, color: 'rgba(71,85,105,0.5)', letterSpacing: '0.1em' }}>天</div>
            </div>
          </div>

          {/* Stats row */}
          <div className="grid grid-cols-3 gap-2 mb-4">
            {[
              { label: '完成任务', value: '47', unit: '项', icon: 'ri-check-double-line' },
              { label: '高效时长', value: '68', unit: 'h', icon: 'ri-time-line' },
              { label: '习惯打卡', value: '12/14', unit: '天', icon: 'ri-repeat-2-line' },
            ].map((s, i) => (
              <div
                key={i}
                className="flex flex-col items-center py-2.5 rounded-lg"
                style={{
                  background: 'rgba(255,255,255,0.03)',
                  border: `1px solid ${color}12`,
                  opacity: insightsShow ? 1 : 0,
                  transition: `opacity 0.4s ease ${i * 0.1}s`,
                }}
              >
                <i className={s.icon} style={{ fontSize: 11, color: `${color}80`, marginBottom: 4 }} />
                <div style={{ fontSize: 16, color, fontWeight: 600, lineHeight: 1 }}>{s.value}</div>
                <div style={{ fontSize: 8, color: 'rgba(71,85,105,0.5)', marginTop: 2 }}>{s.unit} · {s.label}</div>
              </div>
            ))}
          </div>

          {/* Task distribution bars */}
          <div
            className="px-3 py-3 rounded-lg mb-3"
            style={{ background: `${color}05`, border: `1px solid ${color}12` }}
          >
            <div className="flex items-center gap-1.5 mb-3">
              <i className="ri-bar-chart-horizontal-line" style={{ fontSize: 9, color: `${color}80` }} />
              <span style={{ fontSize: 9, color: 'rgba(71,85,105,0.55)', letterSpacing: '0.12em' }}>任务类型分布</span>
            </div>
            <div className="flex flex-col gap-2">
              {periodData.map((d, i) => (
                <TimelineBar
                  key={d.label}
                  label={d.label}
                  value={d.value}
                  color={color}
                  delay={i * 100 + 200}
                  visible={insightsShow}
                />
              ))}
            </div>
          </div>

          {/* Weekly rhythm */}
          <div
            className="px-3 py-3 rounded-lg mb-3"
            style={{ background: `${color}05`, border: `1px solid ${color}12` }}
          >
            <div className="flex items-center gap-1.5 mb-3">
              <i className="ri-rhythm-line" style={{ fontSize: 9, color: `${color}80` }} />
              <span style={{ fontSize: 9, color: 'rgba(71,85,105,0.55)', letterSpacing: '0.12em' }}>周节律</span>
            </div>
            <div className="flex items-end gap-1.5">
              {[
                { day: '一', h: 0.65 }, { day: '二', h: 0.72 }, { day: '三', h: 0.58 },
                { day: '四', h: 0.95 }, { day: '五', h: 0.61 }, { day: '六', h: 0.28 }, { day: '日', h: 0.18 },
              ].map((d, i) => (
                <div key={d.day} className="flex flex-col items-center gap-1 flex-1">
                  <div
                    style={{
                      width: '100%',
                      height: 48 * d.h,
                      background: d.h > 0.8
                        ? `linear-gradient(to top, ${color}80, ${color})`
                        : `${color}40`,
                      borderRadius: '2px 2px 0 0',
                      boxShadow: d.h > 0.8 ? `0 0 8px ${color}50` : 'none',
                      opacity: insightsShow ? 1 : 0,
                      transition: `opacity 0.4s ease ${i * 0.06}s, height 0.6s cubic-bezier(0.34,1.56,0.64,1) ${i * 0.06}s`,
                    }}
                  />
                  <span style={{ fontSize: 8, color: d.h > 0.8 ? color : 'rgba(71,85,105,0.45)' }}>{d.day}</span>
                </div>
              ))}
            </div>
            <div style={{ fontSize: 9, color: 'rgba(71,85,105,0.45)', marginTop: 6, textAlign: 'center' }}>
              周四是你的高峰日，效率比平均高 38%
            </div>
          </div>

          {/* Insights */}
          <div className="flex flex-col gap-2">
            {insights.map((insight, i) => (
              <div
                key={i}
                className="flex items-start gap-2.5"
                style={{
                  opacity: insightsShow ? 1 : 0,
                  transition: `opacity 0.4s ease ${i * 0.1 + 0.3}s`,
                }}
              >
                <div
                  className="flex items-center justify-center rounded-full flex-shrink-0 mt-0.5"
                  style={{
                    width: 20, height: 20,
                    background: insight.emphasis ? `${color}20` : 'rgba(255,255,255,0.04)',
                    border: insight.emphasis ? `1px solid ${color}35` : '1px solid rgba(255,255,255,0.06)',
                  }}
                >
                  <i className={insight.icon} style={{ fontSize: 9, color: insight.emphasis ? color : 'rgba(100,116,139,0.6)' }} />
                </div>
                <p style={{
                  fontSize: 12,
                  color: insight.emphasis ? 'rgba(226,232,240,0.88)' : 'rgba(148,163,184,0.65)',
                  lineHeight: 1.65,
                  fontWeight: insight.emphasis ? 400 : 300,
                }}>
                  {insight.text}
                </p>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Closing note */}
      <div
        className="mt-4 px-3 py-2.5 rounded-xl"
        style={{
          background: `${color}07`,
          border: `1px solid ${color}14`,
          opacity: insightsShow ? 1 : 0,
          transition: `opacity 0.4s ease ${insights.length * 0.1 + 0.2}s`,
        }}
      >
        <p style={{ fontSize: 11, color: 'rgba(148,163,184,0.5)', lineHeight: 1.7, fontStyle: 'italic' }}>
          这只是我的观察，不是评判。如果有什么不准确的，随时告诉我。
        </p>
      </div>
    </div>
  );
}
