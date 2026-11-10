// Command staticlint is a multichecker for the go-url-shortener project.
//
// It bundles five groups of go/analysis analyzers into a single binary:
//
//   - every diagnostic analyzer from the standard
//     golang.org/x/tools/go/analysis/passes tree (the go vet set plus the
//     extra passes that ship with x/tools, including the modernize suite);
//   - all analyzers of the SA class from staticcheck.io (honnef.co/go/tools),
//     which hunt probable bugs, API misuse and performance issues;
//   - the unused check (U1000) of staticcheck.io, an analyzer of a class
//     other than SA;
//   - two public third-party analyzers: bodyclose and nilerr;
//   - the project's own exitcheck analyzer, which forbids direct os.Exit
//     calls in the main function of the main package.
//
// # Running
//
// The command is driven by the multichecker driver from
// golang.org/x/tools/go/analysis/multichecker, which accepts the same
// package patterns as go build and runs every registered analyzer over
// them, resolving analyzer dependencies (SSA construction and friends)
// automatically.
//
// Analyze the whole module:
//
//	go run ./cmd/staticlint ./...
//
// Analyze a single package:
//
//	go run ./cmd/staticlint ./internal/handler
//
// The driver reports each diagnostic as file:line:column: message and
// exits with a non-zero status if at least one diagnostic was printed,
// which makes it usable in CI. A build or load error in the analyzed
// code is reported as well.
//
// Every analyzer can be toggled individually with its -NAME.enable flag,
// and analyzer-specific flags are exposed as -NAME.flag. Useful examples:
//
//	go run ./cmd/staticlint -fieldalignment.enable=false ./...
//	go run ./cmd/staticlint -shadow.enable=true -v ./...
//
// Other driver flags: -v prints the list of registered analyzers and
// per-package timings, -json emits machine-readable diagnostics, -fix
// applies the suggested fixes of the analyzers that provide them
// (the modernize suite does), -flags prints the flags of the analyzers.
//
// The makefile wraps the common invocation:
//
//	make staticlint
//
// # Standard analyzers (golang.org/x/tools/go/analysis/passes)
//
// These are the diagnostic passes of x/tools: the same set the go vet
// command runs by default, plus the passes that ship with x/tools but are
// not enabled by vet. Infrastructure passes (buildssa, inspect, ctrlflow)
// and demonstration passes (findcall, pkgfact, usesgenerics) are omitted:
// they emit no diagnostics themselves and are pulled in automatically as
// dependencies of the checks that use them.
//
//   - appends: append called with a single argument (append(s)).
//   - asmdecl: mismatches between assembly files and Go declarations.
//   - assign: useless assignments, such as x = x or x = v.
//   - atomic: common mistakes around sync/atomic functions.
//   - atomicalign: non-64-bit-aligned arguments to sync/atomic on 32-bit
//     platforms.
//   - bools: degenerate boolean expressions, such as a == a or a && a.
//   - buildtag: malformed //go:build and // +build lines.
//   - cgocall: violations of the cgo pointer-passing rules.
//   - composite: unkeyed composite literals of foreign structs.
//   - copylock: sync.Lock values passed or assigned by value.
//   - deepequalerrors: reflect.DeepEqual applied to error values.
//   - defers: common mistakes in defer statements (defer of nil function,
//     deferred call result discarded and the like).
//   - directive: validation of known toolchain //go: directives.
//   - errorsas: second argument of errors.As is not a pointer to a type
//     implementing error.
//   - fieldalignment: structs whose fields could be reordered to use less
//     memory.
//   - framepointer: assembly that clobbers the frame pointer without
//     saving it.
//   - gofix: validation of //go:fix inline migration directives.
//   - hostport: net.Dial calls with "%s:%d" addresses that break for IPv6.
//   - httpmux: mistakes in net/http.ServeMux pattern registration.
//   - httpresponse: using the result of an HTTP request before the
//     response body has been closed or in a wrong way.
//   - ifaceassert: interface-to-interface type assertions that are always
//     impossible.
//   - loopclosure: references to loop variables from nested functions that
//     outlive the iteration (pre-Go 1.22 semantics).
//   - lostcancel: a context.CancelFunc that is dropped instead of being
//     called.
//   - modernize: the x/tools modernize suite, suggesting modern language
//     and stdlib equivalents: any instead of interface{}, min/max instead
//     of if-chains, slices/maps helpers, for range int, strings.Builder,
//     and so on.
//   - nilfunc: comparing a function value against nil, which is almost
//     always a mistake.
//   - nilness: SSA-based check for nil dereferences and degenerate nil
//     comparisons.
//   - printf: consistency of Printf-family format strings and arguments.
//   - reflectvaluecompare: comparing reflect.Value values with == or
//     reflect.DeepEqual.
//   - scannererr: uses of bufio.Scanner that never check Scanner.Err.
//   - shadow: variables that shadow an outer declaration.
//   - shift: shifts by more bits than the operand width.
//   - sigchanyzer: unbuffered channels passed to signal.Notify.
//   - slog: mismatched key-value pairs in log/slog calls.
//   - sortslice: sort.Slice called on a value that is not a slice.
//   - sqlrowserr: uses of *sql.Rows that never check Rows.Err.
//   - stdmethods: method signatures that look like well-known stdlib
//     methods (Format, Scan, ReadRune, ...) but deviate from them.
//   - stdversion: standard library symbols too new for the go directive
//     of go.mod.
//   - stringintconv: string(int) conversions, almost always a mistake for
//     string(rune).
//   - structtag: malformed or duplicate struct tags.
//   - testinggoroutine: calls to t.Fatal from a non-test goroutine.
//   - tests: malformed names or signatures of test, benchmark and example
//     functions.
//   - timeformat: time.Format calls with a format other than
//     time.RFC3339 (2006-01-02 layout).
//   - unmarshal: non-pointer values passed to Unmarshal and Decode.
//   - unreachable: unreachable code.
//   - unsafeptr: invalid conversions of uintptr to unsafe.Pointer.
//   - unusedresult: results of pure calls (fmt.Sprintf, strings.Replace
//     and the like) discarded without use.
//   - unusedwrite: writes to a variable that is never read afterwards.
//   - waitgroup: misuse of sync.WaitGroup, such as Add inside the
//     goroutine it is supposed to wait for.
//
// # staticcheck SA analyzers (honnef.co/go/tools/staticcheck)
//
// All analyzers of the SA class are registered. They report probable
// bugs, misuse of the standard library and performance problems:
//
//   - SA1000–SA1008: invalid regexps, templates, time layouts, exec
//     invocations, URL parsing, http.Header keys and similar API misuse.
//   - SA1010–SA1011: zero-result FindAll, invalid UTF-8 passed to
//     strings functions.
//   - SA1012–SA1018: nil context.Context, io.Seeker whence order,
//     non-pointer Unmarshal arguments, leaking time.Tick, untrappable
//     signals, unbuffered signal.Notify channels, no-op strings.Replace.
//   - SA1019: usage of deprecated functions, variables and fields.
//   - SA1020–SA1029: invalid host:port, bytes.Equal on net.IP, io.Writer
//     buffer modification, duplicate string cutsets, useless
//     time.Timer.Reset results, unmarshalable channels and functions,
//     unaligned 64-bit atomics, sort.Slice on non-slices, inappropriate
//     context.WithValue keys.
//   - SA1030–SA1032: invalid strconv arguments, overlapping encoder
//     buffers, reversed errors.Is arguments.
//   - SA2000–SA2003: WaitGroup.Add inside the goroutine, empty critical
//     sections, t.FailNow in goroutines, deferred locks.
//   - SA3000–SA3001: TestMain without os.Exit, assignment to b.N.
//   - SA4000–SA4018: identical operands of binary operators, pointless
//     unsigned/negative comparisons, one-iteration loops, field
//     assignments nobody observes, overwritten arguments, dead append
//     results, no-op breaks, NaN comparisons, double negation, repeated
//     if conditions, useless math on converted integers, x ^ 0, self
//     assignments.
//   - SA4019–SA4026: duplicate build constraints, unreachable type switch
//     cases, x = append(y), comparisons of variable addresses, impossible
//     interface-nil comparisons, impossible builtin results, integer
//     division of literals that yields zero, negative zero constants.
//   - SA4027–SA4032: copied url.URL.Query, x % 1, ineffective sorting and
//     random generation, checks of never-nil values against nil,
//     impossible runtime.GOOS comparisons.
//   - SA5000–SA5005: assignment to nil map, deferred Close without an
//     error check, spinning for loops, defers in infinite loops, busy
//     select loops, finalizers preventing collection.
//   - SA5007–SA5012: infinite recursion, invalid struct tags, invalid
//     Printf calls, impossible type assertions, odd-sized slices, nil
//     channel operations.
//   - SA6000–SA6006: regexp recompilation in loops, byte-slice map index
//     optimization, sync.Pool of non-pointers, string-to-rune-slice
//     conversion before range, inefficient ToLower/ToUpper comparisons,
//     io.WriteString on []byte.
//   - SA9001–SA9010: defers in range loops, non-octal file modes, empty
//     if branches, partial const typing, unmarshalable structs, dubious
//     bit shifts, directory deletion, wrong type-assertion else branch,
//     ineffectual compiler directives, uncalled returned functions in
//     defer.
//
// # U1000 unused (honnef.co/go/tools/unused)
//
// An analyzer of the U class, different from SA. It builds the whole
// call graph of a package and reports unexported identifiers that are
// never used: dead functions, methods, types, fields and variables.
// It requires the SSA representation, which the multichecker builds
// automatically.
//
// # bodyclose (github.com/timakin/bodyclose/passes/bodyclose)
//
// A public third-party analyzer. It checks that the Body of every
// *http.Response obtained from http.Get, http.Post, http.Client.Do and
// friends is closed, so that TCP connections can be reused. It is
// relevant to this project, which performs outgoing HTTP requests in
// internal/audit and cmd/loadgen.
//
// # nilerr (github.com/gostaticanalysis/nilerr)
//
// A public third-party analyzer. It finds code that checks err != nil
// but returns nil afterwards, and the mirror case of returning err when
// err is known to be nil — classic error-handling bugs in defer- and
// middleware-heavy server code.
//
// # exitcheck (cmd/staticlint/exitcheck)
//
// The project's own analyzer. It walks every function named main in a
// package main and reports direct calls to os.Exit made from anywhere in
// its body, including nested function literals, because such calls skip
// deferred cleanup. Calls resolved through an import alias are detected
// via type information. os.Exit in other functions of package main and
// in other packages is allowed. The recommended replacements are
// returning an error from main or a zerolog log.Fatal call. The source
// code of this project is kept clean under this analyzer: the loadgen
// command returns errors to main and terminates through log.Fatal.
package main
