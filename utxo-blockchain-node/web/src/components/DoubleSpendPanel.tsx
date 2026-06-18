import { useState } from 'react'
import type { DemoWalletInfo, DemoDoubleSpendResult } from '../lib/types'
import { shortHash } from '../lib/format'

interface Props {
  wallets: DemoWalletInfo[]
  defaultFrom: string | null
  onSubmit: (req: {
    from_wallet: string
    to_address_a: string
    to_address_b: string
    amount: number
    fee: number
  }) => Promise<DemoDoubleSpendResult>
  onExplain: (topic: string) => void
}

export function DoubleSpendPanel({ wallets, defaultFrom, onSubmit, onExplain }: Props) {
  const [from, setFrom] = useState(defaultFrom ?? '')
  const [toA, setToA] = useState('')
  const [toB, setToB] = useState('')
  const [amount, setAmount] = useState('5')
  const [fee, setFee] = useState('1')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [result, setResult] = useState<DemoDoubleSpendResult | null>(null)

  if (from === '' && defaultFrom) setFrom(defaultFrom)

  const addrOf = (n: string) => wallets.find((w) => w.name === n)?.address ?? ''

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!from || !toA || !toB) return
    setBusy(true)
    setErr(null)
    setResult(null)
    try {
      const r = await onSubmit({
        from_wallet: from,
        to_address_a: addrOf(toA),
        to_address_b: addrOf(toB),
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
    <section className="panel p-4 flex flex-col gap-3 ring-1 ring-inset ring-rose-500/20">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h2 className="panel-title text-rose-200">Attempt a double-spend</h2>
          <span className="pill ring-rose-500/30 bg-rose-500/10 text-rose-300">attack demo</span>
        </div>
        <button
          onClick={() => onExplain('double-spend')}
          className="text-[10px] text-indigo-300 hover:text-indigo-200 underline-offset-2 hover:underline"
        >
          What is a double-spend?
        </button>
      </div>

      <p className="text-xs text-slate-400 leading-relaxed">
        Build <em>two</em> transactions that spend the <strong>same</strong> UTXOs but pay
        different recipients. Submit both and see how the mempool refuses the second.
      </p>

      <form onSubmit={submit} className="grid grid-cols-3 gap-2">
        <SmallSelect label="From" value={from} onChange={setFrom} wallets={wallets} />
        <SmallSelect label="To A" value={toA} onChange={setToA} wallets={wallets} exclude={[from]} />
        <SmallSelect label="To B" value={toB} onChange={setToB} wallets={wallets} exclude={[from, toA]} />

        <SmallNumber label="Amount" value={amount} onChange={setAmount} />
        <SmallNumber label="Fee" value={fee} onChange={setFee} />
        <button
          type="submit"
          disabled={busy || !from || !toA || !toB}
          className="btn-danger self-end h-[34px] text-xs"
        >
          {busy ? 'Submitting…' : 'Submit both txs'}
        </button>
      </form>

      {err && <p className="text-xs text-rose-300">{err}</p>}

      {result && (
        <div className="grid grid-cols-2 gap-2 animate-fade-in">
          <OutcomeCard label="tx A" outcome={result.first} />
          <OutcomeCard label="tx B" outcome={result.second} />
        </div>
      )}
    </section>
  )
}

function SmallSelect({
  label,
  value,
  onChange,
  wallets,
  exclude,
}: {
  label: string
  value: string
  onChange: (v: string) => void
  wallets: DemoWalletInfo[]
  exclude?: string[]
}) {
  return (
    <label className="flex flex-col gap-1">
      <span className="text-[10px] uppercase tracking-wider text-slate-400">{label}</span>
      <select className="input text-xs" value={value} onChange={(e) => onChange(e.target.value)}>
        <option value="">…</option>
        {wallets
          .filter((w) => !exclude?.includes(w.name))
          .map((w) => (
            <option key={w.name} value={w.name}>
              {w.name}
            </option>
          ))}
      </select>
    </label>
  )
}

function SmallNumber({
  label,
  value,
  onChange,
}: {
  label: string
  value: string
  onChange: (v: string) => void
}) {
  return (
    <label className="flex flex-col gap-1">
      <span className="text-[10px] uppercase tracking-wider text-slate-400">{label}</span>
      <input
        type="number"
        min="0"
        className="input text-xs"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
    </label>
  )
}

function OutcomeCard({
  label,
  outcome,
}: {
  label: string
  outcome: { txid: string; accepted: boolean; reason?: string }
}) {
  const good = outcome.accepted
  return (
    <div
      className={`rounded-lg p-3 ring-1 ring-inset
                  ${good
                    ? 'bg-emerald-500/10 ring-emerald-500/40'
                    : 'bg-rose-500/10 ring-rose-500/40'}`}
    >
      <div className="flex items-center justify-between mb-1">
        <span className="text-xs font-semibold text-slate-200">{label}</span>
        <span
          className={`pill ${
            good
              ? 'ring-emerald-400/40 bg-emerald-400/10 text-emerald-200'
              : 'ring-rose-500/40 bg-rose-500/10 text-rose-200'
          }`}
        >
          {good ? 'accepted' : 'rejected'}
        </span>
      </div>
      <div className="hash mb-1" title={outcome.txid}>
        {shortHash(outcome.txid)}
      </div>
      {outcome.reason && <p className="text-[11px] text-rose-200/80">{outcome.reason}</p>}
    </div>
  )
}
