type MetricCardProps = {
  label: string;
  value: string;
  detail: string;
};

// MetricCard renders compact dashboard summary metrics on the shared light desktop theme.
export function MetricCard({ label, value, detail }: MetricCardProps) {
  return (
    <div className="rounded-2xl border border-[color:var(--cc-line)] bg-[color:var(--cc-glass)] p-4 shadow-[inset_0_1px_0_rgba(255,255,255,0.62)]">
      <p className="text-xs uppercase tracking-[0.24em] text-[color:var(--cc-ink-muted)]">{label}</p>
      <p className="mt-3 text-3xl font-semibold text-[color:var(--cc-ink)]">{value}</p>
      <p className="mt-2 text-sm text-[color:var(--cc-ink-soft)]">{detail}</p>
    </div>
  );
}
