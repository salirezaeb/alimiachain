package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

const (
	netName   = "AlirezaChain P2P"
	netBanner = "🌐 " + netName + " 🌐"
)

type ChainBlock struct {
	Height    int    `json:"height"`
	Timestamp int64  `json:"timestamp"`
	Data      string `json:"data"`
	Hash      string `json:"hash"`
	PrevHash  string `json:"prevHash"`
}

var (
	ledger []ChainBlock
	mu     sync.RWMutex

	peers []string
)

// --- Core blockchain logic ---

func computeHash(b ChainBlock) string {
	record := strconv.Itoa(b.Height) +
		strconv.FormatInt(b.Timestamp, 10) +
		b.Data +
		b.PrevHash

	sum := sha256.Sum256([]byte(record))
	return hex.EncodeToString(sum[:])
}

func newBlock(prev ChainBlock, data string) ChainBlock {
	b := ChainBlock{
		Height:    prev.Height + 1,
		Timestamp: time.Now().Unix(),
		Data:      data,
		PrevHash:  prev.Hash,
	}
	b.Hash = computeHash(b)
	return b
}

func isBlockValid(newB, prevB ChainBlock) bool {
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

func isChainValid(chain []ChainBlock) bool {
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

// --- Views ---

type BlockView struct {
	Height    int    `json:"height"`
	Timestamp int64  `json:"timestamp"`
	TimeText  string `json:"time"`
	Data      string `json:"data"`
	Hash      string `json:"hash"`
	PrevHash  string `json:"prevHash"`
}

func toView(b ChainBlock) BlockView {
	return BlockView{
		Height:    b.Height,
		Timestamp: b.Timestamp,
		TimeText:  time.Unix(b.Timestamp, 0).Format(time.RFC3339),
		Data:      b.Data,
		Hash:      b.Hash,
		PrevHash:  b.PrevHash,
	}
}

// --- HTTP Handlers ---

func chainHandler(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	defer mu.RUnlock()

	views := make([]BlockView, 0, len(ledger))
	for _, b := range ledger {
		views = append(views, toView(b))
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(views)
}

func pushHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Data string `json:"data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Data) == "" {
		http.Error(w, "data is required", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	last := ledger[len(ledger)-1]
	nb := newBlock(last, payload.Data)

	if !isBlockValid(nb, last) {
		http.Error(w, "new block is not valid", http.StatusInternalServerError)
		return
	}

	ledger = append(ledger, nb)
	log.Printf("🧱 New local block: height=%d hash=%s", nb.Height, nb.Hash)

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(toView(nb))
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	defer mu.RUnlock()

	type Info struct {
		Name      string   `json:"name"`
		Blocks    int      `json:"blocks"`
		LastHash  string   `json:"lastHash"`
		Peers     []string `json:"peers"`
		Timestamp string   `json:"timestamp"`
	}

	last := ledger[len(ledger)-1]

	resp := Info{
		Name:      netName,
		Blocks:    len(ledger),
		LastHash:  last.Hash,
		Peers:     peers,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
}

func peersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(peers)
}

func makeRouter() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/chain", chainHandler).Methods("GET")
	r.HandleFunc("/push", pushHandler).Methods("POST")
	r.HandleFunc("/info", infoHandler).Methods("GET")
	r.HandleFunc("/peers", peersHandler).Methods("GET")
	return r
}

// --- P2P sync ---

func syncLoop(interval time.Duration) {
	for {
		time.Sleep(interval)
		syncWithPeers()
	}
}

func syncWithPeers() {
	if len(peers) == 0 {
		return
	}

	for _, p := range peers {
		url := strings.TrimRight(p, "/") + "/chain"
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("⚠️  Failed to fetch from peer %s: %v", p, err)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			log.Printf("⚠️  Failed to read response from peer %s: %v", p, err)
			continue
		}

		var peerViews []BlockView
		if err := json.Unmarshal(body, &peerViews); err != nil {
			log.Printf("⚠️  Failed to unmarshal chain from peer %s: %v", p, err)
			continue
		}

		peerChain := make([]ChainBlock, 0, len(peerViews))
		for _, v := range peerViews {
			peerChain = append(peerChain, ChainBlock{
				Height:    v.Height,
				Timestamp: v.Timestamp,
				Data:      v.Data,
				Hash:      v.Hash,
				PrevHash:  v.PrevHash,
			})
		}

		if !isChainValid(peerChain) {
			log.Printf("⚠️  Peer chain from %s is not valid", p)
			continue
		}

		mu.Lock()
		if len(peerChain) > len(ledger) {
			log.Printf("🔄 Adopting longer chain from %s (len=%d > %d)", p, len(peerChain), len(ledger))
			ledger = peerChain
		}
		mu.Unlock()
	}
}

func main() {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	peersEnv := os.Getenv("PEERS")
	if peersEnv != "" {
		for _, p := range strings.Split(peersEnv, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				peers = append(peers, p)
			}
		}
	}

	genesis := ChainBlock{
		Height:    0,
		Timestamp: time.Now().Unix(),
		Data:      "Genesis 🌐 " + netName,
		Hash:      "",
		PrevHash:  "",
	}
	genesis.Hash = computeHash(genesis)

	mu.Lock()
	ledger = append(ledger, genesis)
	mu.Unlock()

	addr := ":" + port
	log.Printf("%s", netBanner)
	log.Printf("📡 Node listening on %s", addr)
	if len(peers) > 0 {
		log.Printf("🤝 Peers: %v", peers)
	}

	go syncLoop(5 * time.Second)

	if err := http.ListenAndServe(addr, makeRouter()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
