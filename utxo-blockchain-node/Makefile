# Makefile for utxo-blockchain-node
# Windows-friendly: prefers PowerShell wrappers for OS-specific actions.
# Requires: GNU Make (`choco install make`) and Go 1.22+ on PATH.

# --- Variables ---------------------------------------------------------------

BIN_DIR    := bin
BINARY     := $(BIN_DIR)/node.exe
PKG        := ./...
CMD        := ./cmd/node
SCRIPTS    := scripts

# --- Phony targets -----------------------------------------------------------

.PHONY: help fmt tidy test vet build run clean node1 node2 node3 devnet

help:
	@echo Available targets:
	@echo   fmt     - go fmt $(PKG)
	@echo   tidy    - go mod tidy
	@echo   test    - go test $(PKG) -count=1
	@echo   vet     - go vet $(PKG)
	@echo   build   - compile binary into $(BINARY)
	@echo   run     - run a single local node (defaults from config)
	@echo   clean   - remove bin/ and data/
	@echo   node1   - run node1 via $(SCRIPTS)/run-node1.ps1
	@echo   node2   - run node2 via $(SCRIPTS)/run-node2.ps1
	@echo   node3   - run node3 via $(SCRIPTS)/run-node3.ps1
	@echo   devnet  - print devnet instructions (start node1/node2/node3 in separate shells)

# --- Quality gates -----------------------------------------------------------

fmt:
	go fmt $(PKG)

tidy:
	go mod tidy

test:
	go test $(PKG) -count=1

vet:
	go vet $(PKG)

# --- Build / run -------------------------------------------------------------

build:
	@if not exist $(BIN_DIR) mkdir $(BIN_DIR)
	go build -o $(BINARY) $(CMD)

run:
	go run $(CMD)

# --- Cleanup -----------------------------------------------------------------

clean:
	powershell -NoProfile -ExecutionPolicy Bypass -File $(SCRIPTS)/clean.ps1

# --- Devnet ------------------------------------------------------------------

node1:
	powershell -NoProfile -ExecutionPolicy Bypass -File $(SCRIPTS)/run-node1.ps1

node2:
	powershell -NoProfile -ExecutionPolicy Bypass -File $(SCRIPTS)/run-node2.ps1

node3:
	powershell -NoProfile -ExecutionPolicy Bypass -File $(SCRIPTS)/run-node3.ps1

devnet:
	@echo Start a 3-node local devnet by opening three terminals and running:
	@echo   Terminal 1: make node1
	@echo   Terminal 2: make node2
	@echo   Terminal 3: make node3
	@echo.
	@echo Then verify gossip by mining on node1 and querying /status on node2 / node3:
	@echo   curl -X POST http://127.0.0.1:8001/mine
	@echo   curl http://127.0.0.1:8002/status
	@echo   curl http://127.0.0.1:8003/status
