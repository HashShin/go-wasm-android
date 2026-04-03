//go:build js && wasm

package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"syscall/js"
	"time"
)

// ── existing functions ────────────────────────────────────────────────────────

func greet(_ js.Value, args []js.Value) any {
	user := "World"
	if len(args) > 0 {
		user = args[0].String()
	}
	return fmt.Sprintf("Hello, %s! This is Go (WASM) speaking.", user)
}

func process(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(args[0].String()))
}

func timestamp(_ js.Value, _ []js.Value) any {
	return fmt.Sprintf("%d", time.Now().Unix())
}

func add(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return "0"
	}
	return fmt.Sprintf("%d", args[0].Int()+args[1].Int())
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

// newPromise wraps a Go function in a JS Promise so callers can await it.
// fn receives (resolve, reject) JS functions and must call one of them.
func newPromise(fn func(resolve, reject js.Value)) js.Value {
	return js.Global().Get("Promise").New(js.FuncOf(func(_ js.Value, args []js.Value) any {
		resolve, reject := args[0], args[1]
		go fn(resolve, reject)
		return nil
	}))
}

func resolveBody(resp *http.Response, resolve, reject js.Value) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		reject.Invoke(fmt.Sprintf("read error: %s", err))
		return
	}
	// Return status + body as a JSON-like string for display
	result := fmt.Sprintf("HTTP %d\n\n%s", resp.StatusCode, string(body))
	resolve.Invoke(result)
}

// httpGet performs a GET request and returns a Promise<string>.
// args[0] = URL (string)
// args[1] = optional query params as "key=value&key2=value2" (string)
func httpGet(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return js.Global().Get("Promise").Call("reject", "httpGet: URL required")
	}
	url := args[0].String()
	if len(args) > 1 && args[1].String() != "" {
		url += "?" + args[1].String()
	}

	return newPromise(func(resolve, reject js.Value) {
		resp, err := http.Get(url)
		if err != nil {
			reject.Invoke(fmt.Sprintf("GET error: %s", err))
			return
		}
		resolveBody(resp, resolve, reject)
	})
}

// httpPost performs a POST request with a JSON body and returns a Promise<string>.
// args[0] = URL (string)
// args[1] = JSON body (string)
func httpPost(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return js.Global().Get("Promise").Call("reject", "httpPost: URL and body required")
	}
	url := args[0].String()
	body := args[1].String()

	return newPromise(func(resolve, reject js.Value) {
		resp, err := http.Post(url, "application/json", bytes.NewBufferString(body))
		if err != nil {
			reject.Invoke(fmt.Sprintf("POST error: %s", err))
			return
		}
		resolveBody(resp, resolve, reject)
	})
}

// notify requests notification permission if needed, then fires a notification.
// args[0] = title (string), args[1] = body (string)
func notify(_ js.Value, args []js.Value) any {
	title := "Go App"
	body := "Hello from Go!"
	if len(args) > 0 {
		title = args[0].String()
	}
	if len(args) > 1 {
		body = args[1].String()
	}

	return newPromise(func(resolve, reject js.Value) {
		notifClass := js.Global().Get("Notification")
		if notifClass.IsUndefined() {
			reject.Invoke("Notification API not supported")
			return
		}

		show := func() {
			opts := js.Global().Get("Object").New()
			opts.Set("body", body)
			notifClass.New(title, opts)
			resolve.Invoke("sent")
		}

		if notifClass.Get("permission").String() == "granted" {
			show()
			return
		}

		notifClass.Call("requestPermission").Call("then", js.FuncOf(func(_ js.Value, a []js.Value) any {
			if a[0].String() == "granted" {
				show()
			} else {
				reject.Invoke("permission denied")
			}
			return nil
		}))
	})
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	js.Global().Set("GoApp", js.ValueOf(map[string]any{
		"greet":     js.FuncOf(greet),
		"process":   js.FuncOf(process),
		"timestamp": js.FuncOf(timestamp),
		"add":       js.FuncOf(add),
		"httpGet":   js.FuncOf(httpGet),
		"httpPost":  js.FuncOf(httpPost),
		"notify":    js.FuncOf(notify),
	}))

	select {}
}
