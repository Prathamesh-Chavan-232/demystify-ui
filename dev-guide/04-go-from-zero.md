# 04 · Go from zero

Go for a TypeScript developer, taught through the ~150 lines of Go already in this
repo.

← [03 Containers](03-containers.md) · next → [05 Finishing M0](05-finishing-m0.md)

---

## Do this

**Run Go commands from `services/`.** The module root is there, not at the repo root.

```bash
cd services

go build -o bin/ ./cmd/...   # compile all three binaries into bin/
go test ./...                # run every test
go vet ./...                 # catch likely mistakes; run before every commit
gofmt -l .                   # list unformatted files. formatting is NOT a preference
go run ./cmd/api             # compile and run in one step
```

`./...` means "this directory and everything under it". You will type it constantly.

---

## Five things that will trip you up

Ordered by how much grief they cause, not by how fundamental they are.

### 1. Errors are values. There is no `try`.

This is the biggest adjustment. Go has no exceptions for ordinary failure. Functions
that can fail return an error as their **last return value**, and you check it
immediately.

```go
resp, err := http.DefaultClient.Do(req)
if err != nil {
    return err          // or handle it
}
defer resp.Body.Close()
```

The TypeScript instinct is that this is noisy. The Go position is that the noise _is_
the point: every failure path is visible at the call site, and you cannot accidentally
let one propagate invisibly the way an uncaught `throw` does.

You will write `if err != nil` hundreds of times. That is normal and not a sign you
are doing it wrong.

### 2. Multiple return values

`a, b := f()` is native, not destructuring. Very often it is `value, error`.

`_` discards a value you must accept but do not want:

```go
_, _ = w.Write([]byte("ok"))     // deliberately ignoring both returns
```

Writing `_` is a _statement of intent_ — "I know this returns something; I am choosing
to ignore it." `go vet` flags some ignored errors, so being explicit matters.

### 3. Zero values — there is no `undefined`

Every type has a zero value, and a declared variable always has it. `""` for strings,
`0` for numbers, `nil` for pointers, slices, and maps.

```go
port := os.Getenv(portEnv)   // missing env var gives "", never undefined
if port == "" {
    port = fallbackPort
}
```

There is no distinction between "absent" and "empty string". If you need one, you use
a pointer (`*string`, which can be `nil`) or a second boolean return. Most of the time
you do not need one, and the absence of `null` vs `undefined` vs missing is a relief.

### 4. Capitalisation is the export keyword

There is no `export`. An identifier starting with a **capital letter** is visible
outside its package; lowercase is package-private.

```go
func Serve(…)        // exported — httpx.Serve
func parseAddr(…)    // private to package httpx
```

So renaming `Serve` to `serve` breaks every caller. Capitalisation is API surface, not
style.

### 5. `defer` runs at function exit

```go
defer resp.Body.Close()
```

Schedules a call for when the enclosing **function** returns — regardless of which
return path, including a panic. It is the cleanup mechanism, roughly `try/finally`
without the block.

The gotcha: it fires at _function_ exit, not block exit. A `defer` inside a loop
accumulates and only runs when the whole function ends.

---

## Syntax map

| TypeScript                 | Go                             | Note                              |
| -------------------------- | ------------------------------ | --------------------------------- |
| `const x = 5`              | `x := 5`                       | `:=` declares and infers          |
| `let x: string`            | `var x string`                 | zero value `""`                   |
| `interface Foo { … }`      | `type Foo interface { … }`     | methods only                      |
| `type Foo = { a: string }` | `type Foo struct { A string }` | data                              |
| `class`                    | struct + methods               | no inheritance                    |
| `x?.y`                     | —                              | no optional chaining; check `nil` |
| `x ?? y`                   | —                              | use zero-value checks             |
| `string[]`                 | `[]string`                     | a slice                           |
| `Record<string, number>`   | `map[string]int`               |                                   |
| `async`/`await`            | goroutines + channels          | different model entirely          |
| `throw`                    | `return err`                   | panics exist but are for bugs     |
| `null` / `undefined`       | `nil`                          | one concept, not two              |
| `export`                   | capital letter                 |                                   |
| `import { x } from 'y'`    | `import "y"` then `y.X`        | always namespaced                 |

