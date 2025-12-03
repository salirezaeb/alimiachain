# 🚀 AlimiaChain   
A Minimal Blockchain Architecture Demonstration (PoW + PoS + P2P)

AlimiaChain is an educational blockchain project implemented in Go, showcasing three independent consensus and networking models:

- ⛏ **Proof of Work (PoW)** — hash-based computational mining  
- 🪙 **Proof of Stake (PoS)** — stake-driven validator selection  
- 🌐 **Peer-to-Peer (P2P)** — distributed chain synchronization  

Each system is implemented as a standalone node with an HTTP API, allowing developers to run, test, compare, and study blockchain mechanics in isolation.

## 🚀 Getting Started

Clone the repository and install dependencies:

```bash
git clone https://github.com/salirezaeb/alimiachain.git
cd alimiachain
go mod tidy
```
## 📦 Requirements

To run AlimiaChain, make sure the following dependencies are installed:

- **Go 1.20+** — required to run all three blockchain nodes  
- **Git** — required for cloning the repository  
- **Terminal environment**  
  - Linux  
  - macOS  
  - Windows Subsystem for Linux (WSL)

Each module (**PoW**, **PoS**, **P2P**) is an independent Go program and can be executed using:

```bash
go run main.go
```

## ⛏ Proof of Work (PoW) 

**Directory:** `proof-work/`  
**Entry file:** `main.go`  
**Default Port:** `8081`  
**Environment Variable:** `PORT` (optional)

The PoW node implements a minimal proof-of-work blockchain.  
Blocks are mined by computing SHA-256 hashes until a target difficulty is met.

---

### 🧱 Block Structure

```go
type PowBlock struct {
    Height     int    `json:"height"`
    Timestamp  int64  `json:"timestamp"`
    Data       string `json:"data"`
    Nonce      int64  `json:"nonce"`
    Hash       string `json:"hash"`
    PrevHash   string `json:"prevHash"`
    Difficulty int    `json:"difficulty"`
}
```
### 🔗 Genesis Block

When the node starts, it automatically constructs a **genesis block** with the following properties:

- **Height:** `0`  
- **Data:** `Genesis ⛓️ AlirezaChain PoW`  
- **Nonce:** `0`  
- **PrevHash:** *(empty)*  
- **Difficulty:** `1`  
- **Hash:** calculated using SHA-256  

The genesis block serves as the **root of the blockchain**, and every subsequent block must reference it (directly or indirectly).

---

#### ⛏ Mining Loop Overview

1. Start with `nonce = 0`  
2. Build a candidate block  
3. Compute its SHA-256 hash  
4. If hash meets the target → block is mined  
5. Otherwise, increment `nonce` and repeat  

#### ✅ Block Validation Rules

A mined block is considered valid if:

- `Height(new) = Height(prev) + 1`  
- `PrevHash(new) = Hash(prev)`  
- `calculateHash(new) == new.Hash`  

Only valid blocks are appended to the chain.

### 🎥 PoW Demonstration

The video below provides an operational overview of the PoW module, demonstrating how blocks are mined, validated, and appended to the chain based on the configured difficulty.



https://github.com/user-attachments/assets/2ae84860-9afd-4596-a416-e852f438058a

---

## 🪙 Proof of Stake (PoS) 

**Directory:** `proof-stake/`  
**Entry file:** `main.go`  
**Default Port:** `8082`  
**Environment Variable:** `PORT` (optional)

The PoS node implements a minimal stake-based consensus mechanism.  
Validators accumulate stake, and block creation rights are assigned deterministically based on stake weight.

---

### 🧱 Block Structure

```go
type StakeBlock struct {
    Height    int    `json:"height"`
    Timestamp int64  `json:"timestamp"`
    Data      string `json:"data"`
    Validator string `json:"validator"`
    Hash      string `json:"hash"`
    PrevHash  string `json:"prevHash"`
}
```
### 🔗 Genesis Block

When the PoS node starts, it automatically generates a genesis block with the following properties:

