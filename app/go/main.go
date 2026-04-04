//go:build js && wasm

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"syscall/js"
	"time"
)

// ── globals ───────────────────────────────────────────────────────────────────

// win is the JS global object (window). Cached to avoid repeated calls.
var win = js.Global()

// H is the standard JS callback signature.
type H = func(js.Value, []js.Value) any

// ── media state ───────────────────────────────────────────────────────────────

var cam struct {
	stream js.Value
}

var mic struct {
	stream   js.Value
	ctx      js.Value
	raf      js.Value
	interval js.Value
	tickFn   js.Func
}

// ── DOM helpers ───────────────────────────────────────────────────────────────

func el(id string) js.Value   { return win.Get("document").Call("getElementById", id) }
func setText(id, v string)    { el(id).Set("textContent", v) }
func getVal(id string) string { return el(id).Get("value").String() }

// on registers a click listener on the element with the given id.
func on(id string, fn H) {
	el(id).Call("addEventListener", "click", js.FuncOf(fn))
}

// setStatus updates a status badge element's text and ok/err class.
func setStatus(id, text string, ok bool) {
	cls := "status-badge status-err"
	if ok {
		cls = "status-badge status-ok"
	}
	e := el(id)
	e.Set("textContent", text)
	e.Set("className", cls)
}

// ── JS helpers ────────────────────────────────────────────────────────────────

// jsObj creates a plain JS object from alternating key/value pairs.
func jsObj(pairs ...any) js.Value {
	o := win.Get("Object").New()
	for i := 0; i+1 < len(pairs); i += 2 {
		o.Set(pairs[i].(string), pairs[i+1])
	}
	return o
}

// android returns the named Android bridge and whether it is available.
func android(name string) (js.Value, bool) {
	v := win.Get(name)
	return v, !v.IsUndefined()
}

// stopTracks stops every track on a media stream.
func stopTracks(stream js.Value) {
	tracks := stream.Call("getTracks")
	for i := 0; i < tracks.Get("length").Int(); i++ {
		tracks.Index(i).Call("stop")
	}
}

// ── string helpers ────────────────────────────────────────────────────────────

// or returns s if non-empty, otherwise fallback.
func or(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// prettyJSON formats a "STATUS\n\nBODY" string with indented JSON body.
func prettyJSON(raw string) string {
	idx := strings.Index(raw, "\n\n")
	if idx == -1 {
		return raw
	}
	header, body := raw[:idx], raw[idx+2:]
	var v any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return raw
	}
	out, _ := json.MarshalIndent(v, "", "  ")
	return header + "\n\n" + string(out)
}

// ── async helpers ─────────────────────────────────────────────────────────────

// newPromise wraps fn in a JS Promise, running fn on a new goroutine.
func newPromise(fn func(resolve, reject js.Value)) js.Value {
	return win.Get("Promise").New(js.FuncOf(func(_ js.Value, args []js.Value) any {
		go fn(args[0], args[1])
		return nil
	}))
}

func resolveBody(resp *http.Response, resolve, reject js.Value) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		reject.Invoke(err.Error())
		return
	}
	resolve.Invoke(fmt.Sprintf("HTTP %d\n\n%s", resp.StatusCode, string(body)))
}

