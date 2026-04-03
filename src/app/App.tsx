import { useReducer, useEffect } from 'react';
import { prototypeReducer, createInitialState } from './model/reducer';
import { MOTION } from './model/motion';
import { traySamples } from './model/mockData';
import ObjectTray from './components/ObjectTray';
import DesktopStage from './components/DesktopStage';
import FloatingOrb from './components/FloatingOrb';
import ActionPanel from './components/ActionPanel';
import ContextRail from './components/ContextRail';

export default function App() {
  const [state, dispatch] = useReducer(prototypeReducer, createInitialState());

  useEffect(() => {
    if (state.status === 'recognized') {
      const timer = setTimeout(() => {
        dispatch({ type: 'finishRecognition' });
      }, MOTION.recognitionBeatMs);
      return () => clearTimeout(timer);
    }
  }, [state.status]);

  useEffect(() => {
    if (state.status === 'processing') {
      const timer = setTimeout(() => {
        dispatch({ type: 'finishProcessing' });
      }, MOTION.processingBeatMs);
      return () => clearTimeout(timer);
    }
  }, [state.status]);

  const handleSelectSample = (sampleId: string) => {
    dispatch({ type: 'selectSample', sampleId });
  };

  const handleStartDrag = (sampleId: string, sourceType: 'tray' | 'stage', point: { x: number; y: number }) => {
    dispatch({ type: 'startDrag', sourceId: sampleId, sourceType, x: point.x, y: point.y });
  };

  const handleMoveDrag = (point: { x: number; y: number }) => {
    dispatch({ type: 'moveDrag', x: point.x, y: point.y });
    const orb = document.querySelector('[data-testid="floating-orb"]');
    if (orb) {
      const rect = orb.getBoundingClientRect();
      const orbCenter = { x: rect.x + rect.width / 2, y: rect.y + rect.height / 2 };
      const distance = Math.sqrt(Math.pow(point.x - orbCenter.x, 2) + Math.pow(point.y - orbCenter.y, 2));
      if (distance <= 76) {
        dispatch({ type: 'hoverOrb' });
      } else if (distance <= 140) {
        dispatch({ type: 'enterOrbRange' });
      } else {
        dispatch({ type: 'leaveOrbRange' });
      }
    }
  };

  const handleEndDrag = (point: { x: number; y: number }) => {
    const orb = document.querySelector('[data-testid="floating-orb"]');
    if (orb) {
      const rect = orb.getBoundingClientRect();
      const orbCenter = { x: rect.x + rect.width / 2, y: rect.y + rect.height / 2 };
      const distance = Math.sqrt(Math.pow(point.x - orbCenter.x, 2) + Math.pow(point.y - orbCenter.y, 2));
      if (distance <= 76) {
        dispatch({ type: 'dropOnOrb' });
      } else {
        dispatch({ type: 'leaveOrbRange' });
      }
    }
  };

  const handleChooseAction = (actionId: string) => {
    dispatch({ type: 'chooseAction', actionId });
  };

  const handleOpenDetail = () => {
    dispatch({ type: 'openDetail' });
  };

  const handleCloseDetail = () => {
    dispatch({ type: 'closeDetail' });
  };

  const handleContinueFromResult = () => {
    dispatch({ type: 'continueFromResult' });
  };

  const handleTriggerErrorPreview = () => {
    dispatch({ type: 'triggerErrorPreview' });
  };

  const handleRetry = () => {
    dispatch({ type: 'retryAfterError' });
  };

  const handleViewReason = () => {
    dispatch({ type: 'viewErrorReason' });
  };

  const handleRunDemo = () => {
    if (state.activeSample.kind === 'unsupported') {
      dispatch({ type: 'enterOrbRange' });
      setTimeout(() => dispatch({ type: 'hoverOrb' }), 150);
      setTimeout(() => dispatch({ type: 'dropOnOrb' }), 300);
    } else {
      dispatch({ type: 'enterOrbRange' });
      setTimeout(() => dispatch({ type: 'hoverOrb' }), 150);
      setTimeout(() => dispatch({ type: 'dropOnOrb' }), 300);
      setTimeout(() => dispatch({ type: 'finishRecognition' }), 1050);
    }
  };

  return (
    <main className="app-shell">
      <section aria-label="桌面交互主舞台" className="desktop-stage-container">
        <h1>对象即输入</h1>
        <ObjectTray
          samples={traySamples}
          activeSampleId={state.activeSample.id}
          onSelect={handleSelectSample}
          onStartDrag={handleStartDrag}
          onRunDemo={handleRunDemo}
          onTriggerErrorPreview={handleTriggerErrorPreview}
        />
        <DesktopStage
          samples={traySamples}
          activeSampleId={state.activeSample.id}
          status={state.status}
          dragGhost={state.dragGhost}
          activeResult={state.activeResult}
          onStartDrag={handleStartDrag}
          onPointerMove={handleMoveDrag}
          onPointerEnd={handleEndDrag}
        />
        <FloatingOrb
          status={state.status}
          sample={state.activeSample}
          activeActionId={state.activeActionId}
        />
        <ActionPanel
          status={state.status}
          sample={state.activeSample}
          actions={state.suggestedActions}
          activeActionId={state.activeActionId}
          contextMode={state.activeResult ? 'from-result' : 'fresh'}
          onChooseAction={handleChooseAction}
          onRetry={handleRetry}
          onViewReason={handleViewReason}
        />
      </section>
      <aside aria-label="右侧上下文栏" className="context-rail-container">
        <ContextRail
          mode={state.railMode}
          highlightedState={state.highlightedGalleryState}
          activeResult={state.activeResult}
          activeSample={state.activeSample}
          onCloseDetail={handleCloseDetail}
          onTriggerErrorPreview={handleTriggerErrorPreview}
          onContinueFromResult={handleContinueFromResult}
          onOpenDetail={handleOpenDetail}
          onCopyResult={() => {}}
        />
      </aside>
    </main>
  );
}
