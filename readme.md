

to build:

```bash
tinygo build -o main.wasm -gc=custom -tags="custommalloc nottinygc_envoy"  -target=wasi -scheduler=none main.go
```

Note: As of writing the latest version of go (v1.21) supports a preview version of WASI , which is needed to build WASM binaries which can run out of the browser. This presents a problem as the binaries built by this do not seem to work with envoy yet.

The alt. compiler we used for building WASI, tinygo doesn't have an implementation of "net" library, yet. This creates as a problem as the kafka library we use, uses the "net" library, and thus we are not able to build with tinygo. Tinygo does offer an alt. implementation of "net" library as well, based on unisockets, and we tried swapping kafka library's "net" with this implementation but the alt. implementation does not implement some methods and interfaces yet.

So till official go compiler comes with better support for "WASI" or the alt. "net" library by tinygo has a more complete implementation of the official go "net" library, we are pretty much limited by the technology of our time and we don't have that urgent a use case to build our own kafka implementation which can be compiled on tinygo.

Refs:

* https://github.com/henders/writing-an-envoy-wasm-plugin/tree/part1
* https://github.com/tetratelabs/proxy-wasm-go-sdk
* https://github.com/tetratelabs/proxy-wasm-go-sdk/blob/main/doc/OVERVIEW.md#tinygo-vs-the-official-go-compiler
* https://go.dev/blog/wasi
* https://github.com/tinygo-org/net
* https://github.com/tinygo-org/tinygo/issues/2704
