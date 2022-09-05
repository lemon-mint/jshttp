import { serve } from "https://deno.land/std@0.154.0/http/mod.ts";
import * as _ from "https://cdn.jsdelivr.net/gh/golang/go@go1.19/misc/wasm/wasm_exec.js";

const wasm_file = await Deno.readFile("./main.wasm");
const go = new window.Go();
const instance = await WebAssembly.instantiate(wasm_file, go.importObject);

go.argv = Deno.args.slice(2);
if ((await Deno.permissions.query({ name: "env" })).state == "granted") {
    const env = Deno.env.toObject();
    go.env = env;
}
go.exit = Deno.exit;
go.run(instance.instance);

await serve(async (_req) => {
    try {
        const _resp = await window.__go_jshttp(_req);
        return _resp;
    } catch (e) {
        console.error(e);
        return new Response("Internal Server Error", { status: 500 });
    }
});
