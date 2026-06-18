import { useState } from 'react'

interface Props {
  onSetup: (names: string[], minerName: string) => Promise<void>
  isPreGenesis: boolean
}

export function SetupCard({ onSetup, isPreGenesis }: Props) {
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  const seed = async () => {
    setBusy(true)
    setErr(null)
    try {
      await onSetup(['alice', 'bob', 'carol'], 'alice')
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  if (!isPreGenesis) return null

  return (
    <div className="panel mx-auto max-w-[1600px] p-8 mb-6 animate-fade-in">
      <div className="flex flex-col md:flex-row items-start gap-6">
        <div className="flex-1">
          <div className="flex items-center gap-2 mb-3">
            <span className="pill ring-amber-400/40 bg-amber-400/10 text-amber-200">
              <span className="h-1.5 w-1.5 rounded-full bg-amber-400" />
              pre-genesis
            </span>
          </div>
          <h2 className="text-2xl font-semibold text-slate-100 mb-2">
            Welcome — let's bring the chain to life.
          </h2>
          <p className="text-sm text-slate-400 max-w-2xl leading-relaxed">
            The node is running but the blockchain has not been initialised. Clicking the button
            below will:
          </p>
          <ol className="text-sm text-slate-400 list-decimal list-inside mt-3 space-y-1">
            <li>Create three demo wallets in the node's memory: <Mono>alice</Mono>, <Mono>bob</Mono>, <Mono>carol</Mono></li>
            <li>Mine the <Mono>genesis</Mono> block paying the coinbase subsidy to <Mono>alice</Mono></li>
            <li>Refresh the live state — you'll see Alice's first UTXO appear in the panel below</li>
          </ol>
          {err && (
            <p className="mt-4 text-sm text-rose-300">{err}</p>
          )}
        </div>
        <button
          onClick={seed}
          disabled={busy}
          className="btn-primary text-base px-6 py-3"
        >
          {busy ? 'Setting up…' : 'Initialise demo'}
        </button>
      </div>
    </div>
  )
}

function Mono({ children }: { children: React.ReactNode }) {
  return <code className="mono rounded bg-slate-800/70 px-1.5 py-0.5 text-cyan-300">{children}</code>
}
