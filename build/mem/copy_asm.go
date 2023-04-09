//go:build ignore
// +build ignore

package main

import (
	"github.com/kamalshkeir/kasm/build/internal/x86"
	. "github.com/mmcloughlin/avo/build"
)

func init() {
	ConstraintExpr("!purego")
}

func main() {
	x86.GenerateCopy("Copy", "copies src to dst, returning the number of bytes written.", nil)
}
