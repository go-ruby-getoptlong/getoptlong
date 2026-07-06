<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-getoptlong/brand/main/social/go-ruby-getoptlong-getoptlong.png" alt="go-ruby-getoptlong/getoptlong" width="720"></p>

# getoptlong — go-ruby-getoptlong

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-getoptlong.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of Ruby's
[GetoptLong](https://docs.ruby-lang.org/en/master/GetoptLong.html)** — the
getopt-style command-line option parser bundled with MRI (ruby/getoptlong 0.2.1,
the version shipped with Ruby 4.0.5). It scans an argument list for options
exactly as `GetoptLong` does — the same long/short/abbreviation matching, the
same `=`-joined and separate arguments, the same bundled short flags, the same
`--` terminator, the same three ordering modes, and the same error taxonomy with
**MRI-identical messages** — **without any Ruby runtime**.

It is the `GetoptLong` backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but is a
**standalone, reusable** module with no dependency on the Ruby runtime — a sibling
of [go-ruby-yaml](https://github.com/go-ruby-yaml/yaml) (the Psych port) and
[go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) (the Onigmo engine).

> **What it is — and isn't.** Argv scanning is pure compute and fully
> deterministic, so it lives here as pure Go. Unlike Ruby's `GetoptLong`, which
> mutates the global `ARGV` and reads `$0` / `POSIXLY_CORRECT` / `$stderr`, this
> package operates on an explicit, Parser-owned argument slice and writes errors
> to a caller-supplied `io.Writer` — so it is reusable and free of global state.
> The host (for example `rbgo`) binds `ARGV` and `$0` to a `Parser` and reads the
> remaining, non-option arguments back from `Parser.Args` after processing.

## Features

Faithful port of `GetoptLong#get` / `#each`, validated against the `ruby` binary
on every supported platform:

- **Long, short, and abbreviated** option names. A long name may be given by any
  unique prefix; an ambiguous prefix raises `AmbiguousOption`.
- **Aliases** — an option may have any number of names; the parsed option always
  reports the canonical (first) name, not the alias used.
- **Argument forms** — `--name=value` and `--name value` for long options;
  `-nvalue`, `-n value`, and bundled `-abc` for short options.
- **Three argument flags** — `NoArgument`, `RequiredArgument`, `OptionalArgument`
  — with MRI's exact rules for when the next word is consumed.
- **Three ordering modes** — `Permute` (the default; options and operands mix
  freely), `RequireOrder` (options must precede operands), and `ReturnInOrder`
  (operands are returned as `("", word)` options).
- **The `--` terminator** ends option processing; subsequent words are operands.
- **Full error taxonomy** — `InvalidOption`, `MissingArgument`,
  `NeedlessArgument`, `AmbiguousOption` (all under `Error`) — with the exact
  POSIX-format messages MRI produces, optional `$0`-style program-name prefix,
  and a quiet mode.

CGO-free, dependency-free, **100% test coverage**, `gofmt` + `go vet` clean, and
green across the six 64-bit Go targets (amd64, arm64, riscv64, loong64, ppc64le,
s390x) and three operating systems (Linux, macOS, Windows).

## Install

```sh
go get github.com/go-ruby-getoptlong/getoptlong
```

## Usage

```go
package main

import (
	"fmt"
	"os"

	"github.com/go-ruby-getoptlong/getoptlong"
)

func main() {
	p, err := getoptlong.New(os.Args[1:],
		getoptlong.Option{Names: []string{"--number", "-n"}, Flag: getoptlong.RequiredArgument},
		getoptlong.Option{Names: []string{"--verbose", "-v"}, Flag: getoptlong.OptionalArgument},
		getoptlong.Option{Names: []string{"--help", "-h"}, Flag: getoptlong.NoArgument},
	)
	if err != nil {
		panic(err) // a *SpecError: a malformed option specification
	}
	p.ProgName = "fib"        // mirrors Ruby's $0 in error messages
	p.ErrorWriter = os.Stderr // mirrors Ruby's $stderr

	err = p.Each(func(name, argument string) {
		fmt.Printf("%-10s %q\n", name, argument)
	})
	if err != nil {
		// err is a *getoptlong.Error: InvalidOption / MissingArgument / ...
		os.Exit(1)
	}
	fmt.Println("operands:", p.Args) // the remaining, non-option arguments
}
```

```
$ fib --number 6 -v -- extra
--number   "6"
--verbose  ""
operands: [extra]
```

## Ordering

The three ordering modes mirror Ruby's `GetoptLong` exactly (set with
`SetOrdering`). Given `Foo --zzz Bar --xxx Baz Bat` with `--xxx` taking a
required argument and `--zzz` taking none:

| Ordering        | Options yielded                                  | `p.Args` after |
| --------------- | ------------------------------------------------ | -------------- |
| `Permute`       | `("--zzz","")`, `("--xxx","Baz")`                | `[Foo Bat]`    |
| `RequireOrder`  | *(none — first word is an operand)*              | `[Foo --zzz Bar --xxx Baz Bat]` |
| `ReturnInOrder` | `("","Foo")`, `("--zzz","")`, `("--xxx","Baz")`, `("","Bat")` | `[]` |

Unlike Ruby, this package does **not** consult the `POSIXLY_CORRECT` environment
variable; pass `RequireOrder` explicitly for POSIX behaviour.

## API

```go
// New configures a Parser to scan args for the given options (PERMUTE ordering).
func New(args []string, options ...Option) (*Parser, error)
func NewParser() *Parser

type ArgumentFlag int
const (NoArgument; RequiredArgument; OptionalArgument)

type Ordering int
const (RequireOrder; Permute; ReturnInOrder)

type Option struct { Names []string; Flag ArgumentFlag }

type Parser struct {
	Args        []string  // working argv; leftover operands after scanning
	ProgName    string    // error-message prefix (Ruby's $0)
	ErrorWriter io.Writer // error sink (Ruby's $stderr); nil discards
	// ...
}

func (p *Parser) GetNext() (name, argument string, ok bool, err error) // GetoptLong#get
func (p *Parser) Each(fn func(name, argument string)) error            // GetoptLong#each
func (p *Parser) SetOptions(options ...Option) error                  // set_options
func (p *Parser) SetOrdering(o Ordering) error                        // ordering=
func (p *Parser) Ordering() Ordering
func (p *Parser) SetQuiet(q bool); func (p *Parser) Quiet() bool      // quiet=
func (p *Parser) Err() *Error; func (p *Parser) ErrorMessage() string
func (p *Parser) Terminated() bool

type Error struct { Kind ErrorKind; Message string }      // GetoptLong::Error
type ErrorKind int                                        // InvalidOption / MissingArgument /
                                                          // NeedlessArgument / AmbiguousOption
type SpecError struct { Message string }                  // ArgumentError from New / SetOptions
```

`GetNext` returns `("", "", false, nil)` when there are no more options; when
ordering is `ReturnInOrder` an operand is returned as `("", word, true, nil)`. On
a parse error it returns `("", "", false, *Error)` and records the error
(`Err` / `ErrorMessage`); subsequent calls yield nothing.

## Tests & coverage

The suite pairs deterministic, ruby-free golden tests (which alone hold coverage
at 100%, so the qemu cross-arch and Windows lanes pass the gate) with a
**differential MRI oracle**: every scenario — all three orderings, long / short /
abbreviated names, `=`-joined / separate / bundled arguments, the `--`
terminator, and each error class — is run through both this package and the
system `ruby` and compared byte-for-byte (option stream, leftover argv, error
class + message). The oracle scripts `$stdout.binmode` / `$stdin.binmode` so
Windows text-mode never pollutes the bytes, are gated on `RUBY_VERSION >= "4.0"`,
and skip themselves where `ruby` is absent.

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # 100.0%
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-getoptlong/getoptlong authors.

## WebAssembly

Being pure Go (CGO=0), this library also compiles to **WebAssembly** — both
`GOOS=js GOARCH=wasm` (browser / Node.js) and `GOOS=wasip1 GOARCH=wasm` (WASI).
CI builds both targets on every push, alongside the six 64-bit native/qemu arches.

```sh
GOOS=js     GOARCH=wasm go build ./...   # browser / Node
GOOS=wasip1 GOARCH=wasm go build ./...   # WASI (wasmtime, wasmer, wasmedge, …)
```