// httpRequest fires an async HTTP call and writes the result to the UI.
func httpRequest(resultID, statusID string, do func() (*http.Response, error)) {
	resultEl := el(resultID)
	resultEl.Set("textContent", "Loading…")
	setStatus(statusID, "…", true)

	go func() {
		resp, err := do()
		if err != nil {
			setStatus(statusID, "Error", false)
			resultEl.Set("textContent", err.Error())
			return
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		raw := fmt.Sprintf("HTTP %d\n\n%s", resp.StatusCode, string(b))
		setStatus(statusID, fmt.Sprintf("HTTP %d", resp.StatusCode), resp.StatusCode < 400)
		resultEl.Set("textContent", prettyJSON(raw))
	}()
}

// ── basic handlers ────────────────────────────────────────────────────────────

func doGreet(_ js.Value, _ []js.Value) any {
	name := or(getVal("nameInput"), "World")
	setText("greetResult", fmt.Sprintf("Hello, %s! This is Go (WASM) speaking.", name))
	return nil
}

func doProcess(_ js.Value, _ []js.Value) any {
	setText("processResult", strings.ToUpper(strings.TrimSpace(getVal("textInput"))))
	return nil
}

func doAdd(_ js.Value, _ []js.Value) any {
	a, b := 0, 0
	fmt.Sscanf(getVal("numA"), "%d", &a)
	fmt.Sscanf(getVal("numB"), "%d", &b)
	setText("addResult", fmt.Sprintf("Result: %d", a+b))
	return nil
}

func doTimestamp(_ js.Value, _ []js.Value) any {
	setText("tsResult", fmt.Sprintf("Unix timestamp: %d", time.Now().Unix()))
	return nil
}

// ── HTTP handlers ─────────────────────────────────────────────────────────────

func doGet(_ js.Value, _ []js.Value) any {
	key := or(getVal("getKey"), "param")
	val := or(getVal("getVal"), "value")
	httpRequest("getResult", "getStatus", func() (*http.Response, error) {
		return http.Get("https://httpbun.com/get?" + key + "=" + url.QueryEscape(val))
	})
	return nil
}

func doPost(_ js.Value, _ []js.Value) any {
	body := getVal("postBody")
	httpRequest("postResult", "postStatus", func() (*http.Response, error) {
		return http.Post("https://httpbun.com/post", "application/json", strings.NewReader(body))
	})
	return nil
}

// ── storage ───────────────────────────────────────────────────────────────────

func doWriteFile(_ js.Value, _ []js.Value) any {
	content := "Test write at " + time.Now().Format(time.RFC3339)
	if s, ok := android("AndroidStorage"); ok {
		setText("storageResult", s.Call("writeFile", "test.txt", content).String())
	} else {
		win.Get("localStorage").Call("setItem", "test_file", content)
		setText("storageResult", "localStorage: "+content)
	}
	return nil
}

func doReadFile(_ js.Value, _ []js.Value) any {
	if s, ok := android("AndroidStorage"); ok {
		setText("storageResult", s.Call("readFile", "test.txt").String())
	} else if v := win.Get("localStorage").Call("getItem", "test_file"); !v.IsNull() {
		setText("storageResult", "localStorage: "+v.String())
	} else {
		setText("storageResult", "No file found — write first")
	}
	return nil
}

// ── camera ────────────────────────────────────────────────────────────────────

func doCamera(_ js.Value, _ []js.Value) any {
	win.Get("navigator").Get("mediaDevices").
		Call("getUserMedia", jsObj("video", true)).
		Call("then", js.FuncOf(func(_ js.Value, args []js.Value) any {
			cam.stream = args[0]
			video := el("cameraStream")
			video.Set("srcObject", cam.stream)
			video.Get("style").Set("display", "block")
			setText("cameraResult", "Camera active")
			return nil
		})).
		Call("catch", js.FuncOf(func(_ js.Value, args []js.Value) any {
			switch args[0].Get("name").String() {
			case "NotAllowedError":
				setText("cameraResult", "Permission denied — allow camera in your browser or app settings")
			case "NotFoundError":
				setText("cameraResult", "No camera found on this device")
			default:
				setText("cameraResult", "Error: "+args[0].Get("message").String())
			}
			return nil
		}))
	return nil
}

func doStopCamera(_ js.Value, _ []js.Value) any {
	if !cam.stream.IsUndefined() {
		stopTracks(cam.stream)
		cam.stream = js.Undefined()
	}
	video := el("cameraStream")
	video.Set("srcObject", js.Null())
	video.Get("style").Set("display", "none")
	setText("cameraResult", "Camera stopped")
	return nil
}

// ── microphone ────────────────────────────────────────────────────────────────

func doMic(_ js.Value, _ []js.Value) any {
	if bridge, ok := android("AndroidMicrophone"); ok {
		bridge.Call("start")
		setText("micResult", "Microphone active")
		cb := js.FuncOf(func(_ js.Value, _ []js.Value) any {
			amp := min(bridge.Call("getAmplitude").Float()*3, 1)
			el("volumeFill").Get("style").Set("width", fmt.Sprintf("%.0f%%", amp*100))
			return nil
		})
		mic.interval = win.Call("setInterval", cb, 50)
		return nil
	}

	win.Get("navigator").Get("mediaDevices").
		Call("getUserMedia", jsObj("audio", true)).
		Call("then", js.FuncOf(func(_ js.Value, args []js.Value) any {
			mic.stream = args[0]
			mic.ctx = win.Get("AudioContext").New()
			analyser := mic.ctx.Call("createAnalyser")
			analyser.Set("fftSize", 256)
			mic.ctx.Call("createMediaStreamSource", mic.stream).Call("connect", analyser)

			freqCount := analyser.Get("frequencyBinCount").Int()
			dataArr := win.Get("Uint8Array").New(freqCount)
			setText("micResult", "Microphone active")

			mic.tickFn = js.FuncOf(func(_ js.Value, _ []js.Value) any {
				analyser.Call("getByteFrequencyData", dataArr)
				peak := 0
				for i := 0; i < freqCount; i++ {
					if v := dataArr.Index(i).Int(); v > peak {
						peak = v
					}
				}
				el("volumeFill").Get("style").Set("width", fmt.Sprintf("%.0f%%", float64(peak)/255*100))
				mic.raf = win.Call("requestAnimationFrame", mic.tickFn)
				return nil
			})
			mic.raf = win.Call("requestAnimationFrame", mic.tickFn)
			return nil
		})).
		Call("catch", js.FuncOf(func(_ js.Value, args []js.Value) any {
			switch args[0].Get("name").String() {
			case "NotAllowedError":
				setText("micResult", "Permission denied — allow microphone in your browser or app settings")
			case "NotReadableError":
				setText("micResult", "Microphone in use by another app")
			default:
				setText("micResult", "Error: "+args[0].Get("message").String())
			}
			return nil
		}))
	return nil
}

func doStopMic(_ js.Value, _ []js.Value) any {
	if bridge, ok := android("AndroidMicrophone"); ok {
		bridge.Call("stop")
	}
	if !mic.interval.IsUndefined() {
		win.Call("clearInterval", mic.interval)
		mic.interval = js.Undefined()
	}
	if !mic.raf.IsUndefined() {
		win.Call("cancelAnimationFrame", mic.raf)
		mic.raf = js.Undefined()
		mic.tickFn.Release()
	}
	if !mic.stream.IsUndefined() {
		stopTracks(mic.stream)
		mic.stream = js.Undefined()
	}
	if !mic.ctx.IsUndefined() {
		mic.ctx.Call("close")
		mic.ctx = js.Undefined()
	}
	el("volumeFill").Get("style").Set("width", "0%")
	setText("micResult", "Microphone stopped")
	return nil
}

// ── location ──────────────────────────────────────────────────────────────────

func doLocation(_ js.Value, _ []js.Value) any {
	geo := win.Get("navigator").Get("geolocation")
	if geo.IsUndefined() {
		setText("locationResult", "Geolocation not available — requires HTTPS or localhost")
		return nil
	}
	setText("locationResult", "Getting location…")

	geo.Call("getCurrentPosition",
		js.FuncOf(func(_ js.Value, args []js.Value) any {
			c := args[0].Get("coords")
			setText("locationResult", fmt.Sprintf(
				"Lat: %.6f\nLng: %.6f\nAccuracy: %.0fm",
				c.Get("latitude").Float(),
				c.Get("longitude").Float(),
				c.Get("accuracy").Float(),
			))
			return nil
		}),
		js.FuncOf(func(_ js.Value, args []js.Value) any {
			msg := map[int]string{
				1: "Permission denied — allow location in your browser or app settings",
				2: "Location unavailable — check GPS or network connection",
				3: "Timed out — move to an open area and try again",
			}
			code := args[0].Get("code").Int()
			setText("locationResult", or(msg[code], "Error: "+args[0].Get("message").String()))
			return nil
		}),
		jsObj("enableHighAccuracy", true, "timeout", 10000),
	)
	return nil
}

// ── download ──────────────────────────────────────────────────────────────────

func doOpenFolderModal(_ js.Value, _ []js.Value) any {
	if dl, ok := android("AndroidDownload"); ok {
		dl.Call("pickFolder")
	} else {
		setText("dlResult", "Folder picker is Android only")
	}
	return nil
}

func doDownload(_ js.Value, _ []js.Value) any {
	rawURL := strings.TrimSpace(getVal("dlUrl"))
	filename := or(strings.TrimSpace(getVal("dlFilename")), "download")

	if rawURL == "" {
		setText("dlResult", "Enter a URL first")
		return nil
	}
	if dl, ok := android("AndroidDownload"); ok {
		setText("dlResult", dl.Call("download", rawURL, filename).String())
		return nil
	}

	setText("dlResult", "Downloading…")
	go func() {
		resp, err := http.Get(rawURL)
		if err != nil {
			setText("dlResult", "Error: "+err.Error())
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			setText("dlResult", fmt.Sprintf("Error: HTTP %d", resp.StatusCode))
			return
		}
		data, _ := io.ReadAll(resp.Body)

		jsArr := win.Get("Uint8Array").New(len(data))
		js.CopyBytesToJS(jsArr, data)
		blob := win.Get("Blob").New(win.Get("Array").New(jsArr))
		objURL := win.Get("URL").Call("createObjectURL", blob)

		a := win.Get("document").Call("createElement", "a")
		a.Set("href", objURL)
		a.Set("download", filename)
		a.Call("click")
		win.Get("URL").Call("revokeObjectURL", objURL)
		setText("dlResult", fmt.Sprintf("Saved: %s (%.1f KB)", filename, float64(len(data))/1024))
	}()
	return nil
}

// ── notifications ─────────────────────────────────────────────────────────────

func doNotify(_ js.Value, _ []js.Value) any {
	title := or(getVal("notifTitle"), "Go App")
	body := or(getVal("notifBody"), "Test notification")

	if n, ok := android("AndroidNotification"); ok {
		n.Call("show", title, body)
		setText("notifResult", "Notification sent!")
		return nil
	}

	notifClass := win.Get("Notification")
	if notifClass.IsUndefined() {
		setText("notifResult", "Notifications not supported — requires HTTPS or localhost")
		return nil
	}

	send := func() {
		opts := jsObj("body", body)
		sw := win.Get("navigator").Get("serviceWorker")
		if !sw.IsUndefined() {
			sw.Get("ready").
				Call("then", js.FuncOf(func(_ js.Value, args []js.Value) any {
					args[0].Call("showNotification", title, opts)
					setText("notifResult", "Notification sent!")
					return nil
				})).
				Call("catch", js.FuncOf(func(_ js.Value, args []js.Value) any {
					setText("notifResult", "Error: "+args[0].Call("toString").String())
					return nil
				}))
		} else {
			notifClass.New(title, opts)
			setText("notifResult", "Notification sent!")
		}
	}

	switch notifClass.Get("permission").String() {
	case "granted":
		send()
	case "default":
		notifClass.Call("requestPermission").
			Call("then", js.FuncOf(func(_ js.Value, args []js.Value) any {
				if args[0].String() == "granted" {
					send()
				} else {
					setText("notifResult", "Permission denied — allow notifications in your browser settings")
				}
				return nil
			}))
	default:
		setText("notifResult", "Permission denied — allow notifications in your browser settings")
	}
	return nil
}

// ── GoApp exports ─────────────────────────────────────────────────────────────

func goHttpGet(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return win.Get("Promise").Call("reject", "httpGet: URL required")
	}
	rawURL := args[0].String()
	if len(args) > 1 && args[1].String() != "" {
		rawURL += "?" + args[1].String()
	}
	return newPromise(func(resolve, reject js.Value) {
		resp, err := http.Get(rawURL)
		if err != nil {
			reject.Invoke(err.Error())
			return
		}
		resolveBody(resp, resolve, reject)
	})
}

