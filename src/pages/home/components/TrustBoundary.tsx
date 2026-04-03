import { useEffect, useState } from 'react';

interface PermissionItem {
  id: string;
  name: string;
  category: 'file' | 'network' | 'system' | 'data';
  status: 'granted' | 'denied' | 'pending';
  lastUsed?: string;
}

interface TrustBoundaryProps {
  visible: boolean;
  onClose: () => void;
}

const PERMISSIONS: PermissionItem[] = [
  { id: '1', name: '访问本地文件', category: 'file', status: 'granted', lastUsed: '2分钟前' },
  { id: '2', name: '读取剪贴板', category: 'file', status: 'granted', lastUsed: '刚刚' },
  { id: '3', name: '网络请求', category: 'network', status: 'granted', lastUsed: '进行中' },
  { id: '4', name: '执行系统命令', category: 'system', status: 'denied' },
  { id: '5', name: '访问摄像头', category: 'system', status: 'pending' },
  { id: '6', name: '读取日历', category: 'data', status: 'granted', lastUsed: '1小时前' },
  { id: '7', name: '访问联系人', category: 'data', status: 'denied' },
];

const TOKEN_USAGE = {
  today: 2847,
  limit: 5000,
  history: [
    { date: '周一', used: 2100 },
    { date: '周二', used: 3200 },
    { date: '周三', used: 1800 },
    { date: '周四', used: 2847 },
  ],
};

const categoryIcons: Record<PermissionItem['category'], string> = {
  file: 'ri-folder-3-line',
  network: 'ri-wifi-line',
  system: 'ri-computer-line',
  data: 'ri-database-2-line',
};

const categoryLabels: Record<PermissionItem['category'], string> = {
  file: '文件访问',
  network: '网络',
  system: '系统',
  data: '数据',
};

const statusConfig = {
  granted: { color: '#34d399', bg: 'rgba(52,211,153,0.12)', label: '已授权' },
  denied: { color: '#fb7185', bg: 'rgba(251,113,133,0.12)', label: '已拒绝' },
  pending: { color: '#fbbf24', bg: 'rgba(251,191,36,0.12)', label: '待确认' },
};

