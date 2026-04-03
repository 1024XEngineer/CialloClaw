import { galleryItems } from '../model/mockData';
import { SampleDefinition, ResultDefinition } from '../model/types';

interface ContextRailProps {
  mode: 'gallery' | 'detail';
  highlightedState: string;
  activeResult: ResultDefinition | null;
  activeSample: SampleDefinition;
  onCloseDetail: () => void;
  onTriggerErrorPreview: () => void;
  onContinueFromResult: () => void;
  onOpenDetail: () => void;
  onCopyResult: () => void;
}

export default function ContextRail({
  mode,
  highlightedState,
  activeResult,
  activeSample,
  onCloseDetail,
  onTriggerErrorPreview,
  onContinueFromResult,
  onOpenDetail,
  onCopyResult
}: ContextRailProps) {
  if (mode === 'detail' && activeResult) {
    return (
      <div className="detail-view">
        <div className="detail-header">
          <h2>完整结果</h2>
          <button className="close-btn" onClick={onCloseDetail}>关闭详情</button>
        </div>
        <div className="detail-info">
          <div className="detail-sample-label">{activeSample.label}</div>
          <div className="detail-sample-meta">{activeSample.meta}</div>
          <span className="detail-sample-type">{activeSample.kind}</span>
        </div>
        <div className="detail-content">
          <h3>{activeResult.title}</h3>
          <p>{activeResult.body}</p>
        </div>
        <div className="detail-actions">
          <button className="detail-action-btn" onClick={onContinueFromResult}>继续处理</button>
          <button className="detail-action-btn" onClick={onCopyResult}>复制结果</button>
        </div>
      </div>
    );
  }

  if (activeResult) {
    return (
      <div className="result-rail">
        <div className="result-card">
          <h3>{activeResult.title}</h3>
          <p>{activeResult.body}</p>
          <div className="result-actions">
            {activeResult.actions.map((action) => (
              <button
                key={action.id}
                className="result-action-btn"
                onClick={action.id === 'expand' ? onOpenDetail : action.id === 'copy' ? onCopyResult : onContinueFromResult}
              >
                {action.label}
              </button>
            ))}
          </div>
        </div>
        <div className="state-gallery">
          <div className="gallery-header">状态画廊</div>
          {galleryItems.map((item) => (
            <div
              key={item.id}
              className={`gallery-item ${item.id === highlightedState ? 'active' : ''}`}
              data-testid={item.id}
              data-active={item.id === highlightedState ? 'true' : 'false'}
            >
              <div className="gallery-item-title">{item.title}</div>
              <div className="gallery-item-desc">{item.description}</div>
            </div>
          ))}
          <div className="gallery-footer">
            <button className="gallery-error-btn" onClick={onTriggerErrorPreview}>
              模拟识别失败
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="state-gallery">
      <div className="gallery-header">状态画廊</div>
      {galleryItems.map((item) => (
        <div
          key={item.id}
          className={`gallery-item ${item.id === highlightedState ? 'active' : ''}`}
          data-testid={item.id}
          data-active={item.id === highlightedState ? 'true' : 'false'}
        >
          <div className="gallery-item-title">{item.title}</div>
          <div className="gallery-item-desc">{item.description}</div>
        </div>
      ))}
      <div className="gallery-footer">
        <button className="gallery-error-btn" onClick={onTriggerErrorPreview}>
          模拟识别失败
        </button>
      </div>
    </div>
  );
}
