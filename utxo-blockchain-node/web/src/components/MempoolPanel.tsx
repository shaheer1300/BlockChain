import { useState } from 'react'
import type { MempoolEntry, DemoWalletInfo } from '../lib/types'
import { shortHash, fmtAmount, timeAgo } from '../lib/format'

interface Props {
  mempool: MempoolEntry[]
  wallets: DemoWalletInfo[]
  defaultMiner: string | null
  onMine: (minerWallet: string) => Promise<void>
  onExplain: (topic: string) => void
}

export function MempoolPanel({ mempool, wallets, defaultMiner, onMine, onExplain }: Props) {
  const [miner, setMiner] = useState(defaultMiner ?? '')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  if (miner === '' && defaultMiner) setMiner(defaultMiner)

  const totalFees = mempool.reduce((s, m) => s + m.fee, 0)

  const ownerOf = (a: string) => wallets.find((w) => w.address === a)?.name

  const mine = async () => {
    if (!miner) {
      setErr('select a miner wallet')
      return
    }
    setBusy(true)
    setErr(null)
    try {
      await onMine(miner)
    } catch (ex) {
      setErr(ex instanceof Error ? ex.message : String(ex))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="panel p-4 flex flex-col gap-3 min-h-0">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h2 className="panel-title">Mempool</h2>
          <span className="pill ring-amber-400/40 bg-amber-400/10 text-amber-200">
            {mempool.length} pending
          </span>
        </div>
        <button
          onClick={() => onExplain('mempool')}
          className="text-[10px] text-indigo-300 hover:text-indigo-200 underline-offset-2 hover:underline"
        >
          What is the mempool?
        </button>
      </div>

      <div className="flex-1 overflow-auto -mx-1 px-1 max-h-[260px]">
        {mempool.length === 0 ? (
          <p className="text-xs text-slate-500 italic py-6 text-center">
            Mempool is empty. Submit a transaction to see it queue up here.
          </p>
        ) : (
          <ul className="space-y-1.5">
            {mempool.map((m) => {
              const recipients = m.tx.outputs.map((o) => ownerOf(o.recipient) ?? shortHash(o.recipient))
              return (
                <li
                  key={m.txid}
                  className="rounded-lg px-3 py-2 ring-1 ring-inset ring-amber-400/30 bg-amber-400/5
                             hover:ring-amber-400/50 transition"
                >
                  <div className="flex items-center justify-between gap-3">
                    <div className="min-w-0">
                      <div className="hash truncate" title={m.txid}>
                        {shortHash(m.txid)}
                      </div>
                      <div className="text-[10px] text-slate-500 truncate">
                        {m.tx.inputs.length} in · {m.tx.outputs.length} out · {m.size}B ·{' '}
                        {timeAgo(m.arrived_at)}
                      </div>
                      <div className="text-[10px] text-slate-400 truncate">
                        → {recipients.join(', ')}
                      </div>
                    </div>
                    <div className="text-right">
                      <div className="text-sm font-semibold text-amber-300">
                        {fmtAmount(m.fee)}
                      </div>
                      <div className="text-[10px] text-slate-500">
                        {m.fee_rate.toFixed(2)} / B
                      </div>
                    </div>
                  </div>
                </li>
              )
            })}
          </ul>
        )}
      </div>

      <div className="flex items-end gap-2 pt-2 border-t border-slate-800">
        <label className="flex-1 flex flex-col gap-1">
          <span className="text-[10px] uppercase tracking-wider text-slate-400">
            Miner wallet
          </span>
          <select className="input text-xs" value={miner} onChange={(e) => setMiner(e.target.value)}>
            <option value="">Select…</option>
            {wallets.map((w) => (
              <option key={w.name} value={w.name}>
                {w.name}
              </option>
            ))}
          </select>
        </label>
        <button onClick={mine} disabled={busy || !miner} className="btn-primary text-xs h-[34px]">
          {busy ? 'Mining…' : `Mine block (+${fmtAmount(totalFees)} fees)`}
        </button>
      </div>
      {err && <p className="text-xs text-rose-300">{err}</p>}
    </section>
  )
}
