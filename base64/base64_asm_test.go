//go:build (amd64 || arm64) && !purego
// +build amd64 arm64
// +build !purego

package base64

import (
	"fmt"
	"testing"

	"github.com/kamalshkeir/kasm/internal/buffer"
)

func fillBuffers(b *buffer.Buffer, size int) map[string][]byte {
	bufs := map[string][]byte{
		"head": b.ProtectHead(),
		"tail": b.ProtectTail(),
	}

	for _, buf := range bufs {
		for i := 0; i < size; i++ {
			buf[i] = (255 - byte(i&15)*16) - byte(i&255)/16
		}
	}

	return bufs
}

func TestEncodeASM(t *testing.T) {
	b, err := buffer.New(512)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Release()

	bufs := fillBuffers(&b, 512)

	for _, enc := range encodings {
		if enc.candidate.enc == nil {
			t.Log("asm not enabled")
			continue
		}
		for name, buf := range bufs {
			ok := t.Run(fmt.Sprintf("%s-%s", enc.name, name), func(t *testing.T) {
				dst, err := buffer.New(enc.candidate.EncodedLen(len(buf)))
				if err != nil {
					t.Fatal(err)
				}
				defer dst.Release()

				_, ns := enc.candidate.enc(dst.ProtectTail(), buf, &enc.candidate.enclut[0])

				if len(buf)-ns >= minEncodeLen {
					t.Errorf("encode remain should be less than %d, but is %d", minEncodeLen, len(buf)-ns)
				}
			})

			if !ok {
				break
			}
		}
	}
}

func TestDecodeASM(t *testing.T) {
	b, err := buffer.New(512)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Release()

	bufs := fillBuffers(&b, 512)

	for _, enc := range encodings {
		if enc.candidate.dec == nil {
			t.Log("asm not enabled")
			continue
		}

		for name, buf := range bufs {
			ok := t.Run(fmt.Sprintf("%s-%s", enc.name, name), func(t *testing.T) {
				src := make([]byte, enc.candidate.EncodedLen(len(buf)))
				dst, err := buffer.New(len(buf))
				if err != nil {
					t.Fatal(err)
				}
				defer dst.Release()

				enc.candidate.Encode(src, buf)

				_, ns := enc.candidate.dec(dst.ProtectTail(), src, &enc.candidate.declut[0])

				if len(buf)-ns >= minDecodeLen {
					t.Errorf("decode remain should be less than %d, but is %d", minDecodeLen, len(buf)-ns)
				}
			})

			if !ok {
				break
			}
		}
	}
}
