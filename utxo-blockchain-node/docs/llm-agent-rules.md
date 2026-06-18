# LLM Agent Rules and Best Practices

This document establishes mandatory rules and best practices for LLM agents (and humans) continuing development on this UTXO blockchain node.

## Mandatory Rules

**These rules must never be violated. When a rule conflicts with a feature request, refuse the request and explain why.**

### 1. Do Not Change Consensus Encoding Without Updating Tests

**Rule:** Any change to `types.go` canonical encoding (hash functions, struct serialization, endianness) must be accompanied by:
- ✅ Updated or new tests in `*_test.go`
- ✅ Verification that test vectors still pass
- ✅ Documentation of the encoding change in the file header

**Why:** Consensus encoding is the foundation of blockchain integrity. A single byte wrong breaks all chain compatibility.

**Example violation:**
```go
// ❌ BAD: Changed hash calculation without test update
func (h Hash32) SetHex(hexStr string) error {
    // Changed from little-endian to big-endian
    // BUT: did not update tests
}
```

**Example compliance:**
```go
// ✅ GOOD: Change with test coverage
func (h Hash32) SetHex(hexStr string) error {
    // ... implementation ...
}

func TestHash32SetHex(t *testing.T) {
    tests := []struct {
        hexStr string
        want   Hash32
    }{
        {"0102....", Hash32{0x01, 0x02, ...}},
    }
    // ... validate every case ...
}
```

---

### 2. Do Not Put Consensus Rules in API Handlers

**Rule:** All consensus validation belongs in `internal/consensus/`, not in `internal/api/`.

**Allowed in API handlers:**
- ✅ Input validation (JSON parsing, address format)
- ✅ Error response formatting
- ✅ HTTP status code selection

**Forbidden in API handlers:**
- ❌ Checking if a transaction is valid (belongs in `consensus.ValidateTx`)
- ❌ Checking if a block is valid (belongs in `consensus.ValidateBlock`)
- ❌ Checking if a UTXO can be spent (belongs in `consensus.ValidateTx`)
- ❌ Computing merkle root or block hash (belongs in `types.go`)

**Why:** Separating consensus from API ensures the same rules apply everywhere (mempool, P2P, mining) and prevents inconsistency bugs.

**Example violation:**
```go
// ❌ BAD: Consensus logic in API handler
func (s *Server) handleSubmitTx(w http.ResponseWriter, r *http.Request) {
    // Parse JSON
    var tx types.Transaction
    json.NewDecoder(r.Body).Decode(&tx)
    
    // ❌ Consensus check here!
    if len(tx.Inputs) == 0 {
        http.Error(w, "No inputs", http.StatusBadRequest)
        return
    }
    
    // Should be in consensus.ValidateTx instead
}
```

**Example compliance:**
```go
// ✅ GOOD: Consensus in services, API just calls it
func (s *Server) handleSubmitTx(w http.ResponseWriter, r *http.Request) {
    var tx types.Transaction
    json.NewDecoder(r.Body).Decode(&tx)
    
    // Call service which calls consensus layer
    err := s.svc.SubmitTx(&tx)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    w.WriteHeader(http.StatusOK)
}

// In services.go:
func (svc *Services) SubmitTx(tx *types.Transaction) error {
    utxoView := svc.db  // or mempool UTXO view
    return consensus.ValidateTx(tx, utxoView)
}
```

---

### 3. Do Not Use JSON for TxID or Block Hash

**Rule:** Transaction IDs and block hashes are always serialized as **hex strings** (or binary), never as JSON objects or numbers.

**Correct serialization:**
- ✅ `"txid": "a1b2c3d4..."` (hex string)
- ✅ `types.Hash32` computed via `types.Transaction.TxID()` (binary)

**Forbidden:**
- ❌ `"txid": {"bytes": [...]}`
- ❌ `"txid": 12345` (number)
- ❌ JSON-encoded double-SHA256 (use canonical binary)

**Why:** Consensus requires deterministic hashing. JSON parsing is fragile and language-dependent. Hex strings are human-readable. Canonical binary is machine-friendly.