**Structs vs interfaces.** A struct holds data. An interface lists method signatures.
A type satisfies an interface simply by having the right methods — no `implements`
keyword, no declaration. This is structural typing, which as a TypeScript developer
you already think in.

---

## Reading `services/internal/httpx/httpx.go`

The whole file, in order.

### Package and imports

```go
// Package httpx holds the HTTP server setup shared by the demystify services.
package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	…
)
```

`package httpx` — every file in a directory belongs to one package. The doc comment
starts with `Package <name>` by convention; `go doc` renders it.

All of these are standard library. Go's standard library is unusually complete — an
HTTP server, JSON, TLS, and structured logging are all built in, so a Go service
typically has far fewer dependencies than a Node one.

### The signature

```go
func Serve(name, portEnv, fallbackPort string, mux *http.ServeMux) error {
```

`name, portEnv, fallbackPort string` — three parameters sharing one type annotation.

`mux *http.ServeMux` — a **pointer** to a ServeMux. The `*` means "address of a value"
rather than a copy. Go passes everything by value, so without the pointer the function
would receive a copy and the caller would not see modifications. Rule of thumb: use a
pointer when the value is large or you intend to mutate it.

Returns `error` — a single value, `nil` on success.

### Optional argument, Go-style

```go
if mux == nil {
    mux = http.NewServeMux()
}
```

Go has no default parameters and no overloading. The idiom is to accept `nil` and
substitute. Every caller in `cmd/` passes `nil`; the parameter exists so a future
service can supply real routes.

### Registering a route

```go
mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte("ok"))
})
```

The `"GET /healthz"` pattern — method plus path in one string — is Go 1.22+. Older
tutorials show manual method checks inside the handler; you do not need that.

`w ResponseWriter` is the response, `_ *http.Request` the request, discarded because
this handler ignores it. `[]byte("ok")` converts a string to a byte slice, which is
what `Write` takes.

Note there is no `return` of a response. You _write into_ `w`. That is the whole
handler model.

### Struct literal with field names

```go
srv := &http.Server{
    Addr:              ":" + port,
    Handler:           mux,
    ReadHeaderTimeout: 5 * time.Second,
}
```

`&` takes the address, producing a pointer. Fields not listed get their zero values.

`5 * time.Second` is a typed duration, not a number — the compiler rejects a bare `5`.
This kills the "was that milliseconds or seconds?" bug at compile time.

`ReadHeaderTimeout` is set because without it a client can open a connection, dribble
headers indefinitely, and hold a slot forever — a trivial denial of service. `go vet`
and most linters flag its absence.

### Graceful shutdown

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

