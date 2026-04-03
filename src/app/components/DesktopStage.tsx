import { useEffect } from 'react';
import { SampleDefinition } from '../model/types';
import { PrototypeStatus } from '../model/types';

interface DesktopStageProps {
  samples: SampleDefinition[];
  activeSampleId: string;
  status: PrototypeStatus;
  dragGhost: { sourceId: string; sourceType: 'tray' | 'stage'; x: number; y: number; snapped: boolean } | null;
  activeResult: any;
  onStartDrag: (sampleId: string, sourceType: 'tray' | 'stage', point: { x: number; y: number }) => void;
  onPointerMove: (point: { x: number; y: number }) => void;
  onPointerEnd: (point: { x: number; y: number }) => void;
}

const samplePositions: Record<string, { top: string; left: string }> = {
  'product-pdf': { top: '88px', left: '72px' },
  'whiteboard-image': { top: '132px', left: '336px' },
  'meeting-note': { top: '322px', left: '112px' },
  'research-link': { top: '472px', left: '300px' },
  'archive-zip': { top: '248px', left: '560px' }
};

export default function DesktopStage({
  samples,
  activeSampleId,
  status,
  dragGhost,
  onStartDrag,
  onPointerMove,
  onPointerEnd
}: DesktopStageProps) {
  const isActive = (id: string) => id === activeSampleId;

  const handlePointerDown = (sampleId: string, e: React.PointerEvent) => {
    onStartDrag(sampleId, 'stage', { x: e.clientX, y: e.clientY });
  };

  useEffect(() => {
    const handleMove = (e: PointerEvent) => {
      if (dragGhost) {
        onPointerMove({ x: e.clientX, y: e.clientY });
      }
    };
    const handleUp = (e: PointerEvent) => {
      if (dragGhost) {
        onPointerEnd({ x: e.clientX, y: e.clientY });
      }
    };
    window.addEventListener('pointermove', handleMove);
    window.addEventListener('pointerup', handleUp);
    return () => {
      window.removeEventListener('pointermove', handleMove);
      window.removeEventListener('pointerup', handleUp);
    };
  }, [dragGhost, onPointerMove, onPointerEnd]);

  const showDim = ['nearby', 'hover', 'unsupported-nearby', 'unsupported-hover'].includes(status);

  return (
    <div className={`desktop-stage ${showDim ? 'dimmed' : ''}`}>
      {samples.map((sample) => (
        <div
          key={sample.id}
          className={`desktop-sample ${isActive(sample.id) ? 'active' : 'inactive'} ${isActive(sample.id) ? 'active-sample' : ''}`}
          style={samplePositions[sample.id]}
          data-testid={isActive(sample.id) ? 'desktop-active-sample' : undefined}
          data-drag-source={dragGhost ? dragGhost.sourceType : undefined}
          onPointerDown={(e) => handlePointerDown(sample.id, e)}
        >
          <div className="sample-icon">{sample.kind === 'pdf' ? '📄' : sample.kind === 'image' ? '🖼️' : sample.kind === 'text' ? '📝' : sample.kind === 'link' ? '🔗' : '📦'}</div>
          <div className="sample-label">{sample.label}</div>
          {isActive(sample.id) && <div className="sample-badge">待拖入对象</div>}
        </div>
      ))}
      {dragGhost && (
        <div
          className="drag-ghost"
          style={{
            left: dragGhost.x,
            top: dragGhost.y,
            transform: dragGhost.snapped ? 'translate(-50%, -50%) scale(0.95)' : 'translate(-50%, -50%)'
          }}
          data-snapped={dragGhost.snapped}
        >
          {samples.find(s => s.id === dragGhost.sourceId)?.label}
        </div>
      )}
    </div>
  );
}
