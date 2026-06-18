import { useState } from 'react'
import type { BlockSummary } from '../lib/types'
import { shortHash, timeAgo } from '../lib/format'

interface Props {
  blocks: BlockSummary[]
  onExplain: (topic: string) => void
}

export function BlockchainPanel({ blocks, onExplain }: Props) {
  const [selected, setSelected] = useState<string | null>(null)
  // Backend returns newest-first; reverse for left-to-right oldest→newest reading.
  const ordered = [...blocks].reverse()
  const sel = blocks.find((b) => b.hash === selected) ?? blocks[0]

  return (
    <section className="panel p-4 flex flex-col gap-3 min-h-0">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h2 className="panel-title">Blockchain</h2>
          <span className="pill ring-violet-400/40 bg-violet-400/10 text-violet-200">
            {blocks.length} block{blocks.length === 1 ? '' : 's'}
          </span>
        </div>
        <button
          onClick={() => onExplain('blockchain')}
          className="text-[10px] text-indigo-300 hover:text-indigo-200 underline-offset-2 hover:underline"
        >
          What is a block?
        </button>
      </div>

      {blocks.length === 0 ? (
        <p className="text-xs text-slate-500 italic py-12 text-center">
          The chain has no blocks yet. Mine your first one to begin.
        </p>
      ) : (
        <>
          <div className="flex items-center gap-2 overflow-x-auto pb-2 -mx-1 px-1">
            {ordered.map((b, idx) => {
              const isSel = (selected ?? blocks[0]?.hash) === b.hash
              return (
                <div key={b.hash} className="flex items-center shrink-0">
                  <button
                    onClick={() => setSelected(b.hash)}
                    title={b.hash}
                    className={`relative rounded-xl px-4 py-3 text-left transition
                                ring-1 ring-inset min-w-[150px]
                                ${isSel
                                  ? 'bg-violet-500/15 ring-violet-400/60 shadow-lg shadow-violet-500/20'
                                  : 'bg-slate-800/40 ring-slate-700 hover:ring-slate-500'}`}
                  >
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-[10px] uppercase tracking-wider text-slate-500">
                        height
                      </span>
                      <span className="text-xs text-violet-200 font-semibold">#{b.height}</span>
                    </div>
                    <div className="hash" title={b.hash}>{shortHash(b.hash)}</div>
                    <div className="flex items-center justify-between mt-1 text-[10px] text-slate-500">
                      <span>{b.tx_count} tx</span>
                      <span>{timeAgo(b.timestamp)}</span>
                    </div>
                  </button>
                  {idx < ordered.length - 1 && (
                    <div className="text-violet-500/40 px-2 text-2xl select-none">⟶</div>
                  )}
                </div>
              )
            })}
          </div>

          {sel && <BlockDetails block={sel} />}
        </>
      )}
    </section>
  )
}

function BlockDetails({ block }: { block: BlockSummary }) {
  return (
    <div className="rounded-lg ring-1 ring-inset ring-slate-700/60 bg-slate-900/60 p-4
                    grid grid-cols-2 md:grid-cols-4 gap-x-6 gap-y-2 text-xs animate-fade-in">
      <Detail label="height">
        <span className="text-violet-200">#{block.height}</span>
      </Detail>
      <Detail label="timestamp">
        <span className="text-slate-200">
          {new Date(block.timestamp * 1000).toLocaleString()}
        </span>
      </Detail>
      <Detail label="tx count">
        <span className="text-slate-200">{block.tx_count}</span>
      </Detail>
      <Detail label="nonce">
        <span className="text-slate-200 mono">{block.nonce}</span>
      </Detail>

      <Detail label="block hash" full>
        <span className="hash break-all" title={block.hash}>{block.hash}</span>
      </Detail>
      <Detail label="prev hash" full>
        <span className="hash break-all" title={block.prev_hash}>{block.prev_hash}</span>
      </Detail>
      <Detail label="merkle root" full>
        <span className="hash break-all" title={block.merkle_root}>{block.merkle_root}</span>
      </Detail>
    </div>
  )
}

function Detail({
  label,
  children,
  full,
}: {
  label: string
  children: React.ReactNode
  full?: boolean
}) {
  return (
    <div className={`${full ? 'col-span-2 md:col-span-4' : ''} flex flex-col gap-0.5`}>
      <span className="text-[10px] uppercase tracking-wider text-slate-500">{label}</span>
      {children}
    </div>
  )
}
