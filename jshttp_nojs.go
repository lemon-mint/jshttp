//go:build !js
// +build !js

package jshttp

import "net/http"

func Serve(h http.Handler) {}
