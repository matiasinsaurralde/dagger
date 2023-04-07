package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUTF8DanglingWriter(t *testing.T) {
	cases := []struct {
		s     string
		split int
	}{
		{"Forêt", 4},
		{"Ah ! Ça ira, ça ira !", 7},
		{"🇫🇷 🇺🇾 🇦🇷 🇺🇸 🇵🇹 🇮🇳 🇨🇦 🇬🇧", 2},
	}

	w := NewUTF8DanglingWriter(&bytes.Buffer{})

	for _, c := range cases {
		prev := 0
		next := c.split
		for {
			if next > len(c.s) {
				next = len(c.s)
			}

			s := c.s[prev:next]

			n, err := w.Write([]byte(s))
			require.NoError(t, err)
			require.Equal(t, len(s), n)

			if next == len(c.s) {
				break
			}

			prev = next
			next += c.split
		}
		require.Equal(t, c.s, w.w.(*bytes.Buffer).String())
		w.w.(*bytes.Buffer).Reset()
	}
}
