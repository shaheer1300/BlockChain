import type { UTXO, MempoolEntry, DemoWalletInfo } from '../lib/types'
import { shortHash, fmtAmount } from '../lib/format'

interface Props {
  utxos: UTXO[]
  mempool: MempoolEntry[]
  wallets: DemoWalletInfo[]
  selectedWallet: string | null
  onExplain: (topic: string) => void
}

export function UTXOPanel({ utxos, mempool, wallets, selectedWallet, onExplain }: Props) {
  const ownerOf = (addr: string) => wallets.find((w) => w.address === addr)?.name
  const filterAddr = wallets.find((w) => w.name === selectedWallet)?.address

  // Mark UTXOs that are being spent by some mempool entry — those are
  // "in-flight" and will disappear when the next block is mined.
  const spentSet = new Set<string>()
  for (const m of mempool) {
    for (const inp of m.tx.inputs) {
      spentSet.add(`${inp.prev_out.txid}:${inp.prev_out.index}`)
    }
  }

  const filtered = filterAddr ? utxos.filter((u) => u.output.recipient === filterAddr) : utxos

  return (
    <section className="panel p-4 flex flex-col gap-3 min-h-0">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h2 className="panel-title">UTXOs</h2>
          <span className="pill ring-emerald-400/40 bg-emerald-400/10 text-emerald-200">
            {filtered.length} unspent
          </span>
          {filterAddr && (
            <span className="text-[10px] text-slate-500">
              filtered to <span className="text-cyan-300">{selectedWallet}</span>
            </span>
          )}
        </div>
        <button
          onClick={() => onExplain('utxo')}
          className="text-[10px] text-indigo-300 hover:text-indigo-200 underline-offset-2 hover:underline"
        >
          What is a UTXO?
        </button>
      </div>

      <div className="flex-1 overflow-auto -mx-1 px-1 max-h-64">
        {filtered.length === 0 ? (
          <p className="text-xs text-slate-500 italic py-6 text-center">
            No unspent outputs{filterAddr ? ' for this wallet' : ''} yet — mine a block to create one.
          </p>
        ) : (
          <ul className="space-y-1.5">
            {filtered.map((u) => {
              const key = `${u.outpoint.txid}:${u.outpoint.index}`
              const isSpent = spentSet.has(key)
              const owner = ownerOf(u.output.recipient)
              return (
                <li
                  key={key}
                  className={`rounded-lg px-3 py-2 ring-1 ring-inset transition
                              ${isSpent
                                ? 'bg-amber-400/5 ring-amber-400/40 animate-pulse-glow'
                                : u.coinbase
                                  ? 'bg-violet-400/5 ring-violet-400/30'
                                  : 'bg-emerald-400/5 ring-emerald-400/30'}`}
                >
                  <div className="flex items-center justify-between gap-3">
                    <div className="flex items-center gap-2 min-w-0">
                      <div
                        className={`h-2.5 w-2.5 shrink-0 rounded-full
                                    ${isSpent ? 'bg-amber-400'
                                      : u.coinbase ? 'bg-violet-400' : 'bg-emerald-400'}`}
                      />
                      <div className="min-w-0">
                        <div className="hash truncate" title={u.outpoint.txid}>
                          {shortHash(u.outpoint.txid)}:{u.outpoint.index}
                        </div>
                        <div className="text-[10px] text-slate-500 truncate">
                          {owner ?? shortHash(u.output.recipient)}
                          {u.coinbase && <span className="ml-1 text-violet-300">· coinbase</span>}
                          <span className="ml-1">· height {u.height}</span>
                        </div>
                      </div>
                    </div>
                    <div className="text-right">
                      <div className="text-sm font-semibold text-emerald-300">
                        {fmtAmount(u.output.value)}
                      </div>
                      {isSpent && (
                        <div className="text-[10px] text-amber-300 uppercase tracking-wide">
                          spending…
                        </div>
                      )}
                    </div>
                  </div>
                </li>
              )
            })}
          </ul>
        )}
      </div>
    </section>
  )
}
