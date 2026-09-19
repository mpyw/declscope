// sink.go is the writer every renderer prints through. A report is a few
// dozen writes, and a renderer that checked each one would say nothing else;
// one that checked none would return nil however the write went, which is what
// this replaced.
//
// The file is shared on purpose, so it says so once.
//
//declscope:package

package measure

import (
	"fmt"
	"io"
)

// sink writes until the first error and remembers it. Every later write is a
// no-op, so the caller reports the first failure rather than the last.
type sink struct {
	w   io.Writer
	err error
}

func newSink(w io.Writer) *sink { return &sink{w: w} }

func (s *sink) printf(format string, a ...any) {
	if s.err != nil {
		return
	}
	_, s.err = fmt.Fprintf(s.w, format, a...)
}

func (s *sink) print(text string) {
	if s.err != nil {
		return
	}
	_, s.err = io.WriteString(s.w, text)
}

// flush returns the first write error, after flushing whatever the writer
// itself is holding.
func (s *sink) flush() error {
	if s.err != nil {
		return s.err
	}
	if f, ok := s.w.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}
