package types

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestHash32_StringAndHex(t *testing.T) {
	var h Hash32
	for i := range h {
		h[i] = byte(i)
	}
	want := "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	if got := h.String(); got != want {
		t.Fatalf("Hash32.String() = %q, want %q", got, want)
	}
}

func TestHash32_JSONRoundTrip(t *testing.T) {
	var h Hash32
	for i := range h {
		h[i] = byte(0xA0 + i)
	}
	data, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Hash32
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got != h {
		t.Fatalf("round trip mismatch: got %v, want %v", got, h)
	}
}

func TestHash32_SetHexErrors(t *testing.T) {
	cases := map[string]string{
		"short":      "deadbeef",
		"odd length": strings.Repeat("a", 63),
		"non-hex":    strings.Repeat("z", 64),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			var h Hash32
			if err := h.SetHex(input); err == nil {
				t.Fatalf("expected error for %s", name)
			}
		})
	}
}

func TestHash32_IsZero(t *testing.T) {
	if !ZeroHash.IsZero() {
		t.Fatal("ZeroHash.IsZero() = false")
	}
	var h Hash32
	h[5] = 1
	if h.IsZero() {
		t.Fatal("non-zero Hash32 reported IsZero")
	}
}

func TestAddress_JSONRoundTrip(t *testing.T) {
	var a Address
	for i := range a {
		a[i] = byte(i * 7)
	}
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Address
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got != a {
		t.Fatalf("round trip mismatch")
	}
}

func TestAmount_SafeAdd(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		got, err := Amount(10).SafeAdd(5)
		if err != nil || got != 15 {
			t.Fatalf("got (%d, %v), want (15, nil)", got, err)
		}
	})
	t.Run("overflow", func(t *testing.T) {
		_, err := Amount(math.MaxUint64).SafeAdd(1)
		if !errors.Is(err, ErrAmountOverflow) {
			t.Fatalf("got err %v, want ErrAmountOverflow", err)
		}
	})
}

func TestAmount_SafeSub(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		got, err := Amount(10).SafeSub(3)
		if err != nil || got != 7 {
			t.Fatalf("got (%d, %v), want (7, nil)", got, err)
		}
	})
	t.Run("underflow", func(t *testing.T) {
		_, err := Amount(1).SafeSub(2)
		if !errors.Is(err, ErrAmountOverflow) {
			t.Fatalf("got err %v, want ErrAmountOverflow", err)
		}
	})
}

func TestHashFromBytes(t *testing.T) {
	good := make([]byte, HashSize)
	for i := range good {
		good[i] = byte(i + 1)
	}
	h, err := HashFromBytes(good)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := range h {
		if h[i] != good[i] {
			t.Fatalf("byte %d: got %d, want %d", i, h[i], good[i])
		}
	}
	if _, err := HashFromBytes(good[:31]); err == nil {
		t.Fatal("expected error for short input")
	}
}
