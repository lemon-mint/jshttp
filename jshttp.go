//go:build js
// +build js

package jshttp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"runtime"
	"strings"
	"sync"
	"syscall/js"
)

var httpServer http.Handler = http.DefaultServeMux
var mu sync.Mutex

type jsHTTPResp struct {
	Body       bytes.Buffer
	HTTPHeader http.Header
	Status     int
}

func (j *jsHTTPResp) Header() http.Header {
	if j.HTTPHeader == nil {
		j.HTTPHeader = make(http.Header)
	}
	return j.HTTPHeader
}

func (j *jsHTTPResp) Write(b []byte) (int, error) {
	return j.Body.Write(b)
}

func (j *jsHTTPResp) WriteHeader(statusCode int) {
	j.Status = statusCode
}

func init() {
	js.Global().Set("__go_jshttp", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) < 1 {
			return js.Null()
		}
		_ = this
		JSRequest := args[0]
		JSMethod := JSRequest.Get("method")
		JSURL := JSRequest.Get("url")
		JSHeaders := js.Global().Get("Array").Call("from", JSRequest.Get("headers").Call("entries"))
		var r io.Reader
		if len(args) < 2 {
			JSBufferPromise := JSRequest.Call("arrayBuffer")
			bChan := make(chan []byte, 1)
			errChan := make(chan error, 1)
			success := js.FuncOf(func(_ js.Value, args []js.Value) any {
				JSBodyArray := js.Global().Get("Uint8Array").New(args[0])
				bodyBuffer := make([]byte, JSBodyArray.Get("byteLength").Int())
				js.CopyBytesToGo(bodyBuffer, JSBodyArray)
				bChan <- bodyBuffer
				return nil
			})
			defer success.Release()
			failure := js.FuncOf(func(_ js.Value, args []js.Value) any {
				errChan <- fmt.Errorf("JS Error %s", args[0].String())
				return nil
			})
			defer failure.Release()
			go JSBufferPromise.Call("then", success, failure)
			select {
			case b := <-bChan:
				r = bytes.NewReader(b)
			case err := <-errChan:
				fmt.Println(err)
			}
		} else {
			JSBodyArray := js.Global().Get("Uint8Array").New(args[1])
			bodyBuffer := make([]byte, JSBodyArray.Get("byteLength").Int())
			js.CopyBytesToGo(bodyBuffer, JSBodyArray)
			r = bytes.NewReader(bodyBuffer)
		}

		httpRequest, err := http.NewRequest(JSMethod.String(), JSURL.String(), r)
		if err != nil {
			return js.Null()
		}
		HeadersLen := JSHeaders.Length()
		var hsb strings.Builder
		for i := 0; i < HeadersLen; i++ {
			h := JSHeaders.Index(i)
			key := h.Index(0).String()
			var values string
			if h.Length() >= 2 {
				values = h.Index(1).String()
			} else {
				continue
			}
			hsb.WriteString(key)
			hsb.WriteString(": ")
			hsb.WriteString(values)
			hsb.WriteString("\r\n")
		}
		hsb.WriteString("\r\n")

		tpr := textproto.NewReader(bufio.NewReader(strings.NewReader(hsb.String())))
		mh, err := tpr.ReadMIMEHeader()
		if err != nil {
			return js.Null()
		}
		httpRequest.Header = http.Header(mh)

		var Resp = jsHTTPResp{
			Status:     200,
			HTTPHeader: make(http.Header),
		}
		httpServer.ServeHTTP(&Resp, httpRequest)
		RespBody := Resp.Body.Bytes()
		body := js.Global().Get("Uint8Array").New(len(RespBody))
		js.CopyBytesToJS(body, RespBody)
		JSRespOptions := js.Global().Get("Object").New()
		JSRespHeaders := js.Global().Get("Object").New()
		for key := range Resp.HTTPHeader {
			JSRespHeaders.Set(key, Resp.HTTPHeader.Get(key))
		}
		JSRespOptions.Set("status", Resp.Status)
		JSRespOptions.Set("headers", JSRespHeaders)

		JSResp := js.Global().Get("Response").New(body.Get("buffer"), JSRespOptions)
		return JSResp
	}))
}

func Serve(h http.Handler) {
	mu.Lock()
	if h != nil {
		httpServer = h
	} else {
		httpServer = http.DefaultServeMux
	}
	mu.Unlock()
	if runtime.Compiler == "tinygo" || runtime.GOARCH != "wasm" {
		return
	}
	// Wait
	ch := make(chan bool)
	<-ch
}
