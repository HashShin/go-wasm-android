// Dev web server with hot reload.
// Serves assets/ from disk (live), compiles WASM on Go file changes,
// and pushes a reload signal to the browser via SSE.
package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ── configuration ─────────────────────────────────────────────────────────────

var (
	assetsDir string // e.g. ./assets
	golibDir  string // e.g. ./golib
	wasmOut   string // temp wasm path served to browser
)

// ── SSE hot-reload hub ────────────────────────────────────────────────────────

var hub = &reloadHub{}

type reloadHub struct {
	mu      sync.Mutex
	clients []chan string
}

func (h *reloadHub) add() chan string {
	ch := make(chan string, 1)
	h.mu.Lock()
	h.clients = append(h.clients, ch)
	h.mu.Unlock()
	return ch
}

func (h *reloadHub) remove(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, c := range h.clients {
		if c == ch {
			h.clients = append(h.clients[:i], h.clients[i+1:]...)
			return
		}
	}
}

func (h *reloadHub) broadcast(msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.clients {
		select {
		case ch <- msg:
		default:
		}
	}
	if len(h.clients) > 0 {
		log.Printf("[hot] → reload (%d client(s))  %s", len(h.clients), msg)
	}
}

// ── WASM compiler ─────────────────────────────────────────────────────────────

var wasmMu sync.Mutex

func buildWasm() error {
	wasmMu.Lock()
	defer wasmMu.Unlock()

	log.Println("[wasm] compiling...")
	mainFile := filepath.Join(golibDir, "main.go")
	cmd := exec.Command("go", "build", "-o", wasmOut, mainFile)
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("[wasm] build FAILED: %v", err)
		return err
	}
	log.Printf("[wasm] build OK  (%s)", fileSize(wasmOut))
	return nil
}

// ── file watcher (polling) ────────────────────────────────────────────────────

type watchState struct {
	mu    sync.Mutex
	mtimes map[string]time.Time
}

func newWatchState() *watchState {
	return &watchState{mtimes: make(map[string]time.Time)}
}

func (ws *watchState) changed(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	ws.mu.Lock()
	defer ws.mu.Unlock()
	prev, ok := ws.mtimes[path]
	ws.mtimes[path] = info.ModTime()
	return !ok || info.ModTime().After(prev)
}

func collectFiles(dirs []string, exts []string) []string {
	var out []string
	for _, dir := range dirs {
		filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			for _, ext := range exts {
				if strings.HasSuffix(p, ext) {
					out = append(out, p)
					break
				}
			}
			return nil
		})
	}
	return out
}

func watch() {
	ws := newWatchState()

	// Seed initial state (no reload on first run)
	goFiles := collectFiles([]string{golibDir}, []string{".go"})
	webFiles := collectFiles([]string{assetsDir}, []string{".html", ".css", ".js"})
	for _, f := range append(goFiles, webFiles...) {
		ws.changed(f)
	}

	log.Println("[watch] watching for changes...")

	for range time.Tick(400 * time.Millisecond) {
		goFiles  = collectFiles([]string{golibDir}, []string{".go"})
		webFiles = collectFiles([]string{assetsDir}, []string{".html", ".css", ".js"})

		goChanged := false
		for _, f := range goFiles {
			if ws.changed(f) {
				log.Printf("[watch] Go changed: %s", filepath.Base(f))
				goChanged = true
			}
		}

		webChanged := false
		for _, f := range webFiles {
			if ws.changed(f) {
				log.Printf("[watch] asset changed: %s", filepath.Base(f))
				webChanged = true
			}
		}

		if goChanged {
			if err := buildWasm(); err == nil {
				hub.broadcast("go+assets")
			}
		} else if webChanged {
			hub.broadcast("assets")
		}
	}
}

// ── HTTP handlers ─────────────────────────────────────────────────────────────

// hotHandler is the SSE endpoint — browser connects here and waits for reload.
func hotHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	flusher.Flush()

	ch := hub.add()
	defer hub.remove(ch)

	select {
	case msg := <-ch:
		fmt.Fprintf(w, "event: reload\ndata: %s\n\n", msg)
		flusher.Flush()
	case <-r.Context().Done():
	}
}

// indexHandler serves index.html with the hot-reload SSE script injected.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(filepath.Join(assetsDir, "index.html"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Inject hot-reload script just before </body>
	const script = `<script>
/* dev hot-reload */
(function(){
  function connect(){
    var es = new EventSource('/hot');
    es.addEventListener('reload', function(){ location.reload(); });
    es.onerror = function(){ es.close(); setTimeout(connect, 1000); };
    console.log('[dev] hot reload connected');
  }
  connect();
})();
</script>
`
	data = bytes.Replace(data, []byte("</body>"), []byte(script+"</body>"), 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(data)
}

// wasmHandler serves the freshly compiled app.wasm.
func wasmHandler(w http.ResponseWriter, r *http.Request) {
	wasmMu.Lock()
	defer wasmMu.Unlock()
	w.Header().Set("Content-Type", "application/wasm")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, wasmOut)
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	// Project root passed as first argument by web.sh
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	assetsDir = filepath.Join(root, "app", "ui")
	golibDir  = filepath.Join(root, "app", "go")
	wasmOut   = filepath.Join(os.TempDir(), "gowebapp-dev.wasm")

	// Initial WASM build
	if err := buildWasm(); err != nil {
		log.Fatal("initial WASM build failed — fix errors and restart")
	}

	// Copy wasm_exec.js to a temp file so we can serve it
	wasmExecSrc := wasmExecPath()
	wasmExecData, err := os.ReadFile(wasmExecSrc)
	if err != nil {
		log.Fatalf("wasm_exec.js not found at %s", wasmExecSrc)
	}

	go watch()

	mux := http.NewServeMux()
	mux.HandleFunc("/hot",           hotHandler)
	mux.HandleFunc("/app.wasm",      wasmHandler)
	mux.HandleFunc("/wasm_exec.js",  func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(wasmExecData)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			indexHandler(w, r)
			return
		}
		// Serve other assets (style.css, etc.) directly from disk, no-cache
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, filepath.Join(assetsDir, filepath.Clean(r.URL.Path)))
	})

	addr := "127.0.0.1:7000"
	log.Printf("[dev] http://%s  (hot reload enabled)", addr)
	log.Printf("[dev] edit app/ui/ or app/go/ — browser refreshes automatically")
	log.Fatal(http.ListenAndServe(addr, mux))
}

// ── helpers ───────────────────────────────────────────────────────────────────

func wasmExecPath() string {
	cmd := exec.Command("go", "env", "GOROOT")
	out, _ := cmd.Output()
	root := strings.TrimSpace(string(out))
	// Go 1.21+ puts it in lib/wasm/; older versions use misc/wasm/
	for _, rel := range []string{"lib/wasm/wasm_exec.js", "misc/wasm/wasm_exec.js"} {
		p := filepath.Join(root, rel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	log.Fatal("wasm_exec.js not found in GOROOT")
	return ""
}

func fileSize(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "?"
	}
	mb := float64(info.Size()) / 1024 / 1024
	return fmt.Sprintf("%.1fM", mb)
}

func init() {
	// Log without date prefix for cleaner output
	log.SetFlags(0)
}

// Ensure io is used
var _ = io.Discard
