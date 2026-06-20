interface Props {
  topic: string | null
  onClose: () => void
}

const CONTENT: Record<string, { title: string; body: React.ReactNode }> = {
  overview: {
    title: 'The big picture',
    body: (
      <>
        <P>
          This page is a live view of a real Go process running on your machine. Every panel below
          calls the same HTTP API that a wallet or block explorer would use.
        </P>
        <H>How it fits together</H>
        <P>
          Money in a UTXO chain is not stored as account balances. Instead, every coin lives as an{' '}
          <Term>unspent transaction output</Term> (UTXO). To spend, you produce a transaction that
          consumes one or more UTXOs as inputs and creates new UTXOs as outputs. The miner sweeps
          pending transactions out of the <Term>mempool</Term> and packages them into a{' '}
          <Term>block</Term>; the block's hash chains it to its predecessor, making the history
          tamper-evident.
        </P>
        <H>What to try</H>
        <Bullets>
          <li>Pick Alice and send 10 coins to Bob.</li>
          <li>Watch the mempool fill, then click <em>Mine block</em>.</li>
          <li>See Alice's UTXO disappear and Bob's appear.</li>
          <li>Then try the double-spend panel — the network catches you.</li>
        </Bullets>
      </>
    ),
  },
  wallets: {
    title: 'Wallets — keys, not accounts',
    body: (
      <>
        <P>
          A wallet here is just an <Term>secp256k1 keypair</Term> plus a label. The address you see is{' '}
          <Code>HASH160(public_key)</Code> — 20 bytes, displayed as 40 hex characters.
        </P>
        <P>
          The wallet's <Term>balance</Term> is computed live: we walk every UTXO whose recipient
          equals the wallet's address and sum the values. There is no account ledger anywhere on
          disk — only outputs.
        </P>
        <Note>
          These demo private keys live in the node's memory and are persisted as plaintext JSON
          under <Code>data/&lt;node&gt;/wallets.json</Code>. Real wallets keep keys encrypted on
          your device.
        </Note>
      </>
    ),
  },
  utxo: {
    title: 'UTXOs — coins are outputs',
    body: (
      <>
        <P>
          Each row in the UTXO panel represents a single spendable coin. It is identified by its{' '}
          <Term>outpoint</Term>: the transaction that created it and the output index within that
          transaction. The value is fixed at the time it was created — you cannot mutate a UTXO,
          only consume it.
        </P>
        <H>Colour code</H>
        <Bullets>
          <li>
            <ColorDot c="bg-emerald-400" /> green — unspent and available
          </li>
          <li>
            <ColorDot c="bg-amber-400" /> amber (glow) — about to be spent by a tx in the mempool
          </li>
          <li>
            <ColorDot c="bg-violet-400" /> violet — created by a <Term>coinbase</Term> (block
            reward)
          </li>
        </Bullets>
        <P>
          Because UTXOs are immutable, the chain has the perfect audit trail: every coin can be
          traced back to a coinbase via a sequence of transactions.
        </P>
      </>
    ),
  },
  tx: {
    title: 'Building a transaction',
    body: (
      <>
        <P>When you click <em>Sign &amp; submit</em>, the node does the following on your behalf:</P>
        <Steps>
          <li>List the UTXOs owned by the sender.</li>
          <li>
            Greedily pick the largest UTXOs until their total ≥ <Code>amount + fee</Code>.
          </li>
          <li>Construct a Transaction with those outpoints as inputs and two outputs:
            the payment, and the change paid back to the sender.</li>
          <li>
            Sign each input with the sender's private key using ECDSA over the transaction's
            canonical SHA-256.
          </li>
          <li>
            Hand it to the mempool, which revalidates: signatures, fee non-negativity, no
            conflicts with other pending txs, no spends of non-existent UTXOs.
          </li>
        </Steps>
        <Note>
          The <Term>fee</Term> is whatever you didn't account for in outputs. The miner sweeps it
          into the coinbase when they include your tx in a block — which is why higher fee rates
          get mined faster.
        </Note>
      </>
    ),
  },
  'double-spend': {
    title: 'Why double-spending fails',
    body: (
      <>
        <P>
          A naïve attack: take a coin you've already spent, sign a second transaction that pays
          someone else with the same coin, and hope the network forgets.
        </P>
        <P>
          The mempool keeps a <Code>spends</Code> index — a map from outpoint to the TXID of the
          transaction currently spending it. When you submit tx B, the node looks up every input,
          finds tx A already there, and rejects B as a conflict.
        </P>
        <H>This panel shows that mechanism live</H>
        <Bullets>
          <li>Both transactions are valid in isolation.</li>
          <li>They reference identical inputs but pay different recipients.</li>
          <li>The first to arrive wins; the second is rejected with a clear reason.</li>
        </Bullets>
        <Note>
          In a real network, this race is decided by the miner who builds the next block.
          Anything not in the mempool when that block is mined cannot conflict with what is
          included.
        </Note>
      </>
    ),
  },
  mempool: {
    title: 'The mempool — pending state',
    body: (
      <>
        <P>
          The mempool is an in-memory cache of valid but un-mined transactions. It is the staging
          area between "I signed it" and "it's in the chain forever".
        </P>
        <P>
          Each entry shows the transaction ID, its size in bytes, the fee, and the fee rate
          (fee/byte). Miners sort the mempool by descending fee rate when building the next block,
          so paying more makes you go first.
        </P>
        <H>What <em>mining</em> does</H>
        <Steps>
          <li>Take the highest-fee-rate txs from the mempool that fit.</li>
          <li>Prepend a <Term>coinbase</Term> that pays the block subsidy + fees to the miner.</li>
          <li>
            Search for a <Term>nonce</Term> such that the block's double-SHA-256 starts with N
            zero nibbles — proof-of-work.
          </li>
          <li>Import the new block, update the chain tip, evict mined txs from the mempool.</li>
        </Steps>
      </>
    ),
  },
  blockchain: {
    title: 'Blocks — the canonical ledger',
    body: (
      <>
        <P>
          Each block in the strip below references the previous block by hash. That single
          pointer is enough to make the whole history immutable: changing one byte of an old
          block changes its hash, which breaks every subsequent block's pointer.
        </P>
        <H>Fields shown</H>
        <Bullets>
          <li>
            <Code>height</Code> — position in the chain. Genesis is height 0.
          </li>
          <li>
            <Code>nonce</Code> — the value the miner found that made the hash valid.
          </li>
          <li>
            <Code>merkle_root</Code> — Merkle root over all transaction IDs in the block.
            Used to prove a transaction's inclusion without downloading the whole block.
          </li>
          <li>
            <Code>prev_hash</Code> — the hash of the parent block; the chain link.
          </li>
        </Bullets>
        <Note>
          With every new block, all of the block's transactions become confirmed: their inputs
          are permanently consumed and their outputs are added to the spendable UTXO set.
        </Note>
      </>
    ),
  },
}

