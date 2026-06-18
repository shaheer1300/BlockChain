import { useState } from 'react'
import type { DemoWalletInfo } from '../lib/types'
import { shortHash, fmtAmount, copyToClipboard } from '../lib/format'

interface Props {
  wallets: DemoWalletInfo[]
  selected: string | null
  onSelect: (name: string) => void
  onCreate: (name: string) => Promise<void>
  onExplain: (topic: string) => void
}

export function WalletsPanel({ wallets, selected, onSelect, onCreate, onExplain }: Props) {
  const [newName, setNewName] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newName.trim()) return
    setBusy(true)
    setErr(null)
    try {
      await onCreate(newName.trim())
      setNewName('')
    } catch (ex) {
      setErr(ex instanceof Error ? ex.message : String(ex))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="panel p-4 flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <h2 className="panel-title">Wallets</h2>
        <button
          onClick={() => onExplain('wallets')}
          className="text-[10px] text-indigo-300 hover:text-indigo-200 underline-offset-2 hover:underline"
        >
          What is a wallet?
        </button>
      </div>

      <ul className="space-y-2">
        {wallets.length === 0 && (
          <li className="text-xs text-slate-500 italic py-4 text-center">No wallets yet.</li>
        )}
        {wallets.map((w) => {
          const isSel = selected === w.name
          return (
            <li key={w.name}>
              <button
                onClick={() => onSelect(w.name)}
                className={`w-full text-left rounded-lg px-3 py-2.5 transition group
                            ring-1 ring-inset
                            ${isSel
                              ? 'bg-cyan-400/10 ring-cyan-400/50 shadow-md shadow-cyan-500/10'
                              : 'bg-slate-800/40 ring-slate-700/60 hover:ring-slate-600 hover:bg-slate-800/70'
                            }`}
              >
                <div className="flex items-center justify-between gap-3">
                  <div className="flex items-center gap-2 min-w-0">
                    <Avatar name={w.name} active={isSel} />
                    <div className="min-w-0">
                      <div className="text-sm font-medium text-slate-100 truncate">
                        {w.name}
                      </div>
                      <div
                        className="hash truncate"
                        title={w.address}
                        onClick={async (e) => {
                          e.stopPropagation()
                          await copyToClipboard(w.address)
                        }}
                      >
                        {shortHash(w.address, 8, 6)}
                      </div>
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="text-sm font-semibold text-emerald-300">
                      {fmtAmount(w.balance)}
                    </div>
                    <div className="text-[10px] text-slate-500">satoshi-units</div>
                  </div>
                </div>
              </button>
            </li>
          )
        })}
      </ul>

      <form onSubmit={submit} className="flex items-center gap-2 pt-2 border-t border-slate-800">
        <input
          className="input flex-1"
          placeholder="new wallet name…"
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          disabled={busy}
        />
        <button type="submit" className="btn-primary text-xs px-3 py-2" disabled={busy || !newName.trim()}>
          Add
        </button>
      </form>
      {err && <p className="text-xs text-rose-300">{err}</p>}
    </section>
  )
}

function Avatar({ name, active }: { name: string; active: boolean }) {
  const initial = name.charAt(0).toUpperCase()
  return (
    <div
      className={`h-8 w-8 shrink-0 rounded-full flex items-center justify-center
                  text-sm font-semibold
                  ${active
                    ? 'bg-gradient-to-br from-cyan-400 to-indigo-500 text-slate-950'
                    : 'bg-slate-700 text-slate-200'}`}
    >
      {initial}
    </div>
  )
}
