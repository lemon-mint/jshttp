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
