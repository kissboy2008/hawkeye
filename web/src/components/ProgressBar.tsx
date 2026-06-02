export function ProgressBar({ value, colorClass }: { value: number; colorClass: string }) {
  return (
    <div className="h-2 rounded-full bg-bg-hover/60 overflow-hidden">
      <div
        className={`h-full rounded-full transition-all duration-500 ${colorClass}`}
        style={{ width: `${Math.min(Math.max(value, 0), 100)}%` }}
      />
    </div>
  )
}

export function getBarColor(value: number, warnAt: number, dangerAt: number): string {
  if (value >= dangerAt) return 'bg-err'
  if (value >= warnAt) return 'bg-warn'
  return 'gradient-bar'
}
