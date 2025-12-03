// ------------------------------------------------------------
// 🌟 AlirezaChain PoS Node
// Author: Alireza
// Description: Minimal proof-of-stake blockchain node with HTTP API.
//              Validators stake tokens and are selected to forge blocks
//              proportionally to their stake.
// ------------------------------------------------------------

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"math/big"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

const (
	posName   = "AlirezaChain PoS"
	posBanner = "🪙 " + posName + " 🪙"
)

// StakeBlock represents a block in the PoS chain.
type StakeBlock struct {
	Height    int    `json:"height"`
	Timestamp int64  `json:"timestamp"`
	Data      string `json:"data"`
	Validator string `json:"validator"`
	Hash      string `json:"hash"`
	PrevHash  string `json:"prevHash"`
}

// BlockView is a user-friendly representation for JSON responses.
type BlockView struct {
	Height    int    `json:"height"`
	Timestamp int64  `json:"timestamp"`
	TimeText  string `json:"time"`
	Data      string `json:"data"`
	Validator string `json:"validator"`
	Hash      string `json:"hash"`
	PrevHash  string `json:"prevHash"`
}

func toView(b StakeBlock) BlockView {
	return BlockView{
		Height:    b.Height,
		Timestamp: b.Timestamp,
		TimeText:  time.Unix(b.Timestamp, 0).Format(time.RFC3339),
		Data:      b.Data,
		Validator: b.Validator,
		Hash:      b.Hash,
		PrevHash:  b.PrevHash,
	}
}

// Global chain and state.
var (
	chain  []StakeBlock
	stakes = make(map[string]uint64) // validator -> stake amount
	mu     sync.RWMutex
)

// computeHash calculates the SHA-256 hash of a block.
func computeHash(b StakeBlock) string {
	record := strconv.Itoa(b.Height) +
		strconv.FormatInt(b.Timestamp, 10) +
		b.Data +
		b.Validator +
		b.PrevHash

	sum := sha256.Sum256([]byte(record))
	return hex.EncodeToString(sum[:])
}

// isBlockValid verifies a new block against the previous block.
func isBlockValid(newB, prevB StakeBlock) bool {
	if newB.Height != prevB.Height+1 {
		return false
	}
	if newB.PrevHash != prevB.Hash {
		return false
	}
	if computeHash(newB) != newB.Hash {
		return false
	}
	return true
}

// isChainValid verifies an entire chain.
func isChainValid(c []StakeBlock) bool {
	if len(c) == 0 {
		return false
	}
	for i := 1; i < len(c); i++ {
		if !isBlockValid(c[i], c[i-1]) {
			return false
		}
	}
	return true
}

// selectValidator chooses a validator based on stake and previous hash.
// The higher the stake, the higher the chance of being selected.
func selectValidator(prev StakeBlock) (string, bool) {
	if len(stakes) == 0 {
		return "", false
	}

	// Build a deterministic seed from previous hash.
	seedBytes := sha256.Sum256([]byte(prev.Hash + "|pos"))
	seedInt := new(big.Int).SetBytes(seedBytes[:])

	// Compute total stake.
	var total uint64 = 0
	for _, s := range stakes {
		total += s
	}
	if total == 0 {
		return "", false
	}

	// Pick a random position in [0, total).
	mod := new(big.Int).Mod(seedInt, big.NewInt(int64(total)))
	target := uint64(mod.Int64())

	// Iterate through validators to find the selected one.
	var cumulative uint64 = 0
	// To make it deterministic, iterate validators in sorted order.
	validators := make([]string, 0, len(stakes))
	for v := range stakes {
		validators = append(validators, v)
	}
	sort.Strings(validators)

	for _, v := range validators {
		cumulative += stakes[v]
		if target < cumulative {
			return v, true
		}
	}
	// Fallback: return last validator if something weird happens.
	return validators[len(validators)-1], true
}

// forgeBlock creates a new block selected by PoS.
func forgeBlock(data string) (StakeBlock, bool) {
	mu.RLock()
	defer mu.RUnlock()

	last := chain[len(chain)-1]
	validator, ok := selectValidator(last)
	if !ok {
		return StakeBlock{}, false
	}

	b := StakeBlock{
		Height:    last.Height + 1,
		Timestamp: time.Now().Unix(),
		Data:      data,
		Validator: validator,
		PrevHash:  last.Hash,
	}
	b.Hash = computeHash(b)
	return b, true
}

