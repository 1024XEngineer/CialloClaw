import { PrototypeStatus, SampleDefinition } from '../model/types';

interface FloatingOrbProps {
  status: PrototypeStatus;
  sample: SampleDefinition;
  activeActionId: string | null;
}

export default function FloatingOrb({ status, sample, activeActionId }: FloatingOrbProps) {
  const isSupported = sample.kind !== 'unsupported';
  const eligibility = isSupported ? 'supported' : 'unsupported';
  
  let displayText = '拖入对象';
  if (status === 'nearby' || status === 'unsupported-nearby') {
    displayText = '靠近中';
  } else if (status === 'hover' || status === 'unsupported-hover') {
    displayText = '松手处理';
  } else if (status === 'recognized') {
    displayText = sample.kind === 'unsupported' ? '暂不支持' : '识别中...';
  } else if (status === 'actions' || status === 'processing') {
    displayText = '处理中';
  } else if (status === 'result' || status === 'detail') {
    displayText = '完成';
  } else if (status === 'unsupported' || status === 'error') {
    displayText = '不支持';
  }

  const showWarning = status === 'unsupported-nearby' || status === 'unsupported-hover' || status === 'unsupported';

  return (
    <div
      className={`floating-orb ${status} ${eligibility}`}
      data-testid="floating-orb"
      data-status={status}
      data-eligibility={eligibility}
      aria-label="悬浮球"
    >
      <div className="orb-core">
        <span className="orb-icon">{showWarning ? '⚠️' : '✨'}</span>
      </div>
      <div className="orb-label">{displayText}</div>
    </div>
  );
}
