// Package p2p implements simple HTTP-based peer gossip. Each node maintains a
// peer list from configuration. When a new transaction or block is received or
// locally produced, it is fanned out to all known peers over HTTP POST.
//
// Design constraints (Milestone 12):
//   - stdlib-only HTTP client; no third-party libraries.
//   - Duplicate suppression via a fixed-capacity seen-cache (sync.Map).
//   - Parent-missing blocks are dropped (no orphan queue yet).
//   - Best-effort delivery: peer errors are logged but never returned to the
//     caller, so a peer outage never blocks local progress.
//
// Rule: this package must not import internal/api, internal/node, or any
// other package above it in the dependency graph. It calls back into the
// node through a Callbacks interface to remain decoupled.
package p2p

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

const (
	// broadcastTimeout caps each individual peer HTTP call so that a slow
	// peer never blocks a broadcast indefinitely.
	broadcastTimeout = 5 * time.Second

	// seenCacheMaxSize is the maximum number of entries kept in the
	// seen-cache. Entries are dropped in FIFO order when this limit is
	// reached so the cache cannot grow without bound.
	seenCacheMaxSize = 4096
)

// Callbacks is the interface the Gossiper calls back into the node to
// process received objects. Keeping this interface here prevents p2p from
// importing node/chain/mempool directly.
type Callbacks interface {
	// HandleReceivedTx processes a transaction that arrived from a peer.
	// Implementations should validate and add it to the mempool. Any
	// returned error is logged but does not affect gossip propagation.
	HandleReceivedTx(ctx context.Context, tx *types.Transaction) error

	// HandleReceivedBlock processes a block that arrived from a peer.
	// Implementations should validate and connect it. ErrOrphanBlock or
	// parent-missing errors are expected and should not be propagated as
	// fatal errors — the block is simply discarded in that case.
	HandleReceivedBlock(ctx context.Context, block *types.Block) error
}

// Gossiper fans outgoing objects to configured peers and routes incoming
// objects (received via the HTTP endpoints added in api/server.go) to the
// Callbacks implementation. All exported methods are safe for concurrent use.
type Gossiper struct {
	peers    []string // base URLs, e.g. "http://127.0.0.1:8002"
	log      *slog.Logger
	cb       Callbacks
	client   *http.Client   // shared client with connection-pooling
	seen     sync.Map       // Hash32 → struct{}, presence = already processed
	seenKeys []types.Hash32 // FIFO order for bounded eviction
	seenMu   sync.Mutex     // guards seenKeys slice only
}

// New creates a Gossiper. peers is the list of base peer URLs. cb handles
// objects received from peers; it may be nil if only broadcasting is needed.
func New(peers []string, log *slog.Logger, cb Callbacks) *Gossiper {
	return &Gossiper{
		peers: peers,
		log:   log,
		cb:    cb,
		client: &http.Client{
			Timeout: broadcastTimeout,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 4,
				IdleConnTimeout:     30 * time.Second,
			},
		},
	}
}

// ── seen-cache ────────────────────────────────────────────────────────────────

// markSeen records h in the seen-cache. Returns true if h was already present
// (the caller should suppress rebroadcast in that case). Evicts the oldest
// entry when the cache is full.
func (g *Gossiper) markSeen(h types.Hash32) (alreadySeen bool) {
	if _, loaded := g.seen.LoadOrStore(h, struct{}{}); loaded {
		return true
	}
	g.seenMu.Lock()
	defer g.seenMu.Unlock()
	g.seenKeys = append(g.seenKeys, h)
	if len(g.seenKeys) > seenCacheMaxSize {
		evict := g.seenKeys[0]
		g.seenKeys = g.seenKeys[1:]
		g.seen.Delete(evict)
	}
	return false
}

// ── broadcast helpers ─────────────────────────────────────────────────────────

// broadcastJSON sends body to path on every peer in a fan-out goroutine pool.
// Errors are logged but not returned — gossip delivery is best-effort.
func (g *Gossiper) broadcastJSON(ctx context.Context, path string, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		g.log.Error("p2p: marshal broadcast body", "path", path, "err", err)
		return
	}
	for _, peer := range g.peers {
		peer := peer // capture for goroutine
		go func() {
			if err := g.postJSON(ctx, peer+path, data); err != nil {
				g.log.Warn("p2p: broadcast failed", "peer", peer, "path", path, "err", err)
			}
		}()
	}
}

// postJSON performs a single HTTP POST with a JSON body. It creates its own
// context with broadcastTimeout so that a slow peer is abandoned promptly.
func (g *Gossiper) postJSON(_ context.Context, url string, body []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), broadcastTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("p2p: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("p2p: do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("p2p: peer returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// ── public broadcast API ──────────────────────────────────────────────────────

// BroadcastTx marks tx as seen locally and fans it out to all peers at
// POST /p2p/tx. If tx has already been seen it is a no-op (duplicate
// suppression).
func (g *Gossiper) BroadcastTx(ctx context.Context, tx *types.Transaction) {
	txID := tx.TxID()
	if g.markSeen(txID) {
		return
	}
	g.broadcastJSON(ctx, "/p2p/tx", tx)
}

// BroadcastBlock marks block as seen locally and fans it out to all peers at
// POST /p2p/block. If block has already been seen it is a no-op.
func (g *Gossiper) BroadcastBlock(ctx context.Context, block *types.Block) {
	hash := block.BlockHash()
	if g.markSeen(hash) {
		return
	}
	g.broadcastJSON(ctx, "/p2p/block", block)
}

// ── inbound handlers (called by api/server.go) ────────────────────────────────

// ReceiveTx is called by the HTTP endpoint when a peer pushes a transaction.
// It deduplicates via the seen-cache, calls cb.HandleReceivedTx, then
// re-broadcasts to other peers so the gossip propagates through the network.
func (g *Gossiper) ReceiveTx(ctx context.Context, tx *types.Transaction) error {
	txID := tx.TxID()
	if g.markSeen(txID) {
		return nil // already processed; do not re-broadcast
	}
	if g.cb != nil {
		if err := g.cb.HandleReceivedTx(ctx, tx); err != nil {
			// Log but do not fail: the transaction may already be in the
			// mempool (ErrDuplicateTx) or invalid for policy reasons.
			g.log.Debug("p2p: HandleReceivedTx", "txid", txID, "err", err)
		}
	}
	// Re-broadcast to other peers (fan-out propagation).
	g.broadcastJSON(ctx, "/p2p/tx", tx)
	return nil
}

// ReceiveBlock is called by the HTTP endpoint when a peer pushes a block.
// It deduplicates via the seen-cache, calls cb.HandleReceivedBlock, and
// re-broadcasts on success. Orphan / parent-missing blocks are discarded
// silently.
func (g *Gossiper) ReceiveBlock(ctx context.Context, block *types.Block) error {
	hash := block.BlockHash()
	if g.markSeen(hash) {
		return nil
	}
	if g.cb != nil {
		if err := g.cb.HandleReceivedBlock(ctx, block); err != nil {
			g.log.Debug("p2p: HandleReceivedBlock", "hash", hash, "err", err)
			// Do not re-broadcast invalid or orphan blocks.
			return nil
		}
	}
	g.broadcastJSON(ctx, "/p2p/block", block)
	return nil
}