- **Height:** `0`  
- **Data:** `Genesis 🪙 AlirezaChain PoS`  
- **Validator:** `"genesis"`  
- **PrevHash:** *(empty)*  
- **Hash:** computed using SHA-256  

Additionally, the `"genesis"` validator is assigned **1 stake unit**, forming the root of the PoS blockchain and enabling the first valid forging operation.

---

### 🎯 Stake-Based Validator Selection

Unlike PoW, the PoS node does not mine blocks. Instead, it selects a validator **proportionally to their stake** through a deterministic algorithm.
### 🧪 Block Validation Rules

A forged PoS block is considered valid if it meets the following criteria:

- `Height(new) = Height(prev) + 1`  
- `PrevHash(new) = Hash(prev)`  
- `computeHash(new) == new.Hash`  

Only blocks that satisfy all validation conditions are appended to the blockchain.

### 🎥 Demonstration Video

The video below provides a concise overview of the system's operation, illustrating the core functionality and behavior in action.



https://github.com/user-attachments/assets/76010705-4065-4e76-9163-3c16ae79c7ae


## 🌐 Peer-to-Peer (P2P) Node

**Directory:** `p2p/`  
**Entry file:** `main.go`  
**Default Port:** `8090`  
**Environment Variables:**  
- `PORT` — overrides the default port  
- `PEERS` — comma-separated list of other node URLs  

The P2P node is responsible for network communication and distributed chain synchronization.  
Each node maintains its own ledger and periodically synchronizes with peers to adopt the longest valid chain.

---

### 🧱 Block Structure

```go
type ChainBlock struct {
    Height    int    `json:"height"`
    Timestamp int64  `json:"timestamp"`
    Data      string `json:"data"`
    Hash      string `json:"hash"`
    PrevHash  string `json:"prevHash"`
}
```
### 🧱 Block Validation & Chain Semantics

All blocks in the P2P node follow the same validation rules used in the PoW and PoS modules:

- The **height** increases sequentially (each new block has `Height = previous.Height + 1`).  
- The **PrevHash** field of a block must match the **Hash** of the previous block.  
- The **hash** of each block must be correctly computed using **SHA-256** over the block fields.  

In addition, the P2P node can expose blocks in a view-friendly structure (often named `BlockView`) for JSON responses, providing human-readable fields such as formatted timestamps.

---

### 🔗 Genesis Block

When the P2P node starts, it creates a **genesis block** with the following properties:

- **Height:** `0`  
- **Data:** `Genesis 🌐 AlirezaChain P2P`  
- **PrevHash:** *(empty)*  
- **Hash:** computed using SHA-256  

This genesis block becomes the **root of the local ledger**.  
Each P2P node independently generates its own genesis block unless it later synchronizes with peers and adopts a different (longer) valid chain.

---

### 🔁 Synchronization Logic

The P2P node periodically exchanges chain data with all configured peers.  
A background synchronization loop runs approximately every **5 seconds** and performs the following steps:

1. Sends `GET /chain` to each peer.  
2. Parses the returned chain from the peer.  
3. Validates the entire peer chain.  
4. If the peer chain is **valid** and **longer** than the local ledger, the local ledger is replaced with the peer’s chain.  

This mechanism ensures that:

- Nodes eventually **converge** on the longest valid chain.  
- The network remains **consistent**, even when some nodes temporarily fall out of sync.  
- New nodes can **catch up automatically** by synchronizing with existing peers.  

There is **no mining or forging** in the P2P node itself — it only manages communication and chain adoption.

---

### 🧪 Block Validation Rules

Any chain or block received from peers must satisfy the following conditions:

- `Height(new) = Height(prev) + 1`  
- `PrevHash(new) = Hash(prev)`  
- `computeHash(new) == new.Hash`  

If **any** block in the received chain fails validation, the **entire peer chain is discarded** and the local ledger remains unchanged.

### 🎥 P2P Demonstration

The video below presents a concise overview of the P2P module, demonstrating how nodes exchange data, synchronize their ledgers, and adopt the longest valid chain during network updates.



https://github.com/user-attachments/assets/eb20be9e-241e-49df-93ac-ec0dd3b4210a



