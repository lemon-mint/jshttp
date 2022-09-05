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
	mu         sync.Mutex
	Body       bytes.Buffer
	HTTPHeader http.Header
	Status     int
}

func (j *jsHTTPResp) Header() http.Header {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.HTTPHeader == nil {
		j.HTTPHeader = make(http.Header)
	}
	return j.HTTPHeader
}

func (j *jsHTTPResp) Write(b []byte) (int, error) {
	j.mu.Lock()
	n, err := j.Body.Write(b)
	j.mu.Unlock()

	return n, err
}

func (j *jsHTTPResp) WriteHeader(statusCode int) {
	j.mu.Lock()
	j.Status = statusCode
	j.mu.Unlock()
}

var js_Array = js.Global().Get("Array")
var js_Response = js.Global().Get("Response")
var js_Object = js.Global().Get("Object")
var js_Uint8Array = js.Global().Get("Uint8Array")
var js_Promise = js.Global().Get("Promise")
var js_Error = js.Global().Get("Error")

func init() {
	js.Global().Set("__go_jshttp", js.FuncOf(func(_ js.Value, args []js.Value) any {
		var RespPromise = js_Promise.New(
			js.FuncOf(
				func(_ js.Value, argsPromise []js.Value) any {
					go func() {
						resolve := argsPromise[0]
						reject := argsPromise[1]

						if len(args) < 1 {
							reject.Invoke(
								js_Error.New("required parameter JSRequest missing"),
							)
							return
						}
						JSRequest := args[0]
						JSMethod := JSRequest.Get("method")
						JSURL := JSRequest.Get("url")
						JSHeaders := js_Array.Call("from", JSRequest.Get("headers").Call("entries"))
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
							JSBodyArray := js_Uint8Array.New(args[1])
							bodyBuffer := make([]byte, JSBodyArray.Get("byteLength").Int())
							js.CopyBytesToGo(bodyBuffer, JSBodyArray)
							r = bytes.NewReader(bodyBuffer)
						}

						httpRequest, err := http.NewRequest(JSMethod.String(), JSURL.String(), r)
						if err != nil {
							reject.Invoke(
								js_Error.New(err.Error()),
							)
							return
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
							reject.Invoke(
								js_Error.New(err.Error()),
							)
							return
						}
						httpRequest.Header = http.Header(mh)

						var Resp = jsHTTPResp{
							Status:     200,
							HTTPHeader: make(http.Header),
						}
						var wg sync.WaitGroup
						wg.Add(1)
						go func() {
							defer wg.Done()
							defer func() {
								v := recover()
								if v != nil {
									Resp = jsHTTPResp{
										Status:     200,
										HTTPHeader: make(http.Header),
									}
									http.Error(&Resp, "Internal Server Error", http.StatusInternalServerError)
								}
							}()
							httpServer.ServeHTTP(&Resp, httpRequest)
						}()

						wg.Wait()
						RespBody := Resp.Body.Bytes()
						body := js_Uint8Array.New(len(RespBody))
						js.CopyBytesToJS(body, RespBody)
						JSRespOptions := js_Object.New()
						JSRespHeaders := js_Object.New()
						for key := range Resp.HTTPHeader {
							JSRespHeaders.Set(key, Resp.HTTPHeader.Get(key))
						}
						JSRespOptions.Set("status", Resp.Status)
						JSRespOptions.Set("headers", JSRespHeaders)

						JSResp := js_Response.New(body.Get("buffer"), JSRespOptions)
						resolve.Invoke(JSResp)
						return
					}()

					return js.Null()
				}))
		return RespPromise
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
