import { samples } from './mockData';
import { createInitialState, prototypeReducer } from './reducer';

describe('prototypeReducer', () => {
  it('moves a pdf sample from drop to actions', () => {
    let state = createInitialState();
    state = prototypeReducer(state, { type: 'selectSample', sampleId: 'product-pdf' });
    state = prototypeReducer(state, { type: 'enterOrbRange' });
    state = prototypeReducer(state, { type: 'hoverOrb' });
    state = prototypeReducer(state, { type: 'dropOnOrb' });

    expect(state.status).toBe('recognized');
    expect(state.activeSample.id).toBe('product-pdf');
    expect(state.recognitionLabel).toBe('识别为 PDF');
  });

  it('advances from recognized to actions after the recognition beat', () => {
    let state = createInitialState();
    state = prototypeReducer(state, { type: 'selectSample', sampleId: 'product-pdf' });
    state = prototypeReducer(state, { type: 'enterOrbRange' });
    state = prototypeReducer(state, { type: 'hoverOrb' });
    state = prototypeReducer(state, { type: 'dropOnOrb' });
    state = prototypeReducer(state, { type: 'finishRecognition' });

    expect(state.status).toBe('actions');
    expect(state.suggestedActions[0].label).toBe('总结 PDF');
  });

  it('moves the zip sample to unsupported on release', () => {
    let state = createInitialState();
    state = prototypeReducer(state, { type: 'selectSample', sampleId: 'archive-zip' });
    state = prototypeReducer(state, { type: 'enterOrbRange' });

    expect(state.status).toBe('unsupported-nearby');

    state = prototypeReducer(state, { type: 'hoverOrb' });
    state = prototypeReducer(state, { type: 'dropOnOrb' });

    expect(state.status).toBe('unsupported');
    expect(state.suggestedActions).toHaveLength(0);
  });

  it('returns to idle when the drag leaves the orb range', () => {
    let state = createInitialState();
    state = prototypeReducer(state, { type: 'enterOrbRange' });
    state = prototypeReducer(state, { type: 'hoverOrb' });
    state = prototypeReducer(state, { type: 'leaveOrbRange' });

    expect(state.status).toBe('idle');
  });

  it('resets the scene while preserving the current sample contract', () => {
    let state = createInitialState(samples['whiteboard-image']);
    state = prototypeReducer(state, { type: 'enterOrbRange' });
    state = prototypeReducer(state, { type: 'resetScene' });

    expect(state.status).toBe('idle');
    expect(state.activeSample.id).toBe('whiteboard-image');
    expect(state.suggestedActions[0].id).toBe('ocr');
  });

  it('switches the rail into detail mode and back', () => {
    let state = createInitialState();
    state = prototypeReducer(state, { type: 'selectSample', sampleId: 'product-pdf' });
    state = prototypeReducer(state, { type: 'enterOrbRange' });
    state = prototypeReducer(state, { type: 'hoverOrb' });
    state = prototypeReducer(state, { type: 'dropOnOrb' });
    state = prototypeReducer(state, { type: 'finishRecognition' });
    state = prototypeReducer(state, { type: 'chooseAction', actionId: 'summary' });
    state = prototypeReducer(state, { type: 'finishProcessing' });
    state = prototypeReducer(state, { type: 'openDetail' });

    expect(state.status).toBe('detail');
    expect(state.railMode).toBe('detail');

    state = prototypeReducer(state, { type: 'closeDetail' });

    expect(state.status).toBe('result');
    expect(state.railMode).toBe('gallery');
  });

  it('returns to actions with result context when continuing from a result card', () => {
    let state = createInitialState(samples['product-pdf']);
    state = prototypeReducer(state, { type: 'chooseAction', actionId: 'summary' });
    state = prototypeReducer(state, { type: 'finishProcessing' });
    state = prototypeReducer(state, { type: 'continueFromResult' });

    expect(state.status).toBe('actions');
    expect(state.activeResult?.title).toBe('PDF 总结');
    expect(state.highlightedGalleryState).toBe('gallery-item-actions');
  });
});
