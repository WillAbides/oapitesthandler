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

func Times(n int) ExpectOption {
	return func(o *expectOpts) {
		o.times = n
	}
}

func WithError(err error) ExpectOption {
	return func(o *expectOpts) {
		o.error = err
	}
}

// keyHash creates a hash of a request object, skipping io.Reader fields
// and using typed body fields (JSONBody, FormdataBody, etc.) instead.
// This is necessary because RequestObjects contain io.Reader fields that
// are already consumed by the time we receive the RequestObject.
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

type handleFunc[RESP any] func(found bool, resp RESP, respErr error)

// Handle looks for a matching expectation for the given request.
// If found, it decrements the times counter and calls handle with
// the response and error. If not found, it calls handle with found=false.
func (e *expectations[REQ, RESP]) Handle(req REQ, handle handleFunc[RESP]) {
	e.lock.Lock()
	defer e.lock.Unlock()

	key := keyHash(req)
	idx := slices.IndexFunc(e.expectations, func(e *expectation[REQ, RESP]) bool {
		return e.times > 0 && e.keyHash == key
	})
	if idx == -1 {
		var zeroResp RESP
		handle(false, zeroResp, nil)
		return
	}
	ex := e.expectations[idx]
	ex.times--
	handle(true, ex.response, ex.error)
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
