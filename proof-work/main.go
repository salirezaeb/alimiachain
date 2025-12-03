// ------------------------------------------------------------
// 🔗 AlirezaChain PoW Node
// Author: Alireza Ebrahimian
// Description: Minimal proof-of-work blockchain node with HTTP API,
//              implemented in Go as part of the AlirezaChain project.
// ------------------------------------------------------------

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"math"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

const (
	chainName   = "AlirezaChain PoW"
	chainBanner = "⛓️  " + chainName + " ⛓️"
)

// Block represents a single block in the PoW blockchain.
type PowBlock struct {
	Height    int    `json:"height"`
	Timestamp int64  `json:"timestamp"`
	Data      string `json:"data"`
	Nonce     int64  `json:"nonce"`
	Hash      string `json:"hash"`
	PrevHash  string `json:"prevHash"`
	Difficulty int   `json:"difficulty"`
}

var powChain []PowBlock

// calculateHash computes the SHA-256 hash for a block.
func calculateHash(b PowBlock) string {
	record := strconv.Itoa(b.Height) +
		strconv.FormatInt(b.Timestamp, 10) +
		b.Data +
		strconv.FormatInt(b.Nonce, 10) +
		b.PrevHash +
		strconv.Itoa(b.Difficulty)

	h := sha256.Sum256([]byte(record))
	return hex.EncodeToString(h[:])
}

// mineBlock performs a simple proof-of-work by finding a hash
// that is below a target defined by the difficulty.
func mineBlock(prev PowBlock, data string, difficulty int) PowBlock {
	var nonce int64 = 0
	target := big.NewInt(1)
	shift := uint(256 - difficulty)
	target.Lsh(target, shift)

	for {
		candidate := PowBlock{
			Height:    prev.Height + 1,
			Timestamp: time.Now().Unix(),
			Data:      data,
			PrevHash:  prev.Hash,
			Nonce:     nonce,
			Difficulty: difficulty,
		}
		hashBytes := sha256.Sum256([]byte(
			strconv.Itoa(candidate.Height) +
				strconv.FormatInt(candidate.Timestamp, 10) +
				candidate.Data +
				strconv.FormatInt(candidate.Nonce, 10) +
				candidate.PrevHash +
				strconv.Itoa(candidate.Difficulty),
		))

		var hashInt big.Int
		hashInt.SetBytes(hashBytes[:])

		if hashInt.Cmp(target) == -1 {
			candidate.Hash = hex.EncodeToString(hashBytes[:])
			log.Printf("🧱 Mined new block: height=%d nonce=%d hash=%s", candidate.Height, candidate.Nonce, candidate.Hash)
			return candidate
		}

		nonce++
		if nonce == math.MaxInt64 {
			log.Println("⚠️  Nonce overflow, restarting mining loop")
			nonce = 0
		}
	}
}

// isBlockValid checks whether a new block is valid compared to the previous one.
func isBlockValid(newBlock, prevBlock PowBlock) bool {
	if newBlock.Height != prevBlock.Height+1 {
		return false
	}
	if newBlock.PrevHash != prevBlock.Hash {
		return false
	}
	if calculateHash(newBlock) != newBlock.Hash {
		return false
	}
	return true
}

// isChainValid validates an entire chain.
func isChainValid(chain []PowBlock) bool {
	if len(chain) == 0 {
		return false
	}
	for i := 1; i < len(chain); i++ {
		if !isBlockValid(chain[i], chain[i-1]) {
			return false
		}
	}
	return true
}

// BlockView is a user-friendly representation of a block.
type BlockView struct {
	Height    int    `json:"height"`
	Timestamp int64  `json:"timestamp"`
	TimeText  string `json:"time"`
	Data      string `json:"data"`
	Nonce     int64  `json:"nonce"`
	Hash      string `json:"hash"`
	PrevHash  string `json:"prevHash"`
	Difficulty int   `json:"difficulty"`
}

func toView(b PowBlock) BlockView {
	return BlockView{
		Height:    b.Height,
		Timestamp: b.Timestamp,
		TimeText:  time.Unix(b.Timestamp, 0).Format(time.RFC3339),
		Data:      b.Data,
		Nonce:     b.Nonce,
		Hash:      b.Hash,
		PrevHash:  b.PrevHash,
		Difficulty: b.Difficulty,
	}
}

// --- HTTP Handlers ---

func getChainHandler(w http.ResponseWriter, r *http.Request) {
	views := make([]BlockView, 0, len(powChain))
	for _, b := range powChain {
		views = append(views, toView(b))
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(views); err != nil {
		http.Error(w, "could not encode chain", http.StatusInternalServerError)
	}
}

func mineHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Data       string `json:"data"`
		Difficulty int    `json:"difficulty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Data) == "" {
		http.Error(w, "data is required", http.StatusBadRequest)
		return
	}
	if payload.Difficulty <= 0 || payload.Difficulty > 24 {
		payload.Difficulty = 18
	}

	last := powChain[len(powChain)-1]
	newBlock := mineBlock(last, payload.Data, payload.Difficulty)

	if isBlockValid(newBlock, last) {
		powChain = append(powChain, newBlock)
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(toView(newBlock))
	} else {
		http.Error(w, "mined block is not valid", http.StatusInternalServerError)
	}
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
	type Info struct {
		Name      string `json:"name"`
		Blocks    int    `json:"blocks"`
		LastHash  string `json:"lastHash"`
		Difficulty int   `json:"defaultDifficulty"`
	}

	last := powChain[len(powChain)-1]

	resp := Info{
		Name:      chainName,
		Blocks:    len(powChain),
		LastHash:  last.Hash,
		Difficulty: 18,
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
}

func makeRouter() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/chain", getChainHandler).Methods("GET")
	r.HandleFunc("/mine", mineHandler).Methods("POST")
	r.HandleFunc("/info", infoHandler).Methods("GET")
	return r
}

func main() {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	genesis := PowBlock{
		Height:    0,
		Timestamp: time.Now().Unix(),
		Data:      "Genesis ⛓️ " + chainName,
		Nonce:     0,
		PrevHash:  "",
		Difficulty: 1,
	}
	genesis.Hash = calculateHash(genesis)
	powChain = append(powChain, genesis)

	addr := ":" + port
	log.Printf("%s", chainBanner)
	log.Printf("⚡ PoW node listening on %s", addr)

	if err := http.ListenAndServe(addr, makeRouter()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
