package prompt

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// Reader keeps buffered input across a sequence of prompts, including piped
// answers. Once a read is canceled, discard the Reader; its input may still be
// blocked in an OS read until the process exits.
type Reader struct {
	in  *bufio.Reader
	out io.Writer
}

func NewReader(in io.Reader, out io.Writer) *Reader {
	return &Reader{in: bufio.NewReader(in), out: out}
}

func (r *Reader) String(ctx context.Context, label string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if _, err := fmt.Fprintf(r.out, "%s: ", label); err != nil {
		return "", err
	}
	type answer struct {
		value string
		err   error
	}
	ch := make(chan answer, 1)
	go func() {
		value, err := r.in.ReadString('\n')
		if err == io.EOF && value != "" {
			err = nil
		}
		ch <- answer{strings.TrimSpace(value), err}
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case result := <-ch:
		return result.value, result.err
	}
}
