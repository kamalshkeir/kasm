//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"math/bits"

	. "github.com/kamalshkeir/kasm/build/internal/x86"
	"github.com/kamalshkeir/kasm/cpu"
	. "github.com/mmcloughlin/avo/build"
	. "github.com/mmcloughlin/avo/operand"
	. "github.com/mmcloughlin/avo/reg"
)

func init() {
	ConstraintExpr("!purego")
}

func main() {
	generateCountPair(countPair1{})
	generateCountPair(countPair2{})
	generateCountPair(countPair4{})
	generateCountPair(countPair8{})
	generateCountPair(countPair16{})
	generateCountPair(countPair32{})
	Generate()
}

type countPair interface {
	size() int
	test(a, b Mem)
}

type countPairAVX2 interface {
	countPair
	vpcmpeq(src0, src1, dst VecVirtual)
	vpmovmskb(tmp, src VecVirtual, dst Register)
}

type countPair1 struct{}

func (countPair1) size() int                             { return 1 }
func (countPair1) test(a, b Mem)                         { generateCountPairTest(MOVB, CMPB, GP8, a, b) }
func (countPair1) vpcmpeq(a, b, c VecVirtual)            { VPCMPEQB(a, b, c) }
func (countPair1) vpmovmskb(_, a VecVirtual, b Register) { VPMOVMSKB(a, b) }

type countPair2 struct{}

func (countPair2) size() int                             { return 2 }
func (countPair2) test(a, b Mem)                         { generateCountPairTest(MOVW, CMPW, GP16, a, b) }
func (countPair2) vpcmpeq(a, b, c VecVirtual)            { VPCMPEQW(a, b, c) }
func (countPair2) vpmovmskb(_, a VecVirtual, b Register) { VPMOVMSKB(a, b) }

type countPair4 struct{}

func (countPair4) size() int                             { return 4 }
func (countPair4) test(a, b Mem)                         { generateCountPairTest(MOVL, CMPL, GP32, a, b) }
func (countPair4) vpcmpeq(a, b, c VecVirtual)            { VPCMPEQD(a, b, c) }
func (countPair4) vpmovmskb(_, a VecVirtual, b Register) { VPMOVMSKB(a, b) }

type countPair8 struct{}

func (countPair8) size() int                             { return 8 }
func (countPair8) test(a, b Mem)                         { generateCountPairTest(MOVQ, CMPQ, GP64, a, b) }
func (countPair8) vpcmpeq(a, b, c VecVirtual)            { VPCMPEQQ(a, b, c) }
func (countPair8) vpmovmskb(_, a VecVirtual, b Register) { VPMOVMSKB(a, b) }

type countPair16 struct{}

func (countPair16) size() int {
	return 16
}
func (countPair16) test(a, b Mem) {
	r0, r1 := XMM(), XMM()
	MOVOU(a, r0)
	MOVOU(b, r1)
	mask := GP32()
	PCMPEQQ(r0, r1)
	PMOVMSKB(r1, mask)
	CMPL(mask, U32(0xFFFF))
}
func (countPair16) vpcmpeq(a, b, c VecVirtual) {
	VPCMPEQQ(a, b, c)
}
func (countPair16) vpmovmskb(tmp, src VecVirtual, dst Register) {
	// https://www.felixcloutier.com/x86/vpermq#vpermq--vex-256-encoded-version-
	//
	// Swap each quad word in the lower and upper half of the 32 bytes register,
	// then AND the src and tmp registers to zero each halves that were partial
	// equality; only fully equal 128 bits need to result in setting 1 bits in
	// the destination mask.
	const permutation = (1 << 0) | (0 << 2) | (3 << 4) | (2 << 6)
	VPERMQ(Imm(permutation), src, tmp)
	VPAND(src, tmp, tmp)
	VPMOVMSKB(tmp, dst)
}

type countPair32 struct{}

func (countPair32) size() int {
	return 32
}
func (countPair32) test(a, b Mem) {
	r0, r1, r2, r3 := XMM(), XMM(), XMM(), XMM()
	MOVOU(a, r0)
	MOVOU(a.Offset(16), r1)
	MOVOU(b, r2)
	MOVOU(b.Offset(16), r3)
	mask0, mask1 := GP32(), GP32()
	PCMPEQQ(r0, r2)
	PCMPEQQ(r1, r3)
	PMOVMSKB(r2, mask0)
	PMOVMSKB(r3, mask1)
	ANDL(mask1, mask0)
	CMPL(mask0, U32(0xFFFF))
}
func (countPair32) vpcmpeq(a, b, c VecVirtual) {
	VPCMPEQQ(a, b, c)
}
func (countPair32) vpmovmskb(_, src VecVirtual, dst Register) {
	VPMOVMSKB(src, dst)
}