func goHttpPost(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return win.Get("Promise").Call("reject", "httpPost: URL and body required")
	}
	rawURL, body := args[0].String(), args[1].String()
	return newPromise(func(resolve, reject js.Value) {
		resp, err := http.Post(rawURL, "application/json", bytes.NewBufferString(body))
		if err != nil {
			reject.Invoke(err.Error())
			return
		}
		resolveBody(resp, resolve, reject)
	})
}

// ── setup & main ──────────────────────────────────────────────────────────────

func setupUI() {
	// Android bridge callbacks (invoked by native via evaluateJavascript)
	win.Set("onFolderPicked", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 {
			setText("dlDirDisplay", args[0].String())
		}
		return nil
	}))
	win.Set("onDownloadResult", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 {
			setText("dlResult", args[0].String())
		}
		return nil
	}))

	if dl, ok := android("AndroidDownload"); ok {
		setText("dlDirDisplay", dl.Call("getDownloadDir").String())
	}

	el("modeTag").Set("textContent", "Android APK (Go WASM)")
	el("app").Get("style").Set("display", "block")

	on("greetBtn",      doGreet)
	on("processBtn",    doProcess)
	on("addBtn",        doAdd)
	on("timestampBtn",  doTimestamp)
	on("writeFileBtn",  doWriteFile)
	on("readFileBtn",   doReadFile)
	on("cameraBtn",     doCamera)
	on("stopCameraBtn", doStopCamera)
	on("micBtn",        doMic)
	on("stopMicBtn",    doStopMic)
	on("locationBtn",   doLocation)
	on("folderBtn",     doOpenFolderModal)
	on("downloadBtn",   doDownload)
	on("notifyBtn",     doNotify)
	on("getBtn",        doGet)
	on("postBtn",       doPost)
}

func main() {
	win.Set("GoApp", js.ValueOf(map[string]any{
		"httpGet":  js.FuncOf(goHttpGet),
		"httpPost": js.FuncOf(goHttpPost),
	}))

	setupUI()
	select {}
}