// --- HTTP Handlers ---

// getChainHandler returns the full chain.
func getChainHandler(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	defer mu.RUnlock()

	views := make([]BlockView, 0, len(chain))
	for _, b := range chain {
		views = append(views, toView(b))
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(views)
}

// stakeHandler allows adding stake for a validator.
func stakeHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Validator string `json:"validator"`
		Amount    uint64 `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	payload.Validator = strings.TrimSpace(payload.Validator)
	if payload.Validator == "" || payload.Amount == 0 {
		http.Error(w, "validator and positive amount are required", http.StatusBadRequest)
		return
	}

	mu.Lock()
	stakes[payload.Validator] += payload.Amount
	current := stakes[payload.Validator]
	mu.Unlock()

	log.Printf("💰 Stake updated: validator=%s total=%d", payload.Validator, current)

	w.Header().Set("Content-Type", "application/json")
	resp := map[string]interface{}{
		"validator": payload.Validator,
		"total":     current,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
}

// forgeHandler triggers forging a new block using PoS.
func forgeHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Data string `json:"data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	payload.Data = strings.TrimSpace(payload.Data)
	if payload.Data == "" {
		http.Error(w, "data is required", http.StatusBadRequest)
		return
	}

	b, ok := forgeBlock(payload.Data)
	if !ok {
		http.Error(w, "no stake available for forging", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	last := chain[len(chain)-1]
	if !isBlockValid(b, last) {
		http.Error(w, "forged block is not valid", http.StatusInternalServerError)
		return
	}

	chain = append(chain, b)
	log.Printf("🧱 Forged PoS block: height=%d validator=%s hash=%s", b.Height, b.Validator, b.Hash)

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(toView(b))
}

// validatorsHandler returns the current stake distribution.
func validatorsHandler(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	defer mu.RUnlock()

	type ValidatorStake struct {
		Validator string `json:"validator"`
		Stake     uint64 `json:"stake"`
	}
	list := make([]ValidatorStake, 0, len(stakes))
	for v, s := range stakes {
		list := append(list, ValidatorStake{Validator: v, Stake: s})
		_ = list
	}

	// Correction: we need separate slice to fill
	list = list[:0]
	for v, s := range stakes {
		list = append(list, ValidatorStake{Validator: v, Stake: s})
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(list)
}

// infoHandler returns general information about the chain.
func infoHandler(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	defer mu.RUnlock()

	type Info struct {
		Name      string            `json:"name"`
		Blocks    int               `json:"blocks"`
		LastHash  string            `json:"lastHash"`
		Validators map[string]uint64 `json:"validators"`
		Timestamp string            `json:"timestamp"`
	}

	last := chain[len(chain)-1]

	// Copy stakes map to avoid races.
	valCopy := make(map[string]uint64, len(stakes))
	for v, s := range stakes {
		valCopy[v] = s
	}

	resp := Info{
		Name:       posName,
		Blocks:     len(chain),
		LastHash:   last.Hash,
		Validators: valCopy,
		Timestamp:  time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
}

// router sets up all HTTP routes.
func router() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/chain", getChainHandler).Methods("GET")
	r.HandleFunc("/stake", stakeHandler).Methods("POST")
	r.HandleFunc("/forge", forgeHandler).Methods("POST")
	r.HandleFunc("/validators", validatorsHandler).Methods("GET")
	r.HandleFunc("/info", infoHandler).Methods("GET")
	return r
}

func main() {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	// Initialize genesis block.
	genesis := StakeBlock{
		Height:    0,
		Timestamp: time.Now().Unix(),
		Data:      "Genesis 🪙 " + posName,
		Validator: "genesis",
		PrevHash:  "",
	}
	genesis.Hash = computeHash(genesis)

	mu.Lock()
	chain = append(chain, genesis)
	// Optional initial stake for a demo validator.
	stakes["genesis"] = 1
	mu.Unlock()

	addr := ":" + port
	log.Printf("%s", posBanner)
	log.Printf("🚀 PoS node listening on %s", addr)

	if err := http.ListenAndServe(addr, router()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
