import { PrototypeStatus, SampleDefinition, ActionDefinition } from '../model/types';

interface ActionPanelProps {
  status: PrototypeStatus;
  sample: SampleDefinition;
  actions: ActionDefinition[];
  activeActionId: string | null;
  contextMode: 'fresh' | 'from-result';
  onChooseAction: (actionId: string) => void;
  onRetry: () => void;
  onViewReason: () => void;
}

export default function ActionPanel({
  status,
  sample,
  actions,
  activeActionId,
  onChooseAction,
  onRetry,
  onViewReason
}: ActionPanelProps) {
  const isVisible = ['actions', 'processing', 'unsupported', 'error'].includes(status);
  
  if (!isVisible) return null;

  const isSupported = sample.kind !== 'unsupported';
  const isError = status === 'error';

  return (
    <div 
      className={`action-panel ${isSupported ? 'supported' : 'unsupported'}`}
      data-testid="action-panel"
      data-side="right"
    >
      {isError ? (
        <div className="action-panel-error">
          <div className="error-header">未能完成文档识别，请重试</div>
          <div className="error-actions">
            <button className="action-btn retry" onClick={onRetry}>重试</button>
            <button className="action-btn reason" onClick={onViewReason}>查看原因</button>
          </div>
        </div>
      ) : !isSupported ? (
        <div className="action-panel-unsupported">
          <div className="unsupported-header">暂不支持该格式</div>
          <div className="unsupported-hint">可尝试拖入 PDF、图片、文本或链接</div>
        </div>
      ) : (
        <>
          <div className="action-panel-header">
            <div className="recognized-label">已识别：{sample.label}</div>
            <div className="recognized-meta">{sample.meta}</div>
          </div>
          <div className="action-buttons">
            {actions.map((action) => (
              <button
                key={action.id}
                className={`action-btn ${activeActionId === action.id ? 'active' : ''}`}
                onClick={() => onChooseAction(action.id)}
                disabled={status === 'processing'}
              >
                {action.label}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
