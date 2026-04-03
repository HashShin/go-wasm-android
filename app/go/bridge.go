//go:build js && wasm

// bridge.go — framework helpers for exposing Go to JavaScript.
// You do not need to edit this file.
// Use the helpers below inside main.go to add functions to your app.

package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"syscall/js"
)

// ── Registering functions ─────────────────────────────────────────────────────

// Expose registers a map of Go functions on window.GoApp so JavaScript can call them.
//
// Usage in main():
//
//	Expose(map[string]Fn{
//	    "hello": func(args Args) any {
//	        return "Hi, " + args.String(0, "World")
//	    },
//	})
func Expose(funcs map[string]Fn) {
	obj := make(map[string]any, len(funcs))
	for name, fn := range funcs {
		fn := fn
		obj[name] = js.FuncOf(func(_ js.Value, args []js.Value) any {
			return fn(Args(args))
		})
	}
	js.Global().Set("GoApp", js.ValueOf(obj))
}

// Fn is the signature for every function you expose to JavaScript.
type Fn func(args Args) any

// ── Argument helpers ──────────────────────────────────────────────────────────

// Args is a slice of JavaScript values passed from the browser.
type Args []js.Value

// String returns args[i] as a string, or fallback if i is out of range.
func (a Args) String(i int, fallback string) string {
	if i < len(a) {
		return a[i].String()
	}
	return fallback
}

// Int returns args[i] as an int, or fallback if i is out of range.
func (a Args) Int(i int, fallback int) int {
	if i < len(a) {
		return a[i].Int()
	}
	return fallback
}

// ── Async / Promise ───────────────────────────────────────────────────────────

// Async runs fn in a goroutine and returns a JS Promise<string>.
// Resolve with a string result, or reject with an error message.
//
// Usage:
//
//	return Async(func() (string, error) {
//	    data, err := fetchSomething()
//	    return data, err
//	})
func Async(fn func() (string, error)) js.Value {
	return js.Global().Get("Promise").New(js.FuncOf(func(_ js.Value, pArgs []js.Value) any {
		resolve, reject := pArgs[0], pArgs[1]
		go func() {
			result, err := fn()
			if err != nil {
				reject.Invoke(err.Error())
			} else {
				resolve.Invoke(result)
			}
		}()
		return nil
	}))
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

// GET fetches url and returns a Promise<string> with "HTTP <status>\n\n<body>".
//
// Usage:
//
//	return GET("https://api.example.com/data")
func GET(url string) js.Value {
	return Async(func() (string, error) {
		resp, err := http.Get(url)
		if err != nil {
			return "", err
		}
		return readResponse(resp)
	})
}

// POST sends a JSON body to url and returns a Promise<string>.
//
// Usage:
//
//	return POST("https://api.example.com/submit", `{"key":"value"}`)
func POST(url, jsonBody string) js.Value {
	return Async(func() (string, error) {
		resp, err := http.Post(url, "application/json", strings.NewReader(jsonBody))
		if err != nil {
			return "", err
		}
		return readResponse(resp)
	})
}

func readResponse(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("HTTP %d\n\n%s", resp.StatusCode, string(body)), nil
}
