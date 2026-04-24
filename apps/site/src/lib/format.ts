export function formatDateLabel(input: string | null): string {
  if (!input) {
    return "Unknown publish time";
  }

  const date = new Date(input);

  if (Number.isNaN(date.getTime())) {
    return "Unknown publish time";
  }

  return new Intl.DateTimeFormat("en", {
    year: "numeric",
    month: "short",
    day: "numeric",
  }).format(date);
}

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return "Unknown size";
  }

  const units = ["B", "KB", "MB", "GB"];
  let value = bytes;
  let index = 0;

  while (value >= 1024 && index < units.length - 1) {
    value /= 1024;
    index += 1;
  }

  const fixed = value >= 100 || index === 0 ? 0 : 1;
  return `${value.toFixed(fixed)} ${units[index]}`;
}
