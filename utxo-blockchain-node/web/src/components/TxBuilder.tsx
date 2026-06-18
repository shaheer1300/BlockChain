import { useState } from 'react'
import type { DemoWalletInfo, DemoTxResult } from '../lib/types'
import { shortHash, fmtAmount } from '../lib/format'

interface Props {
  wallets: DemoWalletInfo[]
  defaultFrom: string | null
  onSubmit: (req: {
    from_wallet: string
    to_address: string
    amount: number
    fee: number
  }) => Promise<DemoTxResult>
  onExplain: (topic: string) => void
}

export function TxBuilder({ wallets, defaultFrom, onSubmit, onExplain }: Props) {
  const [from, setFrom] = useState(defaultFrom ?? '')
  const [toMode, setToMode] = useState<'wallet' | 'address'>('wallet')
  const [toWallet, setToWallet] = useState('')
  const [toAddress, setToAddress] = useState('')
  const [amount, setAmount] = useState('10')
  const [fee, setFee] = useState('1')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [result, setResult] = useState<DemoTxResult | null>(null)

  // Keep `from` in sync with the wallet picker selection (defaultFrom).
  if (from === '' && defaultFrom) setFrom(defaultFrom)

  const resolvedTo =
    toMode === 'wallet'
      ? wallets.find((w) => w.name === toWallet)?.address ?? ''
      : toAddress.trim()

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!from || !resolvedTo) return
    setBusy(true)
    setErr(null)
    setResult(null)
    try {
      const r = await onSubmit({
        from_wallet: from,
        to_address: resolvedTo,
        amount: Number(amount),
        fee: Number(fee),
      })
      setResult(r)
    } catch (ex) {
      setErr(ex instanceof Error ? ex.message : String(ex))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="panel p-4 flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <h2 className="panel-title">Build a transaction</h2>
        <button
          onClick={() => onExplain('tx')}
          className="text-[10px] text-indigo-300 hover:text-indigo-200 underline-offset-2 hover:underline"
        >
          What happens here?
        </button>
      </div>

      <form onSubmit={submit} className="grid grid-cols-2 gap-3">
        <Field label="From wallet" full>
          <select className="input" value={from} onChange={(e) => setFrom(e.target.value)}>
            <option value="">Select…</option>
            {wallets.map((w) => (
              <option key={w.name} value={w.name}>
                {w.name} ({fmtAmount(w.balance)})
              </option>
            ))}
          </select>
        </Field>

        <Field label="To" full>
          <div className="flex gap-1 mb-1">
            <ToggleBtn active={toMode === 'wallet'} onClick={() => setToMode('wallet')}>
              wallet
            </ToggleBtn>
            <ToggleBtn active={toMode === 'address'} onClick={() => setToMode('address')}>
              address
            </ToggleBtn>
          </div>
          {toMode === 'wallet' ? (
            <select className="input" value={toWallet} onChange={(e) => setToWallet(e.target.value)}>
              <option value="">Select…</option>
              {wallets
                .filter((w) => w.name !== from)
                .map((w) => (
                  <option key={w.name} value={w.name}>
                    {w.name}
                  </option>
                ))}
            </select>
          ) : (
            <input
              className="input mono"
              placeholder="20 bytes hex (40 chars)…"
              value={toAddress}
              onChange={(e) => setToAddress(e.target.value)}
            />
          )}
        </Field>

        <Field label="Amount">
          <input
            type="number"
            min="1"
            className="input"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
          />
        </Field>
        <Field label="Fee">
          <input
            type="number"
            min="0"
            className="input"
            value={fee}
            onChange={(e) => setFee(e.target.value)}
          />
        </Field>

        <button
          type="submit"
          disabled={busy || !from || !resolvedTo}
          className="btn-primary col-span-2 mt-1"
        >
          {busy ? 'Signing & submitting…' : 'Sign & submit'}
        </button>
      </form>

      {err && (
        <p className="text-xs rounded-md bg-rose-500/10 ring-1 ring-rose-500/40 text-rose-200 px-3 py-2">
          {err}
        </p>
      )}

      {result && <TxResultPreview result={result} wallets={wallets} />}
    </section>
  )
}

function Field({
  label,
  children,
  full,
}: {
  label: string
  children: React.ReactNode
  full?: boolean
}) {
  return (
    <label className={`flex flex-col gap-1 ${full ? 'col-span-2' : ''}`}>
      <span className="text-[10px] uppercase tracking-wider text-slate-400">{label}</span>
      {children}
    </label>
  )
}

function ToggleBtn({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`text-[10px] px-2 py-1 rounded-md transition
                  ${active
                    ? 'bg-indigo-500 text-white'
                    : 'bg-slate-800 text-slate-400 hover:bg-slate-700'}`}
    >
      {children}
    </button>
  )
}

function TxResultPreview({
  result,
  wallets,
}: {
  result: DemoTxResult
  wallets: DemoWalletInfo[]
}) {
  const ownerOf = (a: string) => wallets.find((w) => w.address === a)?.name
  const totalIn = result.inputs.reduce((s, i) => s + i.value, 0)
  const totalOut = result.outputs.reduce((s, o) => s + o.value, 0)
  const fee = totalIn - totalOut
  return (
    <div className="rounded-lg border border-emerald-500/30 bg-emerald-500/5 p-3 animate-fade-in">
      <div className="flex items-center justify-between mb-2">
        <span className="pill ring-emerald-400/40 bg-emerald-400/10 text-emerald-200">
          ✓ submitted to mempool
        </span>
        <span className="hash" title={result.txid}>
          txid {shortHash(result.txid)}
        </span>
      </div>
      <div className="grid grid-cols-[1fr_auto_1fr] gap-3 items-center text-xs">
        <div>
          <div className="text-[10px] uppercase tracking-wider text-slate-400 mb-1">
            inputs ({result.inputs.length})
          </div>
          <ul className="space-y-1">
            {result.inputs.map((i, idx) => (
              <li key={idx} className="flex items-center justify-between gap-2 rounded bg-slate-800/40 px-2 py-1">
                <span className="hash truncate" title={i.txid}>
                  {shortHash(i.txid)}:{i.index}
                </span>
                <span className="text-emerald-300 font-mono">{fmtAmount(i.value)}</span>
              </li>
            ))}
          </ul>
        </div>
        <div className="text-slate-500 text-2xl">→</div>
        <div>
          <div className="text-[10px] uppercase tracking-wider text-slate-400 mb-1">
            outputs ({result.outputs.length})
          </div>
          <ul className="space-y-1">
            {result.outputs.map((o, idx) => (
              <li
                key={idx}
                className={`flex items-center justify-between gap-2 rounded px-2 py-1
                            ${o.purpose === 'change'
                              ? 'bg-slate-800/60 ring-1 ring-slate-700'
                              : 'bg-indigo-500/10 ring-1 ring-indigo-400/40'}`}
              >
                <span className="truncate">
                  <span className="text-[10px] text-slate-500 mr-1">{o.purpose}</span>
                  <span className="text-cyan-200">{ownerOf(o.recipient) ?? shortHash(o.recipient)}</span>
                </span>
                <span className="text-emerald-300 font-mono">{fmtAmount(o.value)}</span>
              </li>
            ))}
          </ul>
        </div>
      </div>
      <div className="mt-2 pt-2 border-t border-slate-700/50 text-[10px] text-slate-500 flex justify-between">
        <span>sum-in {fmtAmount(totalIn)} − sum-out {fmtAmount(totalOut)} = fee</span>
        <span className="text-amber-300 font-mono">{fmtAmount(fee)}</span>
      </div>
    </div>
  )
}
