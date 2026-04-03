import { useEffect } from 'react'

import { processingStages } from '../state/prototypeReducer'
import type { ProcessingStage } from '../types/prototype'

const stageDelays = [0, 300, 700] as const
const completionDelay = 1100

export function useMockProcessing(
  active: boolean,
  onStageChange: (stage: ProcessingStage) => void,
  onComplete: () => void,
) {
  useEffect(() => {
    if (!active) {
      return undefined
    }

    const stageTimeouts = processingStages.map((stage, index) =>
      window.setTimeout(() => {
        onStageChange(stage)
      }, stageDelays[index]),
    )

    const completionTimeout = window.setTimeout(() => {
      onComplete()
    }, completionDelay)

    return () => {
      stageTimeouts.forEach(window.clearTimeout)
      window.clearTimeout(completionTimeout)
    }
  }, [active, onComplete, onStageChange])
}
