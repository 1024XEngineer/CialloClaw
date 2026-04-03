import { useRef, useState, useCallback, useEffect } from 'react';
import type { AgentStateData, AgentStateKey, ModuleKey } from '@/mocks/agentStates';
import { moduleGroups } from '@/mocks/agentStates';
import type { PlanetConfig } from './PlanetNode';
import FocusCard from './FocusCard';
import ProgressTrack from './ProgressTrack';
import ContextStream from './ContextStream';
import AnomalyLayer from './AnomalyLayer';
import NotepadLayer from './NotepadLayer';
import MirrorLayer from './MirrorLayer';
import SenseLayer from './SenseLayer';

interface DetachedWindowProps {
  id: string;
  planet: PlanetConfig;
  stateData: AgentStateData;
  currentStateKey: AgentStateKey;
  moduleKey: ModuleKey;
  initialX: number;
  initialY: number;
  onClose: (id: string) => void;
  onFocus: (id: string) => void;
  zIndex: number;
}

const stateLabelMap: Record<AgentStateKey, string> = {
  standby: '待机', idle_present: '空闲在场', working: '推进中', highlight: '新进展',
  completing: '接近完成', done: '已完成', error_permission: '缺少权限',
  error_blocked: '步骤阻塞', error_missing_info: '缺少信息',
  notepad_processing: '便签处理', notepad_reminder: '重复提醒', scheduled_task: '定时巡检',
  mirror_summary: '周期总结', mirror_habit: '习惯洞察',
  sense_alert: '系统预警', sense_suggestion: '系统建议',
};

type ResizeDir = 'n' | 's' | 'e' | 'w' | 'ne' | 'nw' | 'se' | 'sw';

