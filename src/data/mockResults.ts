import type { ActionKind, ResultPayload } from '../types/prototype'

export const mockResults: Record<ActionKind, ResultPayload> = {
  'analyze-error': {
    title: 'Possible null reference in render path',
    summary: ['Cause is likely missing API payload guard', 'Check the state before mapping items'],
    detail: [
      'The selected stack trace points to Dashboard.tsx:84',
      'The failing path reads items[0].name before loading completes',
    ],
  },
  'explain-chart': {
    title: 'Revenue trend rises after Q2 dip',
    summary: ['Growth resumes in Q3', 'April remains the weakest month'],
    detail: ['The highlighted region shows a 22% climb from June to September'],
  },
  'summarize-page': {
    title: 'Release note summary ready',
    summary: ['Three visible product changes', 'One migration warning'],
    detail: ['The page emphasizes search speed, admin filters, and onboarding updates'],
  },
  'generate-reply': {
    title: 'Reply draft prepared',
    summary: ['Acknowledges the request', 'Calls out the next action clearly'],
    detail: ['The mock reply stays concise and professional for work chat or email'],
  },
  'recognize-text': {
    title: 'Text extracted from selection',
    summary: ['Headline and body copied into structured output'],
    detail: ['The result preserves paragraph order and line breaks'],
  },
  'analyze-content': {
    title: 'Selection analyzed',
    summary: ['Detected mixed text and UI elements', 'Best next step is task-specific refinement'],
    detail: ['The area contains enough structure to continue with a specialized action'],
  },
}
