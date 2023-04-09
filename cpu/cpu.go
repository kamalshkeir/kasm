// Pakage cpu provides APIs to detect CPU features available at runtime.
package cpu

import (
	"github.com/kamalshkeir/kasm/cpu/arm"
	"github.com/kamalshkeir/kasm/cpu/arm64"
	"github.com/kamalshkeir/kasm/cpu/x86"
)

var (
	// X86 is the bitset representing the set of the x86 instruction sets are
	// supported by the CPU.
	X86 = x86.ABI()

	// ARM is the bitset representing which parts of the arm instruction sets
	// are supported by the CPU.
	ARM = arm.ABI()

	// ARM64 is the bitset representing which parts of the arm64 instruction
	// sets are supported by the CPU.
	ARM64 = arm64.ABI()
)
