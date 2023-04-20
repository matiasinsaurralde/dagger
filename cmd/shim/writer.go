package main

import (
	"bytes"
	"io"
	"log"
	"unicode/utf8"
)

type UTF8DanglingWriter struct {
	dangling []byte
	w        io.Writer
}

func NewUTF8DanglingWriter(w io.Writer) *UTF8DanglingWriter {
	return &UTF8DanglingWriter{
		w: w,
	}
}

func (w *UTF8DanglingWriter) Write(b []byte) (int, error) {
	data := w.writeDangling(b)
	_, err := w.w.Write(data)
	return len(b), err
}

func (w *UTF8DanglingWriter) writeDangling(b []byte) []byte {
	data := append(w.dangling, b...)

	checkEncoding, _ := utf8.DecodeLastRune(data)
	if checkEncoding == utf8.RuneError {
		w.dangling = data
		return nil
	}

	w.dangling = nil
	return data
}

type LineBreakWriter struct {
	buffer []byte
	w      io.Writer

	secretLines []string
}

func NewLineBreakWriter(w io.Writer) *LineBreakWriter {
	return &LineBreakWriter{
		w: w,
	}
}

func (w *LineBreakWriter) Write(b []byte) (int, error) {
	data := w.writeDangling(b)
	_, err := w.w.Write(data)
	return len(b), err
}

func (w *LineBreakWriter) writeDangling(b []byte) []byte {
	data := append(w.buffer, b...)

	idx := bytes.LastIndex(data, []byte("\n"))

	if idx == -1 {
		idx = 0
	}

	if idx == len(data)-1 {
		w.buffer = nil
		return data
	}

	// eg. secret=yummy, data[idx:]="allo allo, yu", "yu" == secret[0:2]
	// if not, don't buffer, just write on the underlying writer
	contains, newIdx := containsPartialSecret(data[idx:], w.secretLines)
	if contains {
		w.buffer = data[newIdx:]
		return data[:newIdx]
	}

	w.buffer = nil
	return data
}

func containsPartialSecret(b []byte, secrets []string) (contains bool, index int) {
	for _, secret := range secrets {
		for i := len(secret); i > 0; i-- {
			secretContained := bytes.Contains(b, []byte(secret)[:i])
			log.Println("contained:", secret, secretContained, secret[:i])
			if !secretContained {
				// we go check a smaller set in this secret
				continue
			}

			idx := bytes.LastIndex(b, []byte(secret)[:i])
			// if the last occurence of the partial secret goes till the end of b
			// it means that there is potentially a partial secret at the end of b
			// so we need to buffer it
			if idx+len(secret[:i]) == len(b) && i == len(secret) { // if i == len(secret), it means we could do a full match scrubbing
				break
			}
			return true, idx
		}
	}
	return false, -1
}
