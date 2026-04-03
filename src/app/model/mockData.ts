import { SampleDefinition, ActionDefinition, ResultDefinition, GalleryItem } from './types';

export const samples: Record<string, SampleDefinition> = {
  'product-pdf': {
    id: 'product-pdf',
    label: '产品方案.pdf',
    kind: 'pdf',
    meta: '12 页 PDF，4.8 MB'
  },
  'whiteboard-image': {
    id: 'whiteboard-image',
    label: '白板拍照.png',
    kind: 'image',
    meta: '2480 x 1640，2.1 MB'
  },
  'meeting-note': {
    id: 'meeting-note',
    label: '会议摘录.txt',
    kind: 'text',
    meta: '文本文件',
    preview: '下周一完成首页方案评审 / 确认 OCR 体验文案'
  },
  'research-link': {
    id: 'research-link',
    label: 'https://example.com/ai-desktop-workflow',
    kind: 'link',
    meta: 'example.com，网页链接'
  },
  'archive-zip': {
    id: 'archive-zip',
    label: '项目资料.zip',
    kind: 'unsupported',
    meta: '18.2 MB，压缩文件'
  }
};

export const actionMap: Record<string, ActionDefinition[]> = {
  pdf: [
    { id: 'summary', label: '总结 PDF' },
    { id: 'extract', label: '提取重点' },
    { id: 'qa', label: '生成问答' },
    { id: 'organize', label: '整理内容' }
  ],
  image: [
    { id: 'ocr', label: 'OCR 图片' },
    { id: 'extract', label: '提取重点' },
    { id: 'organize', label: '整理内容' }
  ],
  text: [
    { id: 'translate', label: '翻译文本' },
    { id: 'extract', label: '提取重点' },
    { id: 'organize', label: '整理内容' }
  ],
  link: [
    { id: 'brief', label: '提炼网页要点' },
    { id: 'link-summary', label: '生成摘要' },
    { id: 'organize', label: '整理内容' }
  ]
};

export const recognitionMap: Record<string, { title: string; summary: string }> = {
  'product-pdf': { title: '识别为 PDF', summary: '产品方案.pdf · 12 页 PDF' },
  'whiteboard-image': { title: '识别为图片', summary: '白板拍照.png · 2480 x 1640' },
  'meeting-note': { title: '识别为文本', summary: '下周一完成首页方案评审 / 确认 OCR 体验文案' },
  'research-link': { title: '识别为链接', summary: 'example.com · 网页链接' },
  'archive-zip': { title: '暂不支持该格式', summary: '项目资料.zip · 压缩文件' }
};

export const resultMap: Record<string, Record<string, ResultDefinition>> = {
  'product-pdf': {
    summary: {
      title: 'PDF 总结',
      body: '3 条重点 + 1 段摘要',
      actions: [
        { id: 'expand', label: '展开详情' },
        { id: 'copy', label: '复制结果' },
        { id: 'continue', label: '继续处理' }
      ]
    },
    extract: {
      title: '重点提取',
      body: '重点 1 / 重点 2 / 重点 3',
      actions: [
        { id: 'copy', label: '复制结果' },
        { id: 'organize', label: '整理内容' }
      ]
    },
    qa: {
      title: '示例问答',
      body: '3 组问答卡片',
      actions: [
        { id: 'expand', label: '展开详情' },
        { id: 'continue', label: '继续处理' }
      ]
    },
    organize: {
      title: '整理内容',
      body: '分组小节 + 待办清单',
      actions: [
        { id: 'copy', label: '复制结果' },
        { id: 'expand', label: '展开详情' }
      ]
    }
  },
  'whiteboard-image': {
    ocr: {
      title: '图片文字提取',
      body: '识别文本预览 + 关键信息块',
      actions: [
        { id: 'copy', label: '复制结果' },
        { id: 'organize', label: '整理内容' }
      ]
    },
    extract: {
      title: '重点提取',
      body: '重点 1 / 重点 2 / 重点 3',
      actions: [
        { id: 'copy', label: '复制结果' },
        { id: 'organize', label: '整理内容' }
      ]
    },
    organize: {
      title: '整理内容',
      body: '分组小节 + 待办清单',
      actions: [
        { id: 'copy', label: '复制结果' },
        { id: 'expand', label: '展开详情' }
      ]
    }
  },
  'meeting-note': {
    translate: {
      title: '文本翻译',
      body: '原文 / 译文 / 语言标签',
      actions: [
        { id: 'copy', label: '复制译文' },
        { id: 'extract', label: '提取重点' }
      ]
    },
    extract: {
      title: '重点提取',
      body: '重点 1 / 重点 2 / 重点 3',
      actions: [
        { id: 'copy', label: '复制结果' },
        { id: 'organize', label: '整理内容' }
      ]
    },
    organize: {
      title: '整理内容',
      body: '分组小节 + 待办清单',
      actions: [
        { id: 'copy', label: '复制结果' },
        { id: 'expand', label: '展开详情' }
      ]
    }
  },
  'research-link': {
    brief: {
      title: '网页要点',
      body: '3 条网页要点 + 来源域名',
      actions: [
        { id: 'expand', label: '展开详情' },
        { id: 'organize', label: '整理内容' }
      ]
    },
    'link-summary': {
      title: '网页摘要',
      body: '1 段摘要 + 2 条重点',
      actions: [
        { id: 'expand', label: '展开详情' },
        { id: 'organize', label: '整理内容' }
      ]
    },
    organize: {
      title: '整理内容',
      body: '分组小节 + 待办清单',
      actions: [
        { id: 'copy', label: '复制结果' },
        { id: 'expand', label: '展开详情' }
      ]
    }
  }
};

export const galleryItems: GalleryItem[] = [
  { id: 'gallery-item-idle', title: '默认态', description: '悬浮球等待对象输入' },
  { id: 'gallery-item-nearby', title: '靠近高亮态', description: '对象进入接收范围' },
  { id: 'gallery-item-hover', title: '拖拽悬停态', description: '松手即可开始识别' },
  { id: 'gallery-item-recognized', title: '对象识别态', description: '展示类型与对象摘要' },
  { id: 'gallery-item-actions', title: '动作选择面板', description: '按对象类型给出快捷动作' },
  { id: 'gallery-item-processing', title: '处理中态', description: '当前动作正在执行' },
  { id: 'gallery-item-result', title: '结果卡片态', description: '先看结构化结果' },
  { id: 'gallery-item-detail', title: '侧边详情态', description: '查看完整结果与后续动作' },
  { id: 'gallery-item-error', title: '错误 / 不支持', description: '提示原因与可恢复操作' }
];

export const traySamples = Object.values(samples);
