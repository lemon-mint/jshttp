[![Go Reference](https://img.shields.io/badge/go-reference-%23007d9c?style=for-the-badge&logo=go)](https://pkg.go.dev/v8.run/go/jshttp)

# JSHTTP
Go HTTP Adapter for WebAssembly

# Download and install
```bash
go get -u v8.run/go/jshttp
```

# Usage
```go
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
```

# Using JSHTTP With Deno

```js
import { serve } from "https://deno.land/std@0.153.0/http/mod.ts";
import * as _ from "https://raw.githubusercontent.com/golang/go/go1.19/misc/wasm/wasm_exec.js";

const wasm_file = await Deno.readFile("./main.wasm");
const go = new window.Go();
const instance = await WebAssembly.instantiate(wasm_file, go.importObject);

go.argv = Deno.args.slice(2);
if ((await Deno.permissions.query({ name: "env" })).state == "granted") {
    const env = Deno.env.toObject();
    go.env = env;
}
go.run(instance.instance);

await serve(async (_req) => {
    const _resp = window.__go_jshttp(_req, await _req.arrayBuffer());
    return _resp;
});
```
