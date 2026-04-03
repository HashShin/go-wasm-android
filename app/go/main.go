//go:build js && wasm

// main.go — your app logic.
// Add functions here and register them in Expose() so JavaScript can call them.
// See bridge.go for available helpers: Async, GET, POST, Args.

package main

import (
	"fmt"
	"strings"
	"time"
)

func main() {
	Expose(map[string]Fn{

		// Returns a greeting string.
		// JS: await GoApp.greet("Alice")
		"greet": func(args Args) any {
			user := args.String(0, "World")
			return fmt.Sprintf("Hello, %s! This is Go (WASM) speaking.", user)
		},

		// Converts text to uppercase.
		// JS: await GoApp.process("hello")
		"process": func(args Args) any {
			return strings.ToUpper(strings.TrimSpace(args.String(0, "")))
		},

		// Returns the current Unix timestamp.
		// JS: await GoApp.timestamp()
		"timestamp": func(args Args) any {
			return fmt.Sprintf("%d", time.Now().Unix())
		},

		// Adds two numbers.
		// JS: await GoApp.add(3, 4)
		"add": func(args Args) any {
			return fmt.Sprintf("%d", args.Int(0, 0)+args.Int(1, 0))
		},

		// HTTP GET — optional query string as second arg.
		// JS: await GoApp.httpGet("https://httpbun.com/get", "name=GoApp")
		"httpGet": func(args Args) any {
			url := args.String(0, "")
			if qs := args.String(1, ""); qs != "" {
				url += "?" + qs
			}
			return GET(url)
		},

		// HTTP POST with a JSON body.
		// JS: await GoApp.httpPost("https://httpbun.com/post", '{"key":"val"}')
		"httpPost": func(args Args) any {
			return POST(args.String(0, ""), args.String(1, ""))
		},

	})

	select {}
}
