// Package sample holds small structs marked with // generate:reset.
// They are used to exercise and verify the reset generator.
package sample

import "time"

// Number is a named primitive type: its reset must use the zero
// literal of its underlying kind, not a struct literal.
type Number int

// Sub is a nested struct whose Reset must be chained from owners.
//
// generate:reset
type Sub struct {
	n   int
	txt string
}

// Sample covers every reset rule at once: primitives (plain and
// named), slices, maps, pointers to primitive and struct types, time
// values, and nested resettable structs.
//
// generate:reset
type Sample struct {
	i     int
	s     string
	nu    Number
	sp    *string
	f     float64
	b     bool
	items []string
	m     map[string]string
	t     time.Time
	tp    *time.Time
	child *Sample
	other *Sub
}
