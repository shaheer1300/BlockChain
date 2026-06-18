// Package node — demo-only wallet store.
//
// This file is intentionally NOT a production wallet. It exists solely to
// power the educational frontend at /web. Private keys are held in process
// memory (and optionally persisted as plaintext JSON in the data dir) so
// the UI can build and sign sample transactions without shipping a JS
// secp256k1 library. Never reuse this code for anything holding real value.
package node

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/crypto"
	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/types"
)

// DemoWallet pairs a human-readable label with a secp256k1 key and its
// derived address. The private key bytes are exposed only to demo handlers
// that need to sign transactions on the caller's behalf.
type DemoWallet struct {
	Name    string
	Address types.Address
	priv    *crypto.PrivateKey
}

// PubKeyCompressed returns the 33-byte compressed public key used in
// transaction inputs.
func (w *DemoWallet) PubKeyCompressed() []byte {
	return w.priv.PubKey().SerializeCompressed()
}

// SignInput signs the given transaction input on behalf of this wallet.
func (w *DemoWallet) SignInput(tx *types.Transaction, inputIndex int) ([]byte, error) {
	return crypto.SignInput(tx, inputIndex, w.priv)
}

// walletStore is a thread-safe in-memory map of name → wallet with
// optional JSON persistence. Reset() returns the store to its empty state.
type walletStore struct {
	mu      sync.RWMutex
	wallets map[string]*DemoWallet
	path    string // empty means no persistence
}

// newWalletStore creates a store backed by path. If path is non-empty and
// the file exists, wallets are loaded eagerly.
func newWalletStore(path string) (*walletStore, error) {
	s := &walletStore{
		wallets: make(map[string]*DemoWallet),
		path:    path,
	}
	if path == "" {
		return s, nil
	}
	if err := s.load(); err != nil {
		return nil, fmt.Errorf("walletStore: load %q: %w", path, err)
	}
	return s, nil
}

// Create generates a new keypair, stores it under name, and returns the
// wallet. Returns an error if name is already used.
func (s *walletStore) Create(name string) (*DemoWallet, error) {
	if name == "" {
		return nil, errors.New("walletStore: name is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.wallets[name]; ok {
		return nil, fmt.Errorf("walletStore: wallet %q already exists", name)
	}
	priv, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("walletStore: generate key: %w", err)
	}
	w := &DemoWallet{
		Name:    name,
		Address: crypto.PubKeyToAddress(priv.PubKey()),
		priv:    priv,
	}
	s.wallets[name] = w
	if err := s.persist(); err != nil {
		return nil, err
	}
	return w, nil
}

// Get returns the named wallet, or nil if not found.
func (s *walletStore) Get(name string) *DemoWallet {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.wallets[name]
}

// List returns a name-sorted snapshot of all wallets.
func (s *walletStore) List() []*DemoWallet {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*DemoWallet, 0, len(s.wallets))
	for _, w := range s.wallets {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Reset removes all wallets from memory and persistence.
func (s *walletStore) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wallets = make(map[string]*DemoWallet)
	return s.persist()
}

// ── persistence ───────────────────────────────────────────────────────────────

type walletRecord struct {
	Name    string `json:"name"`
	PrivHex string `json:"priv_hex"`
}

func (s *walletStore) persist() error {
	if s.path == "" {
		return nil
	}
	records := make([]walletRecord, 0, len(s.wallets))
	for _, w := range s.wallets {
		records = append(records, walletRecord{
			Name:    w.Name,
			PrivHex: hex.EncodeToString(w.priv.Serialize()),
		})
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Name < records[j].Name })

	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

func (s *walletStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var records []walletRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return err
	}
	for _, r := range records {
		b, err := hex.DecodeString(r.PrivHex)
		if err != nil {
			return fmt.Errorf("decode priv_hex for %q: %w", r.Name, err)
		}
		priv, err := crypto.PrivateKeyFromBytes(b)
		if err != nil {
			return fmt.Errorf("parse priv key for %q: %w", r.Name, err)
		}
		s.wallets[r.Name] = &DemoWallet{
			Name:    r.Name,
			Address: crypto.PubKeyToAddress(priv.PubKey()),
			priv:    priv,
		}
	}
	return nil
}