export function ExplanationPanel({ topic, onClose }: Props) {
  if (!topic) return null
  const content = CONTENT[topic] ?? CONTENT.overview

  return (
    <>
      <div
        className="fixed inset-0 z-30 bg-slate-950/60 backdrop-blur-sm animate-fade-in"
        onClick={onClose}
      />
      <aside
        className="fixed top-0 right-0 z-40 h-full w-full max-w-md
                   bg-slate-900 ring-1 ring-slate-700/60 shadow-2xl
                   animate-slide-in-right overflow-y-auto"
      >
        <header className="sticky top-0 bg-slate-900/95 backdrop-blur-sm px-5 py-3 border-b border-slate-800 flex items-center justify-between">
          <h2 className="text-sm font-semibold text-slate-100">{content.title}</h2>
          <button
            onClick={onClose}
            className="rounded-md text-slate-400 hover:text-slate-200 hover:bg-slate-800 h-7 w-7 inline-flex items-center justify-center"
            aria-label="Close"
          >
            ✕
          </button>
        </header>
        <div className="px-5 py-4 text-sm text-slate-300 space-y-3 leading-relaxed">
          {content.body}
        </div>
      </aside>
    </>
  )
}

function P({ children }: { children: React.ReactNode }) {
  return <p>{children}</p>
}
function H({ children }: { children: React.ReactNode }) {
  return <h3 className="text-xs font-semibold uppercase tracking-wider text-indigo-300 mt-4 mb-1">{children}</h3>
}
function Bullets({ children }: { children: React.ReactNode }) {
  return <ul className="list-disc list-inside space-y-1 text-slate-400">{children}</ul>
}
function Steps({ children }: { children: React.ReactNode }) {
  return <ol className="list-decimal list-inside space-y-1 text-slate-400">{children}</ol>
}
function Term({ children }: { children: React.ReactNode }) {
  return <span className="font-semibold text-cyan-300">{children}</span>
}
function Code({ children }: { children: React.ReactNode }) {
  return <code className="mono rounded bg-slate-800/70 px-1.5 py-0.5 text-cyan-200">{children}</code>
}
function Note({ children }: { children: React.ReactNode }) {
  return (
    <div className="rounded-md bg-amber-400/5 ring-1 ring-amber-400/30 px-3 py-2 text-amber-100/80">
      <span className="text-amber-300 mr-1">⚠</span>
      {children}
    </div>
  )
}
function ColorDot({ c }: { c: string }) {
  return <span className={`inline-block h-2.5 w-2.5 rounded-full ${c} mr-1.5 align-middle`} />
}
