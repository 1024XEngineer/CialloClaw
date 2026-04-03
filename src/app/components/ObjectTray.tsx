import { SampleDefinition } from '../model/types';

interface ObjectTrayProps {
  samples: SampleDefinition[];
  activeSampleId: string;
  onSelect: (sampleId: string) => void;
  onStartDrag: (sampleId: string, sourceType: 'tray' | 'stage', point: { x: number; y: number }) => void;
  onRunDemo: () => void;
  onTriggerErrorPreview: () => void;
}

export default function ObjectTray({
  samples,
  activeSampleId,
  onSelect,
  onStartDrag,
  onRunDemo,
  onTriggerErrorPreview
}: ObjectTrayProps) {
  const handlePointerDown = (sample: SampleDefinition, e: React.PointerEvent) => {
    onStartDrag(sample.id, 'tray', { x: e.clientX, y: e.clientY });
  };

  const getLabel = (sample: SampleDefinition) => {
    if (sample.kind === 'pdf') return 'PDF';
    if (sample.kind === 'image') return '图片';
    if (sample.kind === 'text') return '文本';
    if (sample.kind === 'link') return '链接';
    if (sample.kind === 'unsupported') return '压缩包';
    return sample.label;
  };

  return (
    <div className="object-tray">
      <div className="tray-header">示例对象托盘</div>
      <div className="tray-items">
        {samples.map((sample) => (
          <button
            key={sample.id}
            className={`tray-item ${sample.id === activeSampleId ? 'active' : ''}`}
            onClick={() => onSelect(sample.id)}
            onPointerDown={(e) => handlePointerDown(sample, e)}
          >
            {getLabel(sample)}
          </button>
        ))}
      </div>
      <div className="tray-controls">
        <button className="demo-button" onClick={onRunDemo}>
          一键演示
        </button>
        <button className="error-button" onClick={onTriggerErrorPreview}>
          模拟识别失败
        </button>
      </div>
    </div>
  );
}