**Example violation:**
```go
// ❌ BAD: JSON object for hash
type TxResponse struct {
    TxID struct {
        Bytes []byte `json:"bytes"`
    } `json:"txid"`
}

// ❌ BAD: JSON number
type BlockResponse struct {
    BlockHash uint64 `json:"block_hash"`
}
```

**Example compliance:**
```go
// ✅ GOOD: Hex string
type TxResponse struct {
    TxID string `json:"txid"`  // hex encoded
}

func (svc *Services) SubmitTx(tx *types.Transaction) error {
    resp := TxResponse{
        TxID: tx.TxID().String(),  // calls Hash32.String() → hex
    }
}

// ✅ GOOD: Binary canonical encoding
resp := struct{
    Data []byte
}{
    Data: tx.CanonicalEncode(),  // raw bytes, not JSON
}
```

---

### 4. Do Not Skip Error Handling

**Rule:** Every function call that returns an error must be checked. Use one of:

1. **Direct check (most common):**
   ```go
   err := SomeFunc()
   if err != nil {
       return nil, err  // or handle and wrap
   }
   ```

2. **Unwrap and check specific errors:**
   ```go
   if err := SomeFunc(); err != nil {
       if errors.Is(err, io.EOF) {
           // Handle EOF specifically
       } else {
           return err
       }
   }
   ```

3. **Panic only for truly unrecoverable failures:**
   ```go
   if err != nil {
       panic("database corruption")  // rare
   }
   ```

4. **Discard only if error is truly impossible:**
   ```go
   _ = svc.Close()  // Close never fails in our design
   ```

**Forbidden:**
- ❌ `SomeFunc()` without checking result
- ❌ `_ = SomeFunc()` without a comment explaining why it's safe to ignore
- ❌ Empty error handling blocks: `if err != nil {}`

**Why:** Silently dropping errors leads to data corruption, confusing state, and hard-to-debug production failures.

**Example violation:**
```go
// ❌ BAD: Error ignored
db.SaveBlock(block)  // what if it fails? block not persisted, then reorg uses stale data

// ❌ BAD: Empty error handling
err := consensus.ValidateTx(tx, utxoView)
if err != nil {
    // no action - now mempool has invalid tx
}
```

**Example compliance:**
```go
// ✅ GOOD: Explicit check
if err := db.SaveBlock(block); err != nil {
    return fmt.Errorf("save block: %w", err)
}

// ✅ GOOD: Specific handling
if err := consensus.ValidateTx(tx, utxoView); err != nil {
    return fmt.Errorf("invalid tx: %w", err)
}

// ✅ GOOD: Comment explaining why safe to ignore
_ = log.Close()  // log.Close returns nil in current implementation
```

---

### 5. Every Milestone Must Add Tests

**Rule:** Every milestone or feature addition must include tests. No feature is complete without tests.

**Test requirements by package:**

| Package | Current Tests | Requirement |
|---------|---------------|-------------|
| `internal/types` | 40 | Add 1+ test per new type or method |
| `internal/crypto` | 21 | Add 1+ test per new hash function |
| `internal/storage` | 18 | Add 1+ test per new bucket or operation |
| `internal/consensus` | 32 | Add 1+ test per new rule (valid + invalid cases) |
| `internal/chain` | 30 | Add 1+ test per new chain operation |
| `internal/mempool` | 14 | Add 1+ test per new eviction/validation rule |
| `internal/api` | 29 | Add 1+ test per new endpoint |
| `internal/p2p` | 17 | Add 1+ test per new broadcast/gossip behavior |

**Minimum test coverage:**
- Happy path: feature works as intended
- Error path: feature fails gracefully when given bad input
- Edge case: boundary conditions (empty, max size, etc.)

**Why:** Tests are your regression suite. Without them, future changes break existing functionality silently.

**Example violation:**
```go
// ❌ BAD: Added new consensus rule without test
func ValidateTx(...) error {
    // Added: check that outputs don't exceed 21M coins total
    // But: no test case for this new rule
}
```

