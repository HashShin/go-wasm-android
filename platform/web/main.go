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
	"strconv"
	"strings"
	"sync"
	"time"
)

// ── configuration ─────────────────────────────────────────────────────────────

var (
	assetsDir string
	golibDir  string
	wasmOut   string
	rootDir   string
)

// ── splash config ─────────────────────────────────────────────────────────────

type splashConfig struct {
	enabled   bool
	bgColor   string
	duration  int
	animation bool
}

func loadSplashConfig() splashConfig {
	enabled := os.Getenv("SPLASH_ENABLED")
	cfg := splashConfig{
		enabled:   enabled != "false",
		bgColor:   envOr("SPLASH_BG_COLOR", "#ffffff"),
		duration:  envInt("SPLASH_DURATION", 2000),
		animation: os.Getenv("SPLASH_ANIMATION") != "false",
	}
	return cfg
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

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
	mu     sync.Mutex
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

	// Also watch splash files
	splashFiles := []string{
		filepath.Join(rootDir, "app", "splash.html"),
		filepath.Join(rootDir, "app", "splash.png"),
		filepath.Join(rootDir, "app", "app.conf"),
	}

	goFiles := collectFiles([]string{golibDir}, []string{".go"})
	webFiles := collectFiles([]string{assetsDir}, []string{".html", ".css", ".js"})
	for _, f := range append(append(goFiles, webFiles...), splashFiles...) {
		ws.changed(f)
	}

	log.Println("[watch] watching for changes...")

	for range time.Tick(400 * time.Millisecond) {
		goFiles = collectFiles([]string{golibDir}, []string{".go"})
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

		for _, f := range splashFiles {
			if ws.changed(f) {
				log.Printf("[watch] splash changed: %s", filepath.Base(f))
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

// indexHandler serves index.html with hot-reload and splash overlay injected.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(filepath.Join(assetsDir, "index.html"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Inject _forceWasm into <head> so it's set before body scripts evaluate IS_DESKTOP
	data = bytes.Replace(data, []byte("</head>"), []byte(`<script>window._forceWasm=true;</script></head>`), 1)

	inject := ""

	// ── Splash overlay ────────────────────────────────────────────────────────
	cfg := loadSplashConfig()
	if cfg.enabled {
		inject += buildSplashOverlay(cfg)
	}

	// ── Hot-reload script ─────────────────────────────────────────────────────
	inject += `<script>
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
	data = bytes.Replace(data, []byte("</body>"), []byte(inject+"</body>"), 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(data)
}

// buildSplashOverlay returns the HTML/JS to inject as a splash overlay.
func buildSplashOverlay(cfg splashConfig) string {
	dismiss := fmt.Sprintf(`<script>
window.SplashBridge={done:function(){
  var el=document.getElementById('_splash');
  if(!el)return;
  el.style.transition='opacity 0.3s ease';
  el.style.opacity='0';
  setTimeout(function(){if(el)el.remove();},300);
}};
setTimeout(function(){SplashBridge.done();}, %d);
</script>`, cfg.duration)

	// Use an iframe if splash.html exists — renders it fully self-contained
	splashHTMLPath := filepath.Join(rootDir, "app", "splash.html")
	if _, err := os.Stat(splashHTMLPath); err == nil {
		return fmt.Sprintf(`<div id="_splash" style="position:fixed;top:0;left:0;width:100%%;height:100%%;z-index:9999;">
<iframe src="/splash.html" style="width:100%%;height:100%%;border:none;display:block;"></iframe>
</div>%s`, dismiss)
	}

	// Native-style overlay from app.conf values
	animStyle := ""
	if cfg.animation {
		animStyle = `<style>
@keyframes _splash_pop{0%{transform:scale(0.8);opacity:0}100%{transform:scale(1);opacity:1}}
#_splash_img{animation:_splash_pop 0.6s cubic-bezier(0.34,1.56,0.64,1) forwards;}
</style>`
	}
	imgSize := envInt("SPLASH_IMAGE_SIZE", 200)
	return fmt.Sprintf(`%s<div id="_splash" style="position:fixed;top:0;left:0;width:100%%;height:100%%;z-index:9999;background:%s;display:flex;align-items:center;justify-content:center;">
<img id="_splash_img" src="/splash_image.png" style="width:%dpx;height:%dpx;object-fit:contain;" onerror="this.style.display='none'" />
</div>%s`, animStyle, cfg.bgColor, imgSize, imgSize, dismiss)
}

func wasmHandler(w http.ResponseWriter, r *http.Request) {
	wasmMu.Lock()
	defer wasmMu.Unlock()
	w.Header().Set("Content-Type", "application/wasm")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, wasmOut)
}

// splashHTMLHandler serves app/splash.html with a SplashBridge shim injected.
func splashHTMLHandler(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(rootDir, "app", "splash.html")
	data, err := os.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// Rewrite Android asset URLs to web-served paths
	data = bytes.ReplaceAll(data, []byte("file:///android_asset/"), []byte("/"))
	// Shim: SplashBridge.done() inside the iframe calls parent.SplashBridge.done()
	shim := `<script>window.SplashBridge={done:function(){try{parent.SplashBridge.done();}catch(e){}}}</script>`
	data = bytes.Replace(data, []byte("</body>"), []byte(shim+"</body>"), 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(data)
}

// splashImageHandler serves app/splash.png or app/Icon.png as splash_image.png.
func splashImageHandler(w http.ResponseWriter, r *http.Request) {
	for _, name := range []string{"splash.png", "Icon.png"} {
		path := filepath.Join(rootDir, "app", name)
		if _, err := os.Stat(path); err == nil {
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "no-store")
			http.ServeFile(w, r, path)
			return
		}
	}
	http.NotFound(w, r)
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	rootDir   = root
	assetsDir = filepath.Join(root, "app", "ui")
	golibDir  = filepath.Join(root, "app", "go")
	wasmOut   = filepath.Join(os.TempDir(), "gowebapp-dev.wasm")

	if err := buildWasm(); err != nil {
		log.Fatal("initial WASM build failed — fix errors and restart")
	}

	wasmExecSrc := wasmExecPath()
	wasmExecData, err := os.ReadFile(wasmExecSrc)
	if err != nil {
		log.Fatalf("wasm_exec.js not found at %s", wasmExecSrc)
	}

	go watch()

	mux := http.NewServeMux()
	mux.HandleFunc("/hot",              hotHandler)
	mux.HandleFunc("/app.wasm",         wasmHandler)
	mux.HandleFunc("/splash.html",      splashHTMLHandler)
	mux.HandleFunc("/splash_image.png", splashImageHandler)
	mux.HandleFunc("/wasm_exec.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(wasmExecData)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			indexHandler(w, r)
			return
		}
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
	log.SetFlags(0)
}

var _ = io.Discard
