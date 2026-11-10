// Package libpkg is not a main package, so os.Exit calls are allowed here.
package libpkg

import "os"

func Cleanup() {
	os.Exit(0)
}
