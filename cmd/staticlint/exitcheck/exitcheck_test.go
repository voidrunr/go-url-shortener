package exitcheck_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/voidrunr/go-url-shortener/cmd/staticlint/exitcheck"
)

func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), exitcheck.Analyzer, "bad", "good", "gen", "libpkg")
}