export default function DetachedWindow({
  id, planet, stateData, currentStateKey, moduleKey,
  initialX, initialY, onClose, onFocus, zIndex,
}: DetachedWindowProps) {
  const [pos, setPos] = useState({ x: initialX, y: initialY });
  const [size, setSize] = useState({ w: 420, h: 540 });
  const [minimized, setMinimized] = useState(false);
  const [contentVisible, setContentVisible] = useState(false);
  const [phase, setPhase] = useState<'entering' | 'visible' | 'exiting'>('entering');
  const [scanLine, setScanLine] = useState(0);
  const [isResizing, setIsResizing] = useState(false);

  const dragRef = useRef<{ startX: number; startY: number; posX: number; posY: number } | null>(null);
  const resizeRef = useRef<{
    dir: ResizeDir;
    startX: number; startY: number;
    origX: number; origY: number;
    origW: number; origH: number;
  } | null>(null);
  const scanAnimRef = useRef<number>(0);
  const startTimeRef = useRef(Date.now());

  const isTask = stateData.module === 'task';
  const isNotepad = stateData.module === 'notepad';
  const isMirror = stateData.module === 'mirror';
  const isSense = stateData.module === 'sense';

  const moduleStates = moduleGroups.find(g => g.key === moduleKey)?.states ?? [];
  const stateIndex = moduleStates.indexOf(currentStateKey);

  const MIN_W = 320;
  const MIN_H = 280;
  const MAX_W = 800;
  const MAX_H = 900;

  useEffect(() => {
    const t1 = setTimeout(() => setPhase('visible'), 60);
    const t2 = setTimeout(() => setContentVisible(true), 280);
    return () => { clearTimeout(t1); clearTimeout(t2); };
  }, []);

  useEffect(() => {
    const animate = () => {
      const elapsed = (Date.now() - startTimeRef.current) / 1000;
      setScanLine((elapsed * 0.35) % 1);
      scanAnimRef.current = requestAnimationFrame(animate);
    };
    scanAnimRef.current = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(scanAnimRef.current);
  }, []);

  const handleClose = useCallback(() => {
    setPhase('exiting');
    setTimeout(() => onClose(id), 300);
  }, [id, onClose]);

  // ── Title bar drag ──
  const onTitleMouseDown = useCallback((e: React.MouseEvent) => {
    if ((e.target as HTMLElement).closest('button')) return;
    e.preventDefault();
    onFocus(id);
    dragRef.current = { startX: e.clientX, startY: e.clientY, posX: pos.x, posY: pos.y };

    const onMove = (ev: MouseEvent) => {
      if (!dragRef.current) return;
      const dx = ev.clientX - dragRef.current.startX;
      const dy = ev.clientY - dragRef.current.startY;
      setPos({
        x: Math.max(0, Math.min(window.innerWidth - size.w, dragRef.current.posX + dx)),
        y: Math.max(0, Math.min(window.innerHeight - 60, dragRef.current.posY + dy)),
      });
    };
    const onUp = () => {
      dragRef.current = null;
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    };
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
  }, [id, pos, size.w, onFocus]);

  // ── Resize from any edge/corner ──
  const onResizeMouseDown = useCallback((e: React.MouseEvent, dir: ResizeDir) => {
    e.preventDefault();
    e.stopPropagation();
    onFocus(id);
    setIsResizing(true);
    resizeRef.current = {
      dir,
      startX: e.clientX,
      startY: e.clientY,
      origX: pos.x,
      origY: pos.y,
      origW: size.w,
      origH: size.h,
    };

    const onMove = (ev: MouseEvent) => {
      if (!resizeRef.current) return;
      const { dir: d, startX, startY, origX, origY, origW, origH } = resizeRef.current;
      const dx = ev.clientX - startX;
      const dy = ev.clientY - startY;

      let newX = origX, newY = origY, newW = origW, newH = origH;

      if (d.includes('e')) newW = Math.max(MIN_W, Math.min(MAX_W, origW + dx));
      if (d.includes('s')) newH = Math.max(MIN_H, Math.min(MAX_H, origH + dy));
      if (d.includes('w')) {
        const w = Math.max(MIN_W, Math.min(MAX_W, origW - dx));
        newX = origX + (origW - w);
        newW = w;
      }
      if (d.includes('n')) {
        const h = Math.max(MIN_H, Math.min(MAX_H, origH - dy));
        newY = origY + (origH - h);
        newH = h;
      }

      setPos({ x: newX, y: newY });
      setSize({ w: newW, h: newH });
    };

    const onUp = () => {
      resizeRef.current = null;
      setIsResizing(false);
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    };
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
  }, [id, pos, size, onFocus]);

  const entryTransform = phase === 'entering'
    ? 'scale(0.85) translateY(12px)'
    : phase === 'exiting'
    ? 'scale(0.9) translateY(-8px)'
    : 'scale(1) translateY(0)';

  const edgeSize = 5;
  const cornerSize = 12;

  return (
    <div
      style={{
        position: 'fixed',
        left: pos.x,
        top: pos.y,
        width: size.w,
        zIndex,
        pointerEvents: 'auto',
        userSelect: isResizing ? 'none' : 'auto',
      }}
      onMouseDown={() => onFocus(id)}
    >
      <div
        style={{
          opacity: phase === 'entering' ? 0 : phase === 'exiting' ? 0 : 1,
          transform: entryTransform,
          transition: 'opacity 0.3s cubic-bezier(0.16,1,0.3,1), transform 0.3s cubic-bezier(0.16,1,0.3,1)',
          position: 'relative',
        }}
      >
        {/* ── Resize handles — edges ── */}
        {/* Top */}
        <div onMouseDown={e => onResizeMouseDown(e, 'n')} style={{ position: 'absolute', top: -edgeSize, left: cornerSize, right: cornerSize, height: edgeSize * 2, cursor: 'n-resize', zIndex: 40 }} />
        {/* Bottom */}
        <div onMouseDown={e => onResizeMouseDown(e, 's')} style={{ position: 'absolute', bottom: -edgeSize, left: cornerSize, right: cornerSize, height: edgeSize * 2, cursor: 's-resize', zIndex: 40 }} />
        {/* Left */}
        <div onMouseDown={e => onResizeMouseDown(e, 'w')} style={{ position: 'absolute', left: -edgeSize, top: cornerSize, bottom: cornerSize, width: edgeSize * 2, cursor: 'w-resize', zIndex: 40 }} />
        {/* Right */}
        <div onMouseDown={e => onResizeMouseDown(e, 'e')} style={{ position: 'absolute', right: -edgeSize, top: cornerSize, bottom: cornerSize, width: edgeSize * 2, cursor: 'e-resize', zIndex: 40 }} />

        {/* ── Resize handles — corners ── */}
        <div onMouseDown={e => onResizeMouseDown(e, 'nw')} style={{ position: 'absolute', top: -edgeSize, left: -edgeSize, width: cornerSize + edgeSize, height: cornerSize + edgeSize, cursor: 'nw-resize', zIndex: 41 }} />
        <div onMouseDown={e => onResizeMouseDown(e, 'ne')} style={{ position: 'absolute', top: -edgeSize, right: -edgeSize, width: cornerSize + edgeSize, height: cornerSize + edgeSize, cursor: 'ne-resize', zIndex: 41 }} />
        <div onMouseDown={e => onResizeMouseDown(e, 'sw')} style={{ position: 'absolute', bottom: -edgeSize, left: -edgeSize, width: cornerSize + edgeSize, height: cornerSize + edgeSize, cursor: 'sw-resize', zIndex: 41 }} />
        <div onMouseDown={e => onResizeMouseDown(e, 'se')} style={{ position: 'absolute', bottom: -edgeSize, right: -edgeSize, width: cornerSize + edgeSize, height: cornerSize + edgeSize, cursor: 'se-resize', zIndex: 41 }} />

        {/* Corner brackets */}
        {(['tl','tr','bl','br'] as const).map(corner => (
          <div key={corner} className="absolute pointer-events-none" style={{
            top: corner.startsWith('t') ? -1 : undefined,
            bottom: corner.startsWith('b') ? -1 : undefined,
            left: corner.endsWith('l') ? -1 : undefined,
            right: corner.endsWith('r') ? -1 : undefined,
            zIndex: 10,
          }}>
            {corner === 'tl' && <><div style={{ width: 14, height: 2, background: planet.color, opacity: 0.7 }} /><div style={{ width: 2, height: 14, background: planet.color, opacity: 0.7 }} /></>}
            {corner === 'tr' && <><div style={{ width: 14, height: 2, background: planet.color, opacity: 0.7, marginLeft: 'auto' }} /><div style={{ width: 2, height: 14, background: planet.color, opacity: 0.7, marginLeft: 'auto' }} /></>}
            {corner === 'bl' && <><div style={{ width: 2, height: 14, background: planet.color, opacity: 0.7 }} /><div style={{ width: 14, height: 2, background: planet.color, opacity: 0.7 }} /></>}
            {corner === 'br' && <><div style={{ width: 2, height: 14, background: planet.color, opacity: 0.7, marginLeft: 'auto' }} /><div style={{ width: 14, height: 2, background: planet.color, opacity: 0.7, marginLeft: 'auto' }} /></>}
          </div>
        ))}

        {/* Resize indicator when active */}
        {isResizing && (
          <div className="absolute pointer-events-none" style={{
            inset: 0,
            border: `1px solid ${planet.color}50`,
            borderRadius: 4,
            zIndex: 50,
            boxShadow: `0 0 20px ${planet.glow}`,
          }} />
        )}

        <div style={{
          background: 'rgba(3,5,12,0.97)',
          border: `1px solid ${planet.color}28`,
          borderRadius: 4,
          backdropFilter: 'blur(60px)',
          overflow: 'hidden',
          boxShadow: `0 0 0 1px ${planet.color}10 inset, 0 32px 64px rgba(0,0,0,0.9), 0 0 50px ${planet.glow}`,
        }}>
          {/* Scan line */}
          <div className="absolute inset-0 pointer-events-none" style={{
            background: `linear-gradient(to bottom, transparent ${scanLine * 100 - 2}%, ${planet.color}07 ${scanLine * 100}%, transparent ${scanLine * 100 + 3}%)`,
            zIndex: 5,
          }} />

          {/* Grid */}
          <div className="absolute inset-0 pointer-events-none" style={{
            backgroundImage: `repeating-linear-gradient(to bottom, transparent, transparent 39px, ${planet.color}05 39px, ${planet.color}05 40px)`,
            zIndex: 2,
          }} />

          {/* Title bar */}
          <div
            className="flex items-center justify-between px-4 py-2.5 cursor-move select-none"
            style={{
              borderBottom: `1px solid ${planet.color}18`,
              background: `linear-gradient(to right, ${planet.color}08, transparent)`,
              zIndex: 20,
              position: 'relative',
            }}
            onMouseDown={onTitleMouseDown}
          >
            <div className="flex items-center gap-2.5">
              <div className="rounded-full" style={{ width: 6, height: 6, background: planet.color, boxShadow: `0 0 6px ${planet.color}`, animation: 'notifPulse 2s ease-in-out infinite' }} />
              <div className="flex items-center justify-center rounded-full" style={{ width: 22, height: 22, background: `radial-gradient(circle at 35% 30%, ${planet.color}cc 0%, ${planet.color}33 100%)`, border: `1px solid ${planet.color}50` }}>
                <i className={planet.icon} style={{ fontSize: 10, color: 'rgba(255,255,255,0.9)' }} />
              </div>
              <div>
                <div style={{ fontSize: 10, color: planet.color, letterSpacing: '0.2em', fontWeight: 600 }}>
                  {planet.label.toUpperCase()}
                </div>
                <div style={{ fontSize: 8, color: 'rgba(71,85,105,0.5)', letterSpacing: '0.1em' }}>
                  {stateLabelMap[currentStateKey]} · {stateIndex + 1}/{moduleStates.length}
                </div>
              </div>
            </div>

            <div className="flex items-center gap-1.5">
              {/* Size display */}
              <div style={{ fontSize: 8, color: 'rgba(71,85,105,0.35)', letterSpacing: '0.06em', marginRight: 4 }}>
                {size.w} × {size.h}
              </div>
              {/* Minimize */}
              <button
                onClick={() => setMinimized(m => !m)}
                className="flex items-center justify-center rounded cursor-pointer transition-all duration-150"
                style={{ width: 20, height: 20, background: 'rgba(255,255,255,0.04)', border: `1px solid ${planet.color}20`, color: 'rgba(100,116,139,0.6)' }}
                onMouseEnter={e => { (e.currentTarget as HTMLButtonElement).style.background = `${planet.color}18`; }}
                onMouseLeave={e => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(255,255,255,0.04)'; }}
              >
                <i className={minimized ? 'ri-add-line' : 'ri-subtract-line'} style={{ fontSize: 10 }} />
              </button>
              {/* Close */}
              <button
                onClick={handleClose}
                className="flex items-center justify-center rounded cursor-pointer transition-all duration-150"
                style={{ width: 20, height: 20, background: 'rgba(255,255,255,0.04)', border: `1px solid ${planet.color}20`, color: 'rgba(100,116,139,0.6)' }}
                onMouseEnter={e => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(251,113,133,0.18)'; (e.currentTarget as HTMLButtonElement).style.color = '#fb7185'; }}
                onMouseLeave={e => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(255,255,255,0.04)'; (e.currentTarget as HTMLButtonElement).style.color = 'rgba(100,116,139,0.6)'; }}
              >
                <i className="ri-close-line" style={{ fontSize: 10 }} />
              </button>
            </div>
          </div>

          {/* Content */}
          {!minimized && (
            <div style={{ height: size.h - 44, overflowY: 'auto', position: 'relative', zIndex: 10 }}>
              <div className="px-4 pt-3 pb-4 flex flex-col gap-3.5">
                <FocusCard stateData={stateData} visible={contentVisible} />
                {isTask && stateData.progressSteps && stateData.progressSteps.length > 0 && (
                  <><div style={{ height: 1, background: `${planet.color}12` }} /><ProgressTrack stateData={stateData} visible={contentVisible} /></>
                )}
                {isTask && stateData.context.length > 0 && (
                  <><div style={{ height: 1, background: `${planet.color}12` }} /><ContextStream stateData={stateData} visible={contentVisible} /></>
                )}
                {isNotepad && (
                  <>
                    <div style={{ height: 1, background: `${planet.color}12` }} />
                    <NotepadLayer stateData={stateData} visible={contentVisible} />
                    {stateData.context.length > 0 && <><div style={{ height: 1, background: `${planet.color}12` }} /><ContextStream stateData={stateData} visible={contentVisible} /></>}
                  </>
                )}
                {isMirror && (
                  <><div style={{ height: 1, background: `${planet.color}12` }} /><MirrorLayer stateData={stateData} visible={contentVisible} /></>
                )}
                {isSense && (
                  <>
                    <div style={{ height: 1, background: `${planet.color}12` }} />
                    <SenseLayer stateData={stateData} visible={contentVisible} />
                    {stateData.context.length > 0 && <><div style={{ height: 1, background: `${planet.color}12` }} /><ContextStream stateData={stateData} visible={contentVisible} /></>}
                  </>
                )}
                {stateData.anomaly && (
                  <AnomalyLayer stateData={stateData} visible={contentVisible} onAction={() => {}} onDismiss={() => {}} />
                )}
                <div className="flex items-center justify-center gap-2 mt-1" style={{ opacity: 0.2 }}>
                  <div className="flex-1 h-px" style={{ background: `linear-gradient(to right, transparent, ${planet.color}30)` }} />
                  <span style={{ fontSize: 8, color: planet.color, letterSpacing: '0.2em' }}>DETACHED · {planet.label.toUpperCase()}</span>
                  <div className="flex-1 h-px" style={{ background: `linear-gradient(to left, transparent, ${planet.color}30)` }} />
                </div>
              </div>
            </div>
          )}

          <div style={{ height: 2, background: `linear-gradient(to right, transparent, ${planet.color}55, transparent)`, opacity: 0.5 }} />
        </div>
      </div>
    </div>
  );
}
