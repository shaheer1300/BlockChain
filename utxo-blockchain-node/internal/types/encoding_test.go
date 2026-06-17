package types

import (
	"bytes"
	"errors"
	"testing"
)

func TestCanonicalEncoder_Determinism(t *testing.T) {
	encode := func() []byte {
		var buf bytes.Buffer
		e := newCanonicalEncoder(&buf)
		e.writeUint32(0xDEADBEEF)
		e.writeUint64(0x0102030405060708)
		e.writeVarBytes([]byte{1, 2, 3, 4})
		if err := e.Err(); err != nil {
			t.Fatalf("encoder error: %v", err)
		}
		return buf.Bytes()
	}
	a := encode()
	b := encode()
	if !bytes.Equal(a, b) {
		t.Fatalf("encoder is not deterministic:\n a=%x\n b=%x", a, b)
	}
}

func TestCanonicalEncoder_Uint32LittleEndian(t *testing.T) {
	var buf bytes.Buffer
	e := newCanonicalEncoder(&buf)
	e.writeUint32(0x01020304)
	if err := e.Err(); err != nil {
		t.Fatalf("encoder error: %v", err)
	}
	want := []byte{0x04, 0x03, 0x02, 0x01}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("got %x, want %x", buf.Bytes(), want)
	}
}

func TestCanonicalEncoder_Uint64LittleEndian(t *testing.T) {
	var buf bytes.Buffer
	e := newCanonicalEncoder(&buf)
	e.writeUint64(0x0102030405060708)
	if err := e.Err(); err != nil {
		t.Fatalf("encoder error: %v", err)
	}
	want := []byte{0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("got %x, want %x", buf.Bytes(), want)
	}
}

func TestCanonicalEncoder_VarBytesLength(t *testing.T) {
	var buf bytes.Buffer
	e := newCanonicalEncoder(&buf)
	e.writeVarBytes([]byte{0x01, 0x02, 0x03})
	if err := e.Err(); err != nil {
		t.Fatalf("encoder error: %v", err)
	}
	want := []byte{0x03, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("got %x, want %x", buf.Bytes(), want)
	}
}

func TestCanonicalEncoder_VarBytesEmpty(t *testing.T) {
	var buf bytes.Buffer
	e := newCanonicalEncoder(&buf)
	e.writeVarBytes(nil)
	if err := e.Err(); err != nil {
		t.Fatalf("encoder error: %v", err)
	}
	want := []byte{0x00, 0x00, 0x00, 0x00}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("got %x, want %x", buf.Bytes(), want)
	}
}

func TestCanonicalEncoder_LenTooLong(t *testing.T) {
	var buf bytes.Buffer
	e := newCanonicalEncoder(&buf)
	e.writeLen(MaxCanonicalSliceLen + 1)
	if !errors.Is(e.Err(), ErrSliceTooLong) {
		t.Fatalf("got err %v, want ErrSliceTooLong", e.Err())
	}
}

func TestCanonicalEncoder_StickyError(t *testing.T) {
	var buf bytes.Buffer
	e := newCanonicalEncoder(&buf)
	e.writeLen(MaxCanonicalSliceLen + 1)
	e.writeUint32(0xAABBCCDD)
	if buf.Len() != 0 {
		t.Fatalf("expected no bytes written after sticky error, got %d", buf.Len())
	}
}
