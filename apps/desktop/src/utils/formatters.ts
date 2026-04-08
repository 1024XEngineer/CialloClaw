// 该文件提供展示层文本与时间格式化能力。 
export function formatStatusLabel(status: string) {
  const statusLabels: Record<string, string> = {
    confirming_intent: "等待意图确认",
    processing: "处理中",
    waiting_auth: "等待授权",
    waiting_input: "等待补充输入",
    paused: "已暂停",
    blocked: "已阻塞",
    failed: "失败",
    completed: "已完成",
    cancelled: "已取消",
    ended_unfinished: "已结束未完成",
  };

  return statusLabels[status] ?? status.replaceAll("_", " ");
}

export function formatTimestamp(value: string | null) {
  if (!value) {
    return "未开始";
  }

  return new Date(value).toLocaleString();
}