export default function TrustBoundary({ visible, onClose }: TrustBoundaryProps) {
  const [show, setShow] = useState(false);
  const [activeTab, setActiveTab] = useState<'permissions' | 'tokens'>('permissions');
  const [hoveredPerm, setHoveredPerm] = useState<string | null>(null);

  useEffect(() => {
    if (visible) {
      const t = setTimeout(() => setShow(true), 50);
      return () => clearTimeout(t);
    }
    setShow(false);
  }, [visible]);

  if (!visible) return null;

  const grantedCount = PERMISSIONS.filter(p => p.status === 'granted').length;
  const deniedCount = PERMISSIONS.filter(p => p.status === 'denied').length;
  const pendingCount = PERMISSIONS.filter(p => p.status === 'pending').length;

  const tokenPercent = (TOKEN_USAGE.today / TOKEN_USAGE.limit) * 100;
  const tokenColor = tokenPercent > 80 ? '#fb7185' : tokenPercent > 50 ? '#fbbf24' : '#34d399';

  return (
    <div
      className="fixed inset-0 flex items-center justify-center pointer-events-auto"
      style={{ zIndex: 250, background: 'rgba(3,5,12,0.9)', backdropFilter: 'blur(12px)' }}
      onClick={onClose}
    >
      <div
        className="relative w-full max-w-2xl mx-4"
        style={{
          opacity: show ? 1 : 0,
          transform: show ? 'scale(1) translateY(0)' : 'scale(0.95) translateY(20px)',
          transition: 'all 0.4s cubic-bezier(0.16,1,0.3,1)',
        }}
        onClick={e => e.stopPropagation()}
      >
        {/* Corner brackets */}
        <div className="absolute -top-1 -left-1" style={{ zIndex: 10 }}>
          <div style={{ width: 20, height: 2, background: '#64748b' }} />
          <div style={{ width: 2, height: 20, background: '#64748b' }} />
        </div>
        <div className="absolute -top-1 -right-1" style={{ zIndex: 10 }}>
          <div style={{ width: 20, height: 2, background: '#64748b', marginLeft: 'auto' }} />
          <div style={{ width: 2, height: 20, background: '#64748b', marginLeft: 'auto' }} />
        </div>
        <div className="absolute -bottom-1 -left-1" style={{ zIndex: 10 }}>
          <div style={{ width: 2, height: 20, background: '#64748b' }} />
          <div style={{ width: 20, height: 2, background: '#64748b' }} />
        </div>
        <div className="absolute -bottom-1 -right-1" style={{ zIndex: 10 }}>
          <div style={{ width: 2, height: 20, background: '#64748b', marginLeft: 'auto' }} />
          <div style={{ width: 20, height: 2, background: '#64748b', marginLeft: 'auto' }} />
        </div>

        {/* Main panel */}
        <div
          style={{
            background: 'rgba(3,5,12,0.98)',
            border: '1px solid rgba(100,116,139,0.2)',
            borderRadius: 4,
            overflow: 'hidden',
            boxShadow: '0 0 0 1px rgba(100,116,139,0.1) inset, 0 40px 80px rgba(0,0,0,0.9)',
          }}
        >
          {/* Header */}
          <div
            className="flex items-center justify-between px-5 py-4"
            style={{ borderBottom: '1px solid rgba(100,116,139,0.15)' }}
          >
            <div className="flex items-center gap-3">
              <div
                className="flex items-center justify-center rounded-full"
                style={{
                  width: 32,
                  height: 32,
                  background: 'rgba(100,116,139,0.15)',
                  border: '1px solid rgba(100,116,139,0.3)',
                }}
              >
                <i className="ri-shield-check-line" style={{ fontSize: 14, color: '#94a3b8' }} />
              </div>
              <div>
                <div style={{ fontSize: 13, color: '#94a3b8', letterSpacing: '0.15em', fontWeight: 600 }}>
                  信任与边界
                </div>
                <div style={{ fontSize: 10, color: 'rgba(71,85,105,0.6)', marginTop: 2 }}>
                  TRUST & BOUNDARY
                </div>
              </div>
            </div>

            <button
              onClick={onClose}
              className="flex items-center justify-center rounded cursor-pointer transition-all duration-200"
              style={{ width: 28, height: 28, background: 'rgba(255,255,255,0.04)', border: '1px solid rgba(100,116,139,0.2)', color: 'rgba(100,116,139,0.7)' }}
              onMouseEnter={e => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(251,113,133,0.15)'; (e.currentTarget as HTMLButtonElement).style.color = '#fb7185'; }}
              onMouseLeave={e => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(255,255,255,0.04)'; (e.currentTarget as HTMLButtonElement).style.color = 'rgba(100,116,139,0.7)'; }}
            >
              <i className="ri-close-line" style={{ fontSize: 14 }} />
            </button>
          </div>

          {/* Tab switcher */}
          <div className="flex items-center px-5 py-3" style={{ borderBottom: '1px solid rgba(100,116,139,0.1)' }}>
            {(['permissions', 'tokens'] as const).map(tab => (
              <button
                key={tab}
                onClick={() => setActiveTab(tab)}
                className="px-4 py-2 rounded cursor-pointer transition-all duration-200"
                style={{
                  background: activeTab === tab ? 'rgba(100,116,139,0.15)' : 'transparent',
                  color: activeTab === tab ? '#94a3b8' : 'rgba(71,85,105,0.6)',
                  fontSize: 11,
                  letterSpacing: '0.1em',
                  marginRight: 8,
                }}
              >
                {tab === 'permissions' ? '红绿灯授权' : 'Token 消耗'}
              </button>
            ))}
          </div>

          {/* Content */}
          <div className="p-5" style={{ minHeight: 360 }}>
            {activeTab === 'permissions' ? (
              <>
                {/* Traffic light summary */}
                <div className="flex items-center gap-4 mb-5">
                  <div
                    className="flex-1 flex items-center gap-2 px-3 py-2.5 rounded-lg"
                    style={{ background: 'rgba(52,211,153,0.08)', border: '1px solid rgba(52,211,153,0.2)' }}
                  >
                    <div className="rounded-full" style={{ width: 8, height: 8, background: '#34d399', boxShadow: '0 0 8px #34d399' }} />
                    <span style={{ fontSize: 11, color: '#34d399' }}>{grantedCount} 已授权</span>
                  </div>
                  <div
                    className="flex-1 flex items-center gap-2 px-3 py-2.5 rounded-lg"
                    style={{ background: 'rgba(251,191,36,0.08)', border: '1px solid rgba(251,191,36,0.2)' }}
                  >
                    <div className="rounded-full" style={{ width: 8, height: 8, background: '#fbbf24', boxShadow: '0 0 8px #fbbf24' }} />
                    <span style={{ fontSize: 11, color: '#fbbf24' }}>{pendingCount} 待确认</span>
                  </div>
                  <div
                    className="flex-1 flex items-center gap-2 px-3 py-2.5 rounded-lg"
                    style={{ background: 'rgba(251,113,133,0.08)', border: '1px solid rgba(251,113,133,0.2)' }}
                  >
                    <div className="rounded-full" style={{ width: 8, height: 8, background: '#fb7185', boxShadow: '0 0 8px #fb7185' }} />
                    <span style={{ fontSize: 11, color: '#fb7185' }}>{deniedCount} 已拒绝</span>
                  </div>
                </div>

                {/* Permission list */}
                <div className="flex flex-col gap-2">
                  {PERMISSIONS.map((perm, i) => {
                    const cfg = statusConfig[perm.status];
                    const isHovered = hoveredPerm === perm.id;
                    return (
                      <div
                        key={perm.id}
                        className="flex items-center justify-between px-3 py-2.5 rounded-lg cursor-pointer transition-all duration-200"
                        style={{
                          background: isHovered ? 'rgba(255,255,255,0.04)' : 'transparent',
                          border: '1px solid rgba(255,255,255,0.06)',
                          opacity: show ? 1 : 0,
                          transform: show ? 'translateX(0)' : 'translateX(-10px)',
                          transition: `all 0.3s ease ${i * 0.05}s`,
                        }}
                        onMouseEnter={() => setHoveredPerm(perm.id)}
                        onMouseLeave={() => setHoveredPerm(null)}
                      >
                        <div className="flex items-center gap-3">
                          <div
                            className="flex items-center justify-center rounded"
                            style={{
                              width: 28,
                              height: 28,
                              background: 'rgba(255,255,255,0.04)',
                              border: '1px solid rgba(255,255,255,0.08)',
                            }}
                          >
                            <i className={categoryIcons[perm.category]} style={{ fontSize: 12, color: 'rgba(148,163,184,0.6)' }} />
                          </div>
                          <div>
                            <div style={{ fontSize: 12, color: 'rgba(226,232,240,0.85)' }}>{perm.name}</div>
                            <div style={{ fontSize: 9, color: 'rgba(71,85,105,0.5)', marginTop: 1 }}>
                              {categoryLabels[perm.category]}
                              {perm.lastUsed && ` · ${perm.lastUsed}`}
                            </div>
                          </div>
                        </div>

                        <div
                          className="flex items-center gap-1.5 px-2.5 py-1 rounded-full"
                          style={{ background: cfg.bg }}
                        >
                          <div className="rounded-full" style={{ width: 5, height: 5, background: cfg.color }} />
                          <span style={{ fontSize: 9.5, color: cfg.color, letterSpacing: '0.05em' }}>{cfg.label}</span>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </>
            ) : (
              <>
                {/* Token usage card */}
                <div
                  className="p-4 rounded-lg mb-5"
                  style={{ background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.06)' }}
                >
                  <div className="flex items-center justify-between mb-4">
                    <div>
                      <div style={{ fontSize: 10, color: 'rgba(71,85,105,0.6)', letterSpacing: '0.1em', marginBottom: 4 }}>
                        今日 Token 消耗
                      </div>
                      <div className="flex items-baseline gap-2">
                        <span style={{ fontSize: 28, color: tokenColor, fontWeight: 700, letterSpacing: '-0.02em' }}>
                          {TOKEN_USAGE.today.toLocaleString()}
                        </span>
                        <span style={{ fontSize: 11, color: 'rgba(71,85,105,0.5)' }}>
                          / {TOKEN_USAGE.limit.toLocaleString()}
                        </span>
                      </div>
                    </div>
                    <div
                      className="flex items-center justify-center rounded-full"
                      style={{
                        width: 56,
                        height: 56,
                        background: `conic-gradient(${tokenColor} ${tokenPercent * 3.6}deg, rgba(255,255,255,0.05) 0deg)`,
                        position: 'relative',
                      }}
                    >
                      <div
                        className="rounded-full flex items-center justify-center"
                        style={{
                          width: 48,
                          height: 48,
                          background: 'rgba(3,5,12,0.98)',
                          position: 'absolute',
                        }}
                      >
                        <span style={{ fontSize: 12, color: tokenColor, fontWeight: 600 }}>
                          {Math.round(tokenPercent)}%
                        </span>
                      </div>
                    </div>
                  </div>

                  {/* Progress bar */}
                  <div className="relative" style={{ height: 6, background: 'rgba(255,255,255,0.05)', borderRadius: 3 }}>
                    <div
                      style={{
                        position: 'absolute',
                        left: 0,
                        top: 0,
                        height: '100%',
                        width: `${tokenPercent}%`,
                        background: `linear-gradient(to right, ${tokenColor}60, ${tokenColor})`,
                        borderRadius: 3,
                        boxShadow: `0 0 10px ${tokenColor}40`,
                      }}
                    />
                    {/* Warning markers */}
                    <div
                      className="absolute top-0 bottom-0"
                      style={{ left: '50%', width: 1, background: 'rgba(251,191,36,0.3)' }}
                    />
                    <div
                      className="absolute top-0 bottom-0"
                      style={{ left: '80%', width: 1, background: 'rgba(251,113,133,0.3)' }}
                    />
                  </div>
                  <div className="flex justify-between mt-2">
                    <span style={{ fontSize: 9, color: 'rgba(71,85,105,0.4)' }}>0</span>
                    <span style={{ fontSize: 9, color: 'rgba(251,191,36,0.5)' }}>50%</span>
                    <span style={{ fontSize: 9, color: 'rgba(251,113,133,0.5)' }}>80%</span>
                    <span style={{ fontSize: 9, color: 'rgba(71,85,105,0.4)' }}>100%</span>
                  </div>
                </div>

                {/* History chart */}
                <div>
                  <div style={{ fontSize: 10, color: 'rgba(71,85,105,0.6)', letterSpacing: '0.1em', marginBottom: 12 }}>
                    近 4 日消耗趋势
                  </div>
                  <div className="flex items-end gap-3" style={{ height: 120 }}>
                    {TOKEN_USAGE.history.map((day, i) => {
                      const height = (day.used / TOKEN_USAGE.limit) * 100;
                      const dayColor = height > 80 ? '#fb7185' : height > 50 ? '#fbbf24' : '#34d399';
                      return (
                        <div key={day.date} className="flex-1 flex flex-col items-center gap-2">
                          <div
                            style={{
                              width: '100%',
                              height: `${Math.min(height, 100)}%`,
                              background: `linear-gradient(to top, ${dayColor}40, ${dayColor})`,
                              borderRadius: '2px 2px 0 0',
                              opacity: show ? 1 : 0,
                              transition: `opacity 0.4s ease ${i * 0.1}s, height 0.6s cubic-bezier(0.34,1.56,0.64,1) ${i * 0.1}s`,
                              boxShadow: height > 80 ? `0 0 12px ${dayColor}40` : 'none',
                            }}
                          />
                          <span style={{ fontSize: 9, color: 'rgba(71,85,105,0.5)' }}>{day.date}</span>
                          <span style={{ fontSize: 8, color: dayColor }}>{(day.used / 1000).toFixed(1)}k</span>
                        </div>
                      );
                    })}
                  </div>
                </div>
              </>
            )}
          </div>

          {/* Footer */}
          <div
            className="px-5 py-3 flex items-center justify-between"
            style={{ borderTop: '1px solid rgba(100,116,139,0.1)' }}
          >
            <div className="flex items-center gap-2" style={{ opacity: 0.4 }}>
              <i className="ri-shield-keyhole-line" style={{ fontSize: 10, color: '#64748b' }} />
              <span style={{ fontSize: 9, color: 'rgba(100,116,139,0.6)', letterSpacing: '0.1em' }}>
                所有权限可随时在设置中调整
              </span>
            </div>
            <button
              className="px-4 py-1.5 rounded cursor-pointer transition-all duration-200"
              style={{
                background: 'rgba(100,116,139,0.15)',
                border: '1px solid rgba(100,116,139,0.3)',
                color: '#94a3b8',
                fontSize: 10,
                letterSpacing: '0.1em',
              }}
              onMouseEnter={e => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(100,116,139,0.25)'; }}
              onMouseLeave={e => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(100,116,139,0.15)'; }}
            >
              管理设置
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}