**Example compliance:**
```go
// ✅ GOOD: New rule with comprehensive tests
func ValidateTx(...) error {
    // Added: check that outputs don't exceed 21M coins total
}

// In types_test.go:
func TestValidateTx_RejectExceedsMaxSupply(t *testing.T) {
    tx := &types.Transaction{
        Outputs: []types.TxOutput{
            {Value: 21_000_000},
            {Value: 1},  // exceeds 21M
        },
    }
    err := consensus.ValidateTx(tx, view)
    if err == nil {
        t.Error("expected error for exceeds max supply")
    }
}

func TestValidateTx_AcceptAtMaxSupply(t *testing.T) {
    tx := &types.Transaction{
        Outputs: []types.TxOutput{
            {Value: 21_000_000},
        },
    }
    err := consensus.ValidateTx(tx, view)
    if err != nil {
        t.Errorf("expected no error at max supply, got %v", err)
    }
}
```

---

### 6. Reorg Logic Must Use Undo Records

**Rule:** When disconnecting blocks during a reorg, **always** use the undo records (`BlockUndo`) to restore UTXO state. Never re-execute the block or recalculate UTXOs.

**Correct approach:**
```go
// ✅ GOOD: Disconnect using undo record
func disconnectBlock(tx *WriteTx, blockIdx *BlockIndex) error {
    undo, err := tx.GetUndo(blockIdx.Hash)  // Get undo from storage
    if err != nil {
        return err
    }
    
    // Restore outputs in REVERSE order (important!)
    for i := len(undo.Spent) - 1; i >= 0; i-- {
        if err := tx.PutUTXO(undo.Spent[i]); err != nil {
            return err
        }
    }
    
    return nil
}
```

**Forbidden approach:**
```go
// ❌ BAD: Re-executing block
func disconnectBlock(block *Block) error {
    for _, tx := range block.Transactions {
        // Re-validating? Re-computing hashes? No!
        // This is slow and can yield different results
        // due to timestamp, signature verification order, etc.
    }
}
```

**Why:** Undo records are computed once at connection time and stored atomically. Re-execution is:
- Expensive (re-validate every input)
- Non-deterministic (floating point? external state?)
- Fragile (consensus rule changes break old blocks)

**See also:** [docs/reorgs.md](./reorgs.md)

---

### 7. Keep Packages Small and Dependency Direction Clean

**Rule:** Follow Go package principles:

1. **Acyclic dependencies:** Packages must form a DAG (directed acyclic graph). No package imports a package that imports it.
   ```
   consensus ← chain ← node
   ↑
   types ← consensus
   ```

2. **Single responsibility:** Each package has one clear purpose:
   - `types`: data structures only (no logic)
   - `crypto`: cryptographic operations only
   - `consensus`: validation rules only
   - `storage`: persistence only
   - `chain`: block chain logic only
   - `mempool`: transaction pool only
   - `api`: HTTP only
   - `p2p`: network gossip only

3. **Small interfaces:** Don't export large monolithic interfaces. Break them into smaller focused interfaces:
   ```go
   // ✅ GOOD: Focused interface
   type UTXOGetter interface {
       GetUTXO(op OutPoint) (*UTXO, error)
   }
   
   // ❌ BAD: Kitchen sink
   type Everything interface {
       GetUTXO(...)
       SaveBlock(...)
       ValidateTx(...)
       BroadcastTx(...)
       // ... 20 more methods
   }
   ```

4. **No circular imports:** If package A imports package B, then B cannot import A.

**Example violation:**
```go
// ❌ BAD: Circular dependency
// storage/db.go imports chain package
import "github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/chain"

// chain/chain.go imports storage package
import "github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/storage"

// This creates a cycle: storage → chain → storage
```

**Example compliance:**
```
// ✅ GOOD: Clean DAG
types (lowest level, no imports except stdlib)
  ↑
crypto (imports types only)
  ↑
consensus (imports types, crypto)
  ↑
storage (imports types)
  ↑
chain (imports types, storage, consensus)
  ↑
mempool (imports types, storage, consensus)
  ↑
node services (imports all above, no reverse imports)
  ↑
api handlers (imports services, types)
  ↑
p2p (imports types, api, node services)
```