go func() {
    <-ctx.Done()
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    _ = srv.Shutdown(shutdownCtx)
}()
```

This is the densest passage in the file.

A **context** carries cancellation across API boundaries. `signal.NotifyContext`
returns one that is cancelled when the process receives SIGINT (Ctrl-C) or SIGTERM
(what `docker stop` sends).

`go func() { … }()` starts a **goroutine** — a concurrent function, far cheaper than an
OS thread. This is Go's answer to `async`, but note the difference: there is no
`await` and no promise. It just runs alongside.

`<-ctx.Done()` **receives from a channel** and blocks until something arrives.
`Done()` returns a channel that closes on cancellation, so this line means "wait here
until we are told to shut down".

So: the goroutine parks until a shutdown signal arrives, then gives in-flight requests
10 seconds to finish before forcing the server closed. Without it, `docker stop` kills
connections mid-response.

### The error check that is not an error

```go
if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
    return err
}
return nil
```

`ListenAndServe` blocks until the server stops, then _always_ returns a non-nil error.
A clean shutdown returns `http.ErrServerClosed` — which is expected, not a failure.

`errors.Is` compares through wrapping (Go errors can wrap other errors, like a
`cause` chain). Comparing with `==` would break the moment something wraps the error.

`if err := …; err != nil` declares `err` scoped to the `if`. Very common idiom.

---

## Reading `services/cmd/api/main.go`

```go
package main

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-health" {
		os.Exit(httpx.HealthCheck("API_PORT", "8080"))
	}
	if err := httpx.Serve("api", "API_PORT", "8080", nil); err != nil {
		slog.Error("server failed", "service", "api", "err", err)
		os.Exit(1)
	}
}
```

`package main` with a `func main()` is what makes a directory an executable rather
than a library. Each of `cmd/api`, `cmd/palette`, `cmd/audit` is its own `package
main` producing its own binary.

The `-health` branch is the distroless healthcheck from [ch 03](03-containers.md).
`os.Args[0]` is the program name, so real arguments start at index 1.

`slog.Error("msg", "key", value, "key", value)` is structured logging — alternating
keys and values, emitted as machine-readable output.

`os.Exit(1)` terminates immediately. **It skips `defer`red calls**, so never use it
where cleanup matters.

---

## Modules, packages, and import paths

```
services/go.mod   →  module github.com/Prathamesh-Chavan-232/demystify-ui/services
```

- **Module** — the unit of versioning and dependency. One `go.mod`. Ours is
  `services/`.
- **Package** — one directory of `.go` files. `internal/httpx` is a package.
- **Import path** — module path + directory:
  `github.com/…/demystify-ui/services/internal/httpx`.

**The module path should match the clone URL.** Go fetches dependencies by URL, so a
mismatch breaks `go get` for anyone else. This is why the path says `demystify-ui`
(the actual repo name) rather than `demystify`.

**`internal/` is enforced by the compiler.** Anything under a directory named
`internal` can only be imported by code in the parent tree. `services/internal/httpx`
is importable by `services/cmd/api`, and by nothing outside `services/`. It is a real
access boundary, not a naming convention.

---

## Exercise

Add a `/version` endpoint to `httpx.Serve`, and cover it with a table-driven test.

**Part 1** — register a route returning a hardcoded version string. Verify:

```bash
cd services && go build -o bin/ ./cmd/... && ./bin/api &
curl localhost:8080/version
```

**Part 2** — write `services/internal/httpx/httpx_test.go` using a table-driven test,
the idiomatic Go pattern. Use `httptest.NewRecorder()` rather than starting a real
server.

<details>
<summary>Answer</summary>

In `Serve`, after the `/healthz` registration:

```go
mux.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) {
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte(Version))
})
```

and at package level:

```go
// Version is the build version, overridden at link time with
//   -ldflags="-X …/internal/httpx.Version=$(git rev-parse --short HEAD)"
var Version = "dev"
```

The test:

```go
package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoutes(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantCode int
		wantBody string
	}{
		{name: "healthz", path: "/healthz", wantCode: http.StatusOK, wantBody: "ok"},
		{name: "version", path: "/version", wantCode: http.StatusOK, wantBody: "dev"},
		{name: "unknown", path: "/nope", wantCode: http.StatusNotFound},
	}

	mux := http.NewServeMux()
	registerRoutes(mux)      // extract route setup out of Serve first

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if rec.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantCode)
			}
			if tt.wantBody != "" && rec.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}
```

**What to notice.**

_The table._ A slice of anonymous structs, one per case. Adding a case is one line, and
every case runs through identical logic — so they cannot drift apart the way copied
test functions do. This is the default shape of a Go test.

_`t.Run`_ creates a subtest, so failures report as `TestRoutes/version` and you can run
one with `-run TestRoutes/version`.

_`t.Errorf` vs `t.Fatalf`_ — `Errorf` records the failure and continues; `Fatalf` stops
the subtest. Use `Errorf` when later assertions still yield information.

_Refactor first._ Testing meant extracting route registration out of `Serve`, because
`Serve` blocks forever. Code that is hard to test is usually telling you something —
here, that setup and running were tangled.

_Same package._ The file is `package httpx`, not `httpx_test`, so it can reach
unexported identifiers. Use `package httpx_test` when you want to test only the public
API.
</details>

---

## Where to go deeper

- [A Tour of Go](https://go.dev/tour/) — the official interactive introduction. An
  afternoon, and it covers everything above properly.
- [Effective Go](https://go.dev/doc/effective_go) — idiom rather than syntax. Read it
  once you have written a few hundred lines.
- `go doc net/http.Server` — offline documentation for anything in scope.

**Your Go workload is small for now.** ~150 lines today; the substantial work
(k-means palette extraction, ΔE bulk audit) does not arrive until M5. You do not need
to be fluent yet — you need to read `httpx.go` without flinching.

---

← [03 Containers](03-containers.md) · next → [05 Finishing M0](05-finishing-m0.md)
