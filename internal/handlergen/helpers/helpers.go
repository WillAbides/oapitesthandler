package helpers

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"slices"
	"sync"
)

type expectOpts struct {
	error   error
	times   int
	atLeast bool
}

type ExpectOption func(*expectOpts)

// Times sets the number of times the expectation should be met.
// Panics if n is negative.
func Times(n int) ExpectOption {
	if n < 0 {
		panic("Times: n must be non-negative")
	}
	return func(o *expectOpts) {
		o.times = n
	}
}

// MinTimes sets the minimum number of times the expectation should be met.
// When n is 0, the expectation acts as a stub and can be called any number of times.
// Panics if n is negative.
func MinTimes(n int) ExpectOption {
	if n < 0 {
		panic("MinTimes: n must be non-negative")
	}
	return func(o *expectOpts) {
		o.times = n
		o.atLeast = true
	}
}

// WithError sets an error to be returned from the strict handler instead of the response.
// When using WithError, set the response to the zero value because it will be ignored anyway.
func WithError(err error) ExpectOption {
	return func(o *expectOpts) {
		o.error = err
	}
}

func keyHash(req any, rawRequestBody []byte) string {
	// json marshal/unmarshal errors are programming errors, so we can panic on them
	// specifically it means our assumption about the request type is wrong

	reqBytes, err := json.Marshal(req)
	if err != nil {
		panic(err)
	}
	var mp map[string]any
	err = json.Unmarshal(reqBytes, &mp)
	if err != nil {
		panic(err)
	}
	delete(mp, "Body")

	h := fnv.New128()
	err = json.NewEncoder(h).Encode(map[string]any{
		"request":        mp,
		"rawRequestBody": rawRequestBody,
	})
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

type expectResponse[REQ, RESP any] struct {
	error    error
	request  REQ
	response RESP

	keyHash string
	times   int
	atLeast bool
}

type expectResponses[REQ, RESP any] struct {
	expectations []*expectResponse[REQ, RESP]
	lock         sync.Mutex
}

type TB interface {
	Cleanup(func())
	Errorf(format string, args ...any)
}

func (e *expectResponses[REQ, RESP]) expect(t TB, req REQ, rawRequestBody io.Reader, resp RESP, opts ...ExpectOption) {
	e.lock.Lock()
	defer e.lock.Unlock()

	options := expectOpts{times: 1}
	for _, o := range opts {
		o(&options)
	}

	var bodyBytes []byte
	if rawRequestBody != nil {
		var err error
		bodyBytes, err = io.ReadAll(rawRequestBody)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}
	}

	ex := &expectResponse[REQ, RESP]{
		request:  req,
		response: resp,
		error:    options.error,
		times:    options.times,
		atLeast:  options.atLeast,
		keyHash:  keyHash(req, bodyBytes),
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

func (e *expectResponses[REQ, RESP]) getResponse(t TB, req REQ, rawRequestBody io.Reader) (RESP, error) {
	e.lock.Lock()
	defer e.lock.Unlock()

	var zeroResp RESP

	bodyBytes, err := func() ([]byte, error) {
		if rawRequestBody == nil {
			return nil, nil
		}
		return io.ReadAll(rawRequestBody)
	}()
	if err != nil {
		return zeroResp, fmt.Errorf("reading request body: %w", err)
	}

	key := keyHash(req, bodyBytes)
	idx := slices.IndexFunc(e.expectations, func(e *expectResponse[REQ, RESP]) bool {
		return (e.atLeast || e.times > 0) && e.keyHash == key
	})
	if idx == -1 {
		t.Errorf("no expectation found for request %+v", req)
		return zeroResp, fmt.Errorf("no expectation found for request")
	}

	ex := e.expectations[idx]
	if ex.times > 0 {
		ex.times--
	}
	return ex.response, ex.error
}
