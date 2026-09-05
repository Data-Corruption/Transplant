package prompt

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestReaderKeepsPipedAnswersAndRejectsEOF(t *testing.T) {
	r := NewReader(strings.NewReader("first\n\nlast"), io.Discard)
	for _, want := range []string{"first", "", "last"} {
		got, err := r.String(context.Background(), "answer")
		if err != nil || got != want {
			t.Fatalf("got %q, %v; want %q", got, err, want)
		}
	}
	if _, err := r.String(context.Background(), "answer"); !errors.Is(err, io.EOF) {
		t.Fatalf("got %v; want EOF", err)
	}
}

func TestReaderCancellation(t *testing.T) {
	in, out := io.Pipe()
	defer in.Close()
	defer out.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := NewReader(in, io.Discard).String(ctx, "answer"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v", err)
	}
}
