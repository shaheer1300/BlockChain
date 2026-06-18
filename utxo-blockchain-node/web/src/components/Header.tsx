import { useEffect, useState } from 'react'
import type { StatusResult } from '../lib/types'
import { shortHash } from '../lib/format'

interface Props {
  status: StatusResult | undefined
  isPolling: boolean
  onReset: () => void
  onExplain: (topic: string) => void
}

export function Header({ status, isPolling, onReset, onExplain }: Props) {
  const [secondsAgo, setSecondsAgo] = useState(0)
  useEffect(() => {
    if (!isPolling) return
    setSecondsAgo(0)
    const id = setInterval(() => setSecondsAgo((s) => s + 1), 1000)
    return () => clearInterval(id)
  }, [status])

  const height = status?.height
  const tip = status?.tip_hash
  const initialized = height !== null && height !== undefined

  return (
    <header className="panel mx-auto max-w-[1600px] px-6 py-4 mb-6 flex items-center gap-6">
      <div className="flex items-center gap-3">
        <div className="h-10 w-10 rounded-xl bg-gradient-to-br from-indigo-500 to-cyan-400
                        flex items-center justify-center shadow-lg shadow-indigo-500/40">
          <span className="text-lg font-bold text-slate-950">⛓</span>
        </div>
        <div>
          <h1 className="text-lg font-semibold text-slate-100 leading-tight">
            UTXO Blockchain — Visual Demo
          </h1>
          <p className="text-xs text-slate-500">
            Live view of a running Go node · educational walkthrough
          </p>
        </div>
      </div>

      <div className="flex-1" />

      <button
        onClick={() => onExplain('overview')}
        className="text-xs text-indigo-300 hover:text-indigo-200 underline-offset-2 hover:underline"
      >
        How does this work?
      </button>

      <div className="flex items-center gap-4 text-xs">
        <Stat label="node">
          <span className="text-slate-200">{status?.node_id ?? '—'}</span>
        </Stat>
        <Stat label="network">
          <span className="text-slate-200">{status?.network ?? '—'}</span>
        </Stat>
        <Stat label="height">
          <span className={initialized ? 'text-emerald-300' : 'text-amber-300'}>
            {initialized ? height : 'pre-genesis'}
          </span>
        </Stat>
        <Stat label="tip">
          <span className="hash" title={tip ?? ''}>{shortHash(tip)}</span>
        </Stat>
        <div className="flex items-center gap-1.5">
          <span
            className={`h-2 w-2 rounded-full ${isPolling ? 'bg-emerald-400 animate-pulse' : 'bg-slate-600'}`}
            title="Live polling status"
          />
          <span className="text-slate-500">{isPolling ? `live · ${secondsAgo}s` : 'paused'}</span>
        </div>
      </div>

      <button onClick={onReset} className="btn-danger text-xs px-3 py-1.5">
        Reset
      </button>
    </header>
  )
}

function Stat({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col items-start">
      <span className="text-[10px] uppercase tracking-wider text-slate-500">{label}</span>
      {children}
    </div>
  )
}
