package testutil

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TB struct {
	t            testing.TB
	cleanupFuncs []func()
	errors       []string
	mu           sync.Mutex
}

func NewTB(t testing.TB) *TB {
	tb := &TB{
		t: t,
	}
	t.Cleanup(tb.RunCleanups)
	return tb
}

func (tb *TB) Cleanup(f func()) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.cleanupFuncs = append(tb.cleanupFuncs, f)
}

func (tb *TB) Errorf(format string, args ...any) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.errors = append(tb.errors, fmt.Sprintf(format, args...))
}

// RunCleanups runs the registered cleanup functions in LIFO order.
func (tb *TB) RunCleanups() {
	tb.mu.Lock()
	funcs := make([]func(), len(tb.cleanupFuncs))
	copy(funcs, tb.cleanupFuncs)
	slices.Reverse(funcs)
	tb.cleanupFuncs = tb.cleanupFuncs[:0]
	tb.mu.Unlock()

	for _, f := range funcs {
		f()
	}
}

func (tb *TB) error() error {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	var err error
	for _, s := range tb.errors {
		err = errors.Join(err, errors.New(s))
	}
	return err
}

// AssertNoErrors asserts that no errors were recorded.
func (tb *TB) AssertNoErrors() bool {
	return assert.NoError(tb.t, tb.error())
}

// AssertErrors asserts that errors were recorded.
func (tb *TB) AssertErrors() bool {
	return assert.Error(tb.t, tb.error())
}
