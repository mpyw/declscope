package measure

import (
	"errors"
	"testing"
)

// sinkTestWriter fails after the given number of writes, so that a failure
// partway through a report can be observed.
type sinkTestWriter struct {
	ok  int
	err error
}

func (w *sinkTestWriter) Write(p []byte) (int, error) {
	if w.ok == 0 {
		return 0, w.err
	}
	w.ok--
	return len(p), nil
}

// TestSinkKeepsTheFirstError checks what a renderer reports when a write
// fails partway: the first failure, not the last, and nothing written after
// it. A report is dozens of writes, and returning nil however they went is
// what this replaced.
func TestSinkKeepsTheFirstError(t *testing.T) {
	first := errors.New("disk full")
	w := &sinkTestWriter{ok: 1, err: first}
	out := newSink(w)

	out.print("one")
	out.printf("%s", "two")
	out.print("three")

	if got := out.flush(); !errors.Is(got, first) {
		t.Errorf("flush returned %v, want the first write error", got)
	}
	if w.ok != 0 {
		t.Error("the sink kept writing after a failure")
	}
}

// TestSinkFlushesTheWriterItWraps checks that a buffered writer is flushed,
// since every text report is rendered through a tabwriter.
func TestSinkFlushesTheWriterItWraps(t *testing.T) {
	flushed := false
	out := newSink(&sinkTestFlusher{flushed: &flushed})
	if err := out.flush(); err != nil {
		t.Fatal(err)
	}
	if !flushed {
		t.Error("the wrapped writer was not flushed")
	}
}

type sinkTestFlusher struct{ flushed *bool }

func (*sinkTestFlusher) Write(p []byte) (int, error) { return len(p), nil }
func (f *sinkTestFlusher) Flush() error              { *f.flushed = true; return nil }
