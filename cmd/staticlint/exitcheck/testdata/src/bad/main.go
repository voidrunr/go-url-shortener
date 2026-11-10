package main

import (
	"log"
	"os"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			os.Exit(2) // want `direct os\.Exit call in main function of main package is forbidden`
		}
	}()

	if err := run(); err != nil {
		os.Exit(1) // want `direct os\.Exit call in main function of main package is forbidden`
	}

	go func() {
		os.Exit(3) // want `direct os\.Exit call in main function of main package is forbidden`
	}()
}

func run() error { return nil }

// exitFromHelper is out of scope: the analyzer checks only the main function.
func exitFromHelper() {
	os.Exit(4)

	_ = log.Printf
}

// aliasExit uses an import alias, the call must still be recognized by
// type information.
func aliasExit() {
	osExit := os.Exit
	_ = osExit
}