---

## Best Practices for LLM Agents

### Before Making Changes

1. **Read the existing code** in the relevant package to understand the pattern and conventions.
2. **Run all tests** to establish a baseline: `go test ./... -count=1`
3. **Identify the failing test(s)** or feature requirement
4. **Create a todo list** of changes needed

### During Implementation

1. **Maintain test baseline:** Changes must not reduce test count or pass rate
2. **Follow naming conventions:** Use camelCase for variables, PascalCase for exported functions
3. **Add comments for complex logic:** Especially in consensus code
4. **Keep functions small:** If a function exceeds ~30 lines, consider splitting it
5. **Use table-driven tests** for multiple input cases

### After Implementation

1. **Run `go fmt ./...`** to format code
2. **Run `go vet ./...`** to check for common mistakes
3. **Run `go test ./... -count=1`** to verify all tests pass
4. **Document breaking changes** in commit message and code comments

### When Debugging

1. **Add temporary print statements** using `log.Println`, then remove after debugging
2. **Run single test:** `go test -run TestName ./package`
3. **Check error types:** Use `errors.Is()` or `errors.As()` to understand failures
4. **Review the related test file:** `*_test.go` often contains examples of correct usage

---

## Code Style Guidelines

### Comments

- Block comments for package documentation
- Line comments for code explanation (place above the line)
- Uncommented code is self-explanatory (good naming)

```go
// MyFunc does X and returns Y.
func MyFunc() (Y, error) {
    // Check precondition
    if len(inputs) == 0 {
        return nil, errors.New("empty inputs")
    }
    // ... implementation
}
```

### Variable Names

- Short names for loop variables: `for i, tx := range txs { }`
- Descriptive names for return values: `func GetBlock(...) (*Block, error)`
- Avoid abbreviations unless universal: `num` → `count`, `tx` → `transaction` (use `tx` for blockchain context)

### Error Messages

- Lowercase, no period: `"invalid hash"`
- Include context: `"validate tx: invalid signature"`
- Wrap with `fmt.Errorf(...%w...)` to preserve stack

```go
if err := consensus.ValidateTx(tx, utxoView); err != nil {
    return fmt.Errorf("submit tx: %w", err)  // wraps error
}
```

### Constants and Enums

- All caps with underscores: `INITIAL_SUBSIDY`, `HALVING_INTERVAL`
- Group related constants in blocks

```go
const (
    InitialSubsidy   = 50_000_000
    HalvingInterval  = 210_000
    MaxBlockSize     = 4_000_000
)
```

---

## Testing Checklist

Before submitting code for review:

- [ ] All new functions have unit tests
- [ ] Happy path tests pass
- [ ] Error path tests pass  
- [ ] Edge case tests pass (empty, max, etc.)
- [ ] Table-driven tests cover all cases
- [ ] Test names are descriptive
- [ ] `go test ./... -count=1` passes
- [ ] `go vet ./...` produces no output
- [ ] `go fmt ./...` changes nothing
- [ ] No test files are skipped (`skip()` only for known flakiness)

---

## When in Doubt

1. **Check the nearest existing test** for the pattern and conventions
2. **Read the package `_test.go`** to see how similar features are tested
3. **Look at Milestone 12 code** (recent, high-quality reference)
4. **Check `docs/consensus.md`** for consensus rules
5. **Check `docs/reorgs.md`** for reorg logic
6. **Check `docs/api.md`** for API patterns

---

## Contacts / Future Handoff

If you are a human inheriting this codebase:

1. Start with [README.md](../README.md) to understand the project
2. Read [docs/consensus.md](./consensus.md) to understand blockchain rules
3. Read [docs/testing.md](./testing.md) to understand how to test
4. Run `go test ./... -v` to see all tests and their status
5. Pick a small milestone and follow the pattern from Milestone 1-12

If you are an LLM agent, follow the rules above religiously. The codebase is designed for incremental safe changes. Trust the tests.

---

**Last updated:** After Milestone 12 (all 219 tests passing)