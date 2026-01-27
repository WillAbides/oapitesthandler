package helpers

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"slices"
	"sync"
)

type expectOpts struct {
	error error
	times int
}

type ExpectOption func(*expectOpts)

// Times sets the number of times the expectation should be met.
func Times(n int) ExpectOption {
	return func(o *expectOpts) {
		o.times = n
	}
}

// WithError sets an error to be returned from the strict handler instead of the response.
// When using WithError, set the response to the zero value because it will be ignored anyway.
func WithError(err error) ExpectOption {
	return func(o *expectOpts) {
		o.error = err
	}
}

func keyHash(req any) string {
	h := fnv.New128()
	err := json.NewEncoder(h).Encode(req)
	if err != nil {
		// if encoding fails, panic as this indicates a programming error, not a runtime error
		// specifically it means our assumption about the request type is wrong
		panic("hash write failed: " + err.Error())
	}
	return hex.EncodeToString(h.Sum(nil))
}

type expectation[REQ, RESP any] struct {
	error    error
	request  REQ
	response RESP

	keyHash string
	times   int
}

type expectations[REQ, RESP any] struct {
	expectations []*expectation[REQ, RESP]
	lock         sync.Mutex
}

type TB interface {
	Cleanup(func())
	Errorf(format string, args ...any)
}

func (e *expectations[REQ, RESP]) Expect(t TB, req REQ, resp RESP, opts ...ExpectOption) {
	e.lock.Lock()
	defer e.lock.Unlock()

	options := expectOpts{times: 1}
	for _, o := range opts {
		o(&options)
	}

	ex := &expectation[REQ, RESP]{
		request:  req,
		response: resp,
		error:    options.error,
		times:    options.times,
		keyHash:  keyHash(req),
	}
	e.expectations = append(e.expectations, ex)

	t.Cleanup(func() {
		e.lock.Lock()
		defer e.lock.Unlock()

		if ex.times > 0 {
			t.Errorf("expectation for request %+v not met, %d calls remaining", req, ex.times)
		}
	})
}

func (e *expectations[REQ, RESP]) GetResponse(t TB, req REQ) (RESP, error) {
	e.lock.Lock()
	defer e.lock.Unlock()

	key := keyHash(req)
	idx := slices.IndexFunc(e.expectations, func(e *expectation[REQ, RESP]) bool {
		return e.times > 0 && e.keyHash == key
	})
	if idx == -1 {
		var zeroResp RESP
		t.Errorf("no expectation found for request %+v", req)
		return zeroResp, fmt.Errorf("no expectation found for request")
	}

	ex := e.expectations[idx]
	ex.times--
	return ex.response, ex.error
}
