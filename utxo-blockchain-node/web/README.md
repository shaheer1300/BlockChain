# UTXO Blockchain — Visual Demo

A React + Vite + TypeScript frontend that turns the Go UTXO node into an
interactive teaching tool. Every panel reads live data from the node's
HTTP API; every action (`create wallet`, `submit tx`, `mine block`,
`double-spend`) hits a real endpoint and changes real on-disk state.

## Stack

- **Vite 5** — fast dev server with HMR
- **React 18** + **TypeScript 5** (strict mode)
- **Tailwind CSS 3** — utility-first styling, custom colour roles for
  blockchain concepts
- **No state library** — `useState` + 2-second polling of `/demo/state`
  is enough for the demo's volume

## Prerequisites

- Node.js ≥ 18 (tested on v22)
- The Go node running locally with `DEMO_MODE=1` (see below)

## Quick start

### 1. Start the Go node with demo mode enabled

From the repo root (`utxo-blockchain-node/`):

```powershell
$env:DEMO_MODE = "1"
$env:HTTP_ADDR = "127.0.0.1:8001"
$env:DATA_DIR  = "./data/demo"
go run ./cmd/node
```

Or, if you've used the provided run scripts, edit `scripts/run-node1.ps1`
and add `$env:DEMO_MODE = "1"` near the top, then `./scripts/run-node1.ps1`.

### 2. Start the frontend

```powershell
cd web
npm install        # only the first time
npm run dev
```

Open <http://localhost:5173>. The page polls every 2 s.

### 3. Walk through the demo

1. The first time you load the page the chain has no genesis block. Click
   **Initialise demo**. The app will:
   - Create three wallets: `alice`, `bob`, `carol`.
   - Mine the genesis block, paying the coinbase subsidy to `alice`.
2. Pick `alice` in the wallets panel — her UTXO appears in the UTXO panel
   (green = unspent, violet = coinbase).
3. Use the **Build a transaction** form to send 10 coins to `bob` with a
   1-coin fee. Watch:
   - Alice's UTXO turns amber and pulses ("about to be spent").
   - A new entry appears in the mempool panel.
4. Click **Mine block** in the mempool panel (choose any miner). After the
   block lands:
   - The mempool empties.
   - A new block appears in the blockchain strip below.
   - Alice's UTXO is gone; Bob has 10 coins; Alice's change is back.
5. Try the **double-spend** panel: same sender, two recipients, same
   inputs. The second tx is rejected — the panel shows you exactly why.

Every panel has a "What is this?" link that opens a slide-in explanation
written for the educated layperson.

## Config

| Variable        | Default                  | Meaning                                              |
|-----------------|--------------------------|------------------------------------------------------|
| `VITE_API_URL`  | `http://127.0.0.1:8001`  | Node URL the dev proxy forwards `/api/*` to.         |
| `VITE_API_BASE` | `/api`                   | Path used by the in-browser fetch client. Set to a full origin (e.g. `https://my-node.example`) when serving the built frontend separately. |

For a multi-node demo, run three nodes (`run-node1`, `run-node2`,
`run-node3`) and start three Vite dev servers on different ports each
pointing at a different node:

```powershell
$env:VITE_API_URL="http://127.0.0.1:8002"; npm run dev -- --port 5174
```

## Production build

```powershell
npm run build      # outputs to web/dist/
npm run preview    # serves dist/ for local smoke testing
```

The built bundle is ~180 kB minified (~57 kB gzipped). It can be served
by any static host — but remember to set `VITE_API_BASE` at build time to
the public URL of your backend so the browser knows where to send
requests.

## Architecture

```
web/
├── index.html              # Vite entry HTML
├── vite.config.ts          # dev proxy → http://127.0.0.1:8001
├── tailwind.config.js      # custom colour roles (utxo, spent, ...)
├── src/
│   ├── main.tsx            # React root
│   ├── App.tsx             # layout + polling + state orchestration
│   ├── index.css           # Tailwind + component classes
│   ├── lib/
│   │   ├── api.ts          # typed fetch client
│   │   ├── types.ts        # mirrors Go consensus types
│   │   └── format.ts       # shortHash, fmtAmount, timeAgo, copy helpers
│   └── components/
│       ├── Header.tsx
│       ├── SetupCard.tsx
│       ├── WalletsPanel.tsx
│       ├── UTXOPanel.tsx
│       ├── TxBuilder.tsx
│       ├── DoubleSpendPanel.tsx
│       ├── MempoolPanel.tsx
│       ├── BlockchainPanel.tsx
│       └── ExplanationPanel.tsx
```

## What the frontend does NOT do

These belong to the backend — the frontend defers everything to it:

- **Sign transactions.** Demo private keys live in the node so we don't
  ship a JS secp256k1 library. The node signs on the user's behalf.
- **Validate consensus.** Every accept/reject decision comes from the
  node's `consensus.ValidateTx` and `chain.ImportBlock`.
- **Compute hashes.** TXIDs and block hashes are returned by the node;
  the frontend never recomputes them.

This keeps the educational story honest: when the UI says "the network
rejected this", that is literally a `consensus` package call returning an
error.

## Security note

`DEMO_MODE=1` enables:

- A permissive CORS handler (`Access-Control-Allow-Origin: *`).
- HTTP endpoints under `/demo/*` that mint keypairs and sign on behalf
  of any caller who knows a wallet name.

**Never enable this on a node connected to the public internet.** It is
a local-development teaching tool, nothing more.
