import type { CertInfo } from '../../types'

export function CertBadge({ cert }: { cert: CertInfo }) {
  const color = cert.days_left <= 7 ? 'text-err' : cert.days_left <= 30 ? 'text-warn' : 'text-ok'
  return (
    <div className={`flex items-center gap-1 text-xs ${color}`}>
      <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
      </svg>
      <span>剩余 {cert.days_left} 天</span>
    </div>
  )
}