func generateCountPair(code countPair) {
	size := code.size()
	TEXT(fmt.Sprintf("countPair%d", size), NOSPLIT, "func(b []byte) int")

	p := Load(Param("b").Base(), GP64())
	n := Load(Param("b").Len(), GP64())
	r := GP64()
	XORQ(r, r)
	SUBQ(Imm(uint64(size)), n)

	if _, ok := code.(countPairAVX2); ok {
		JumpIfFeature("avx2", cpu.AVX2)
	}

	Label("tail")
	CMPQ(n, Imm(0))
	JLE(LabelRef("done"))

	Label("generic")
	x := GP64()
	MOVQ(r, x)
	INCQ(x)
	code.test(Mem{Base: p}, (Mem{Base: p}).Offset(size))
	CMOVQEQ(x, r)
	ADDQ(Imm(uint64(size)), p)
	SUBQ(Imm(uint64(size)), n)
	CMPQ(n, Imm(0))
	JG(LabelRef("generic"))

	Label("done")
	Store(r, ReturnIndex(0))
	RET()

	if avx, ok := code.(countPairAVX2); ok {
		const avxChunk = 256
		const avxLanes = avxChunk / 32
		Label("avx2")
		CMPQ(n, U32(avxChunk+uint64(size)))
		JL(LabelRef(fmt.Sprintf("avx2_tail%d", avxChunk/2)))

		masks := make([]GPVirtual, avxLanes)
		for i := range masks {
			masks[i] = GP64()
			XORQ(masks[i], masks[i])
		}

		regA := make([]VecVirtual, avxLanes)
		regB := make([]VecVirtual, avxLanes)
		for i := range regA {
			regA[i] = YMM()
			regB[i] = YMM()
		}

		Label(fmt.Sprintf("avx2_loop%d", avxChunk))
		generateCountPairAVX2(r, p, regA, regB, masks, avx)
		ADDQ(U32(avxChunk), p)
		SUBQ(U32(avxChunk), n)
		CMPQ(n, U32(avxChunk+uint64(size)))
		JGE(LabelRef(fmt.Sprintf("avx2_loop%d", avxChunk)))

		for chunk := avxChunk / 2; chunk >= 32; chunk /= 2 {
			Label(fmt.Sprintf("avx2_tail%d", chunk))
			CMPQ(n, Imm(uint64(chunk+size)))
			JL(LabelRef(fmt.Sprintf("avx2_tail%d", chunk/2)))
			lanes := chunk / 32
			generateCountPairAVX2(r, p, regA[:lanes], regB[:lanes], masks[:lanes], avx)
			ADDQ(U32(uint64(chunk)), p)
			SUBQ(U32(uint64(chunk)), n)
		}

		Label("avx2_tail16")
		if size < 16 {
			CMPQ(n, Imm(uint64(16+size)))
			JL(LabelRef("avx2_tail"))
			generateCountPairAVX2(r, p, []VecVirtual{XMM()}, []VecVirtual{XMM()}, masks[:1], avx)
			ADDQ(Imm(16), p)
			SUBQ(Imm(16), n)
		}

		Label("avx2_tail")
		VZEROUPPER()
		if size < 32 {
			if shift := divideShift(size); shift > 0 {
				SHRQ(Imm(uint64(shift)), r)
			}
		}
		JMP(LabelRef("tail"))
	}
}

func generateCountPairTest(mov func(Op, Op), cmp func(Op, Op), reg func() GPVirtual, a, b Mem) {
	r := reg()
	mov(a, r)
	cmp(r, b)
}

func generateCountPairAVX2(r, p Register, regA, regB []VecVirtual, masks []GPVirtual, code countPairAVX2) {
	size := code.size()
	moves := make(map[int]VecVirtual)

	for i, reg := range regA {
		VMOVDQU((Mem{Base: p}).Offset(i*32), reg)
		moves[i*32] = reg
	}

	for i, reg := range regB {
		// Skip loading from memory a second time if we already loaded the
		// offset in the previous loop. This optimization applies for items
		// of size 32.
		if moves[i*32+size] == nil {
			lo := moves[i*32+(size-16)]
			hi := moves[i*32+(size+16)]

			if lo != nil && hi != nil {
				// https://www.felixcloutier.com/x86/vperm2i128#vperm2i128
				//
				// The data was already loaded, but split across two registers.
				// We recompose it using a permutation of the upper and lower
				// halves of the registers holding the contiguous data.
				//
				// Note that in Go assembly the arguments are reversed;
				// SRC1 is `lo` and SRC2 is `hi`, but we pass them in the
				// reverse order.
				const permutation = (1 << 0) | (2 << 4)
				VPERM2I128(Imm(permutation), hi, lo, reg)
			} else {
				VMOVDQU((Mem{Base: p}).Offset(i*32+size), reg)
			}
		}
	}

	for i := range regA {
		// The load may have been elided if there was offset overlaps between
		// the two sources.
		if mov := moves[i*32+size]; mov != nil {
			code.vpcmpeq(regA[i], mov, regB[i])
		} else {
			code.vpcmpeq(regA[i], regB[i], regB[i])
		}
		code.vpmovmskb(regA[i], regB[i], masks[i].As32())
	}

	for _, mask := range masks {
		POPCNTQ(mask, mask)
		if size == 32 {
			SHRQ(Imm(uint64(divideShift(size))), mask)
		}
	}

	ADDQ(divideAndConquerSum(masks), r)
}

func divideShift(size int) int {
	return bits.TrailingZeros(uint(size))
}

func divideAndConquerSum(regs []GPVirtual) GPVirtual {
	switch len(regs) {
	case 1:
		return regs[0]

	case 2:
		r0, r1 := regs[0], regs[1]
		ADDQ(r1, r0)
		return r0

	default:
		i := len(regs) / 2
		r0 := divideAndConquerSum(regs[:i])
		r1 := divideAndConquerSum(regs[i:])
		ADDQ(r1, r0)
		return r0
	}
}
