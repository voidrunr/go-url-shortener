// This file is the single source of truth for the analyzer set: every
// analyzer passed to multichecker is registered here, in four groups.
// Infrastructure passes (buildssa, inspect, ctrlflow) and demo passes
// (findcall, pkgfact, usesgenerics) of golang.org/x/tools/go/analysis/passes
// are not listed: they produce no diagnostics of their own and are loaded
// automatically as dependencies of the checkers that need them.

package main

import (
	"sort"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/appends"
	"golang.org/x/tools/go/analysis/passes/asmdecl"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/atomicalign"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/deepequalerrors"
	"golang.org/x/tools/go/analysis/passes/defers"
	"golang.org/x/tools/go/analysis/passes/directive"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/fieldalignment"
	"golang.org/x/tools/go/analysis/passes/framepointer"
	"golang.org/x/tools/go/analysis/passes/gofix"
	"golang.org/x/tools/go/analysis/passes/hostport"
	"golang.org/x/tools/go/analysis/passes/httpmux"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/ifaceassert"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/modernize"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/nilness"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/reflectvaluecompare"
	"golang.org/x/tools/go/analysis/passes/scannererr"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/sigchanyzer"
	"golang.org/x/tools/go/analysis/passes/slog"
	"golang.org/x/tools/go/analysis/passes/sortslice"
	"golang.org/x/tools/go/analysis/passes/sqlrowserr"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/stdversion"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/testinggoroutine"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/timeformat"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"
	"golang.org/x/tools/go/analysis/passes/unusedwrite"
	"golang.org/x/tools/go/analysis/passes/waitgroup"

	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/unused"

	"github.com/gostaticanalysis/nilerr"
	"github.com/timakin/bodyclose/passes/bodyclose"

	"github.com/voidrunr/go-url-shortener/cmd/staticlint/exitcheck"
)

// standardAnalyzers are the diagnostic analyzers from
// golang.org/x/tools/go/analysis/passes, the same checks the go vet
// command runs plus the extra passes that ship with x/tools (defers,
// fieldalignment, hostport, httpmux, nilness, scannererr, shadow,
// sqlrowserr and the modernize suite).
var standardAnalyzers = append(
	[]*analysis.Analyzer{
		appends.Analyzer,
		asmdecl.Analyzer,
		assign.Analyzer,
		atomic.Analyzer,
		atomicalign.Analyzer,
		bools.Analyzer,
		buildtag.Analyzer,
		cgocall.Analyzer,
		composite.Analyzer,
		copylock.Analyzer,
		deepequalerrors.Analyzer,
		defers.Analyzer,
		directive.Analyzer,
		errorsas.Analyzer,
		fieldalignment.Analyzer,
		framepointer.Analyzer,
		gofix.Analyzer,
		hostport.Analyzer,
		httpmux.Analyzer,
		httpresponse.Analyzer,
		ifaceassert.Analyzer,
		loopclosure.Analyzer,
		lostcancel.Analyzer,
	},
	append(
		slices(modernize.Suite),
		nilfunc.Analyzer,
		nilness.Analyzer,
		printf.Analyzer,
		reflectvaluecompare.Analyzer,
		scannererr.Analyzer,
		shadow.Analyzer,
		shift.Analyzer,
		sigchanyzer.Analyzer,
		slog.Analyzer,
		sortslice.Analyzer,
		sqlrowserr.Analyzer,
		stdmethods.Analyzer,
		stdversion.Analyzer,
		stringintconv.Analyzer,
		structtag.Analyzer,
		testinggoroutine.Analyzer,
		tests.Analyzer,
		timeformat.Analyzer,
		unmarshal.Analyzer,
		unreachable.Analyzer,
		unsafeptr.Analyzer,
		unusedresult.Analyzer,
		unusedwrite.Analyzer,
		waitgroup.Analyzer,
	)...,
)

// staticcheckAnalyzers are all analyzers of the SA class from
// honnef.co/go/tools (staticcheck.io), the checks that find probable
// bugs and performance issues.
var staticcheckAnalyzers = unwrap(staticcheck.Analyzers)

// extraStaticcheckAnalyzers holds analyzers of staticcheck.io classes
// other than SA; the unused check (U1000) reports unreachable code.
var extraStaticcheckAnalyzers = []*analysis.Analyzer{
	unused.Analyzer.Analyzer,
}

// thirdPartyAnalyzers are public analyzers from independent authors:
// bodyclose checks that HTTP response bodies are closed, nilerr finds
// code that returns nil even though it checked err != nil.
var thirdPartyAnalyzers = []*analysis.Analyzer{
	bodyclose.Analyzer,
	nilerr.Analyzer,
}

// ownAnalyzers are the analyzers written for this project; exitcheck
// forbids direct os.Exit calls in the main function of the main package.
var ownAnalyzers = []*analysis.Analyzer{
	exitcheck.Analyzer,
}

// analyzers returns the full multichecker set, sorted by name so that
// the -help output and runs are deterministic.
func analyzers() []*analysis.Analyzer {
	all := make([]*analysis.Analyzer, 0,
		len(standardAnalyzers)+
			len(staticcheckAnalyzers)+
			len(extraStaticcheckAnalyzers)+
			len(thirdPartyAnalyzers)+
			len(ownAnalyzers))

	all = append(all, standardAnalyzers...)
	all = append(all, staticcheckAnalyzers...)
	all = append(all, extraStaticcheckAnalyzers...)
	all = append(all, thirdPartyAnalyzers...)
	all = append(all, ownAnalyzers...)

	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	return all
}

// unwrap converts staticcheck lint.Analyzer wrappers into plain
// go/analysis analyzers, which is what multichecker expects.
func unwrap(wrappers []*lint.Analyzer) []*analysis.Analyzer {
	list := make([]*analysis.Analyzer, 0, len(wrappers))
	for _, w := range wrappers {
		list = append(list, w.Analyzer)
	}
	return list
}

// slices copies a slice of analyzers to allow variadic expansion
// without aliasing the source slice.
func slices(src []*analysis.Analyzer) []*analysis.Analyzer {
	out := make([]*analysis.Analyzer, len(src))
	copy(out, src)
	return out
}
