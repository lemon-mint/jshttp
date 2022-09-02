package main

import (
	"net/http"

	"v8.run/go/jshttp"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("Hello, World!"))
	})
	jshttp.Serve(mux)
}
