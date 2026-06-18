import { useCallback, useEffect, useRef, useState } from 'react'
import { api, ApiError } from './lib/api'
import type { DemoState } from './lib/types'
import { Header } from './components/Header'
import { SetupCard } from './components/SetupCard'
import { WalletsPanel } from './components/WalletsPanel'
import { UTXOPanel } from './components/UTXOPanel'
import { TxBuilder } from './components/TxBuilder'
import { DoubleSpendPanel } from './components/DoubleSpendPanel'
import { MempoolPanel } from './components/MempoolPanel'
import { BlockchainPanel } from './components/BlockchainPanel'
import { ExplanationPanel } from './components/ExplanationPanel'

const POLL_INTERVAL_MS = 2000

export default function App() {
  const [state, setState] = useState<DemoState | undefined>()
  const [error, setError] = useState<string | null>(null)
  const [selectedWallet, setSelectedWallet] = useState<string | null>(null)
  const [explainTopic, setExplainTopic] = useState<string | null>(null)
  const [toast, setToast] = useState<{ kind: 'ok' | 'err'; msg: string } | null>(null)
  const pollingRef = useRef<number | null>(null)

  const refresh = useCallback(async () => {
    try {
      const s = await api.state()
      setState(s)
      setError(null)
    } catch (e) {
      if (e instanceof ApiError) {
        setError(`${e.status}: ${e.message}`)
      } else if (e instanceof Error) {
        setError(e.message)
      }
    }
  }, [])

  // Initial load + polling loop.
  useEffect(() => {
    refresh()
    pollingRef.current = window.setInterval(refresh, POLL_INTERVAL_MS)
    return () => {
      if (pollingRef.current) window.clearInterval(pollingRef.current)
    }
  }, [refresh])

  // Auto-select the first wallet so the UI feels alive on first load.
  useEffect(() => {
    if (!selectedWallet && state?.wallets.length) {
      setSelectedWallet(state.wallets[0].name)
    }
  }, [state, selectedWallet])

  const showToast = (kind: 'ok' | 'err', msg: string) => {
    setToast({ kind, msg })
    window.setTimeout(() => setToast(null), 4000)
  }

  const handleSetup = async (names: string[], minerName: string) => {
    for (const n of names) {
      try {
        await api.createWallet(n)
      } catch (e) {
        // ignore "already exists" so re-running is idempotent.
        if (e instanceof ApiError && /exists/i.test(e.message)) continue
        throw e
      }
    }
    await api.mine(minerName)
    await refresh()
    setSelectedWallet(minerName)
    showToast('ok', `Genesis mined. ${minerName} received the coinbase.`)
  }

  const handleCreateWallet = async (name: string) => {
    await api.createWallet(name)
    await refresh()
    setSelectedWallet(name)
    showToast('ok', `Wallet "${name}" created.`)
  }

  const handleSubmitTx = async (req: {
    from_wallet: string
    to_address: string
    amount: number
    fee: number
  }) => {
    const res = await api.submitTx(req)
    await refresh()
    showToast('ok', `Tx submitted to mempool.`)
    return res
  }

  const handleDoubleSpend = async (req: {
    from_wallet: string
    to_address_a: string
    to_address_b: string
    amount: number
    fee: number
  }) => {
    const res = await api.doubleSpend(req)
    await refresh()
    if (res.second.accepted) {
      showToast('err', 'Both accepted?! That is a bug — please report.')
    } else {
      showToast('ok', `Conflict detected: ${res.second.reason ?? 'rejected'}`)
    }
    return res
  }

  const handleMine = async (minerWallet: string) => {
    const r = await api.mine(minerWallet)
    await refresh()
    showToast('ok', `Block #${r.height} mined.`)
  }

  const handleReset = async () => {
    if (!confirm('Reset wipes wallets + mempool. The on-disk chain stays. Continue?')) return
    try {
      await api.reset()
      setSelectedWallet(null)
      await refresh()
      showToast('ok', 'Demo state reset.')
    } catch (e) {
      showToast('err', e instanceof Error ? e.message : String(e))
    }
  }

  const isPreGenesis =
    state !== undefined && state.wallets.length === 0 && state.status.height === null
  const isLive = error === null

  return (
    <div className="min-h-screen px-4 py-6">
      <Header
        status={state?.status}
        isPolling={isLive}
        onReset={handleReset}
        onExplain={setExplainTopic}
      />

      {error && (
        <div className="mx-auto max-w-[1600px] mb-6 rounded-lg bg-rose-500/10 ring-1 ring-rose-500/40 text-rose-200 px-4 py-3 text-sm">
          <strong>Disconnected from node:</strong> {error}. Make sure the Go node is running with{' '}
          <code className="mono">DEMO_MODE=1</code>.
        </div>
      )}

      <SetupCard onSetup={handleSetup} isPreGenesis={isPreGenesis} />

      <main className="mx-auto max-w-[1600px] grid grid-cols-12 gap-4 mb-6">
        <div className="col-span-12 lg:col-span-3 flex flex-col gap-4">
          <WalletsPanel
            wallets={state?.wallets ?? []}
            selected={selectedWallet}
            onSelect={setSelectedWallet}
            onCreate={handleCreateWallet}
            onExplain={setExplainTopic}
          />
          <UTXOPanel
            utxos={state?.utxos ?? []}
            mempool={state?.mempool ?? []}
            wallets={state?.wallets ?? []}
            selectedWallet={selectedWallet}
            onExplain={setExplainTopic}
          />
        </div>

        <div className="col-span-12 lg:col-span-6 flex flex-col gap-4">
          <TxBuilder
            wallets={state?.wallets ?? []}
            defaultFrom={selectedWallet}
            onSubmit={handleSubmitTx}
            onExplain={setExplainTopic}
          />
          <DoubleSpendPanel
            wallets={state?.wallets ?? []}
            defaultFrom={selectedWallet}
            onSubmit={handleDoubleSpend}
            onExplain={setExplainTopic}
          />
        </div>

        <div className="col-span-12 lg:col-span-3 flex flex-col gap-4">
          <MempoolPanel
            mempool={state?.mempool ?? []}
            wallets={state?.wallets ?? []}
            defaultMiner={selectedWallet}
            onMine={handleMine}
            onExplain={setExplainTopic}
          />
        </div>
      </main>

      <div className="mx-auto max-w-[1600px]">
        <BlockchainPanel blocks={state?.blocks ?? []} onExplain={setExplainTopic} />
      </div>

      <ExplanationPanel topic={explainTopic} onClose={() => setExplainTopic(null)} />

      {toast && (
        <div
          className={`fixed bottom-6 right-6 rounded-lg px-4 py-2 text-sm shadow-xl
                      ring-1 ring-inset animate-fade-in z-50
                      ${toast.kind === 'ok'
                        ? 'bg-emerald-500/15 ring-emerald-500/40 text-emerald-100'
                        : 'bg-rose-500/15 ring-rose-500/40 text-rose-100'}`}
        >
          {toast.msg}
        </div>
      )}

      <footer className="mx-auto max-w-[1600px] mt-8 text-center text-xs text-slate-600">
        <p>
          Educational demo · this UI talks to a real running Go node ·{' '}
          <button
            onClick={() => setExplainTopic('overview')}
            className="underline-offset-2 hover:underline text-slate-500 hover:text-slate-300"
          >
            How does this work?
          </button>
        </p>
      </footer>
    </div>
  )
}
