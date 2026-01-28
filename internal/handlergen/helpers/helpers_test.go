package helpers

import (
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/willabides/oapitesthandler/internal/testutil"
)

// Test types for request/response
type testRequest struct {
	ID       int
	Name     string
	Body     io.Reader // Should be skipped in hash
	JSONBody *testBody
}

type testBody struct {
	Field1 string
	Field2 int
}

type testResponse struct {
	Status  int
	Message string
}

func TestKeyHash(t *testing.T) {
	t.Run("same request produces same hash", func(t *testing.T) {
		req1 := testRequest{
			ID:       1,
			Name:     "test",
			JSONBody: &testBody{Field1: "value", Field2: 42},
		}
		req2 := testRequest{
			ID:       1,
			Name:     "test",
			JSONBody: &testBody{Field1: "value", Field2: 42},
		}

		hash1 := keyHash(req1, nil)
		hash2 := keyHash(req2, nil)

		assert.Equal(t, hash1, hash2, "expected same hash for identical requests")
	})

	t.Run("different requests produce different hashes", func(t *testing.T) {
		req1 := testRequest{ID: 1, Name: "test"}
		req2 := testRequest{ID: 2, Name: "test"}

		hash1 := keyHash(req1, nil)
		hash2 := keyHash(req2, nil)

		assert.NotEqual(t, hash1, hash2, "expected different hashes for different requests")
	})

	t.Run("io.Reader fields are skipped", func(t *testing.T) {
		req1 := testRequest{
			ID:   1,
			Name: "test",
			Body: strings.NewReader("body content 1"),
		}
		req2 := testRequest{
			ID:   1,
			Name: "test",
			Body: strings.NewReader("body content 2"),
		}

		hash1 := keyHash(req1, nil)
		hash2 := keyHash(req2, nil)

		assert.Equal(t, hash1, hash2, "expected same hash when only io.Reader differs")
	})

	t.Run("nil vs non-nil pointer", func(t *testing.T) {
		req1 := testRequest{ID: 1, Name: "test", JSONBody: nil}
		req2 := testRequest{ID: 1, Name: "test", JSONBody: &testBody{}}

		hash1 := keyHash(req1, nil)
		hash2 := keyHash(req2, nil)

		assert.NotEqual(t, hash1, hash2, "expected different hashes for nil vs non-nil pointer")
	})
}

func TestHashValue(t *testing.T) {
	t.Run("nil values", func(t *testing.T) {
		var nilPtr *testRequest
		hash1 := keyHash(nilPtr, nil)
		hash2 := keyHash(nilPtr, nil)
		assert.Equal(t, hash1, hash2, "expected consistent hash for nil pointer")
	})

	t.Run("slices", func(t *testing.T) {
		type sliceReq struct {
			Values []int
		}
		req1 := sliceReq{Values: []int{1, 2, 3}}
		req2 := sliceReq{Values: []int{1, 2, 3}}
		req3 := sliceReq{Values: []int{1, 2, 4}}

		hash1 := keyHash(req1, nil)
		hash2 := keyHash(req2, nil)
		hash3 := keyHash(req3, nil)

		assert.Equal(t, hash1, hash2, "expected same hash for identical slices")
		assert.NotEqual(t, hash1, hash3, "expected different hash for different slices")
	})

	t.Run("maps", func(t *testing.T) {
		type mapReq struct {
			Values map[string]int
		}
		req1 := mapReq{Values: map[string]int{"a": 1, "b": 2}}
		req2 := mapReq{Values: map[string]int{"b": 2, "a": 1}} // Different order
		req3 := mapReq{Values: map[string]int{"a": 1, "b": 3}}

		hash1 := keyHash(req1, nil)
		hash2 := keyHash(req2, nil)
		hash3 := keyHash(req3, nil)

		assert.Equal(t, hash1, hash2, "expected same hash for maps regardless of insertion order")
		assert.NotEqual(t, hash1, hash3, "expected different hash for different map values")
	})

	t.Run("arrays", func(t *testing.T) {
		type arrayReq struct {
			Values [3]int
		}
		req1 := arrayReq{Values: [3]int{1, 2, 3}}
		req2 := arrayReq{Values: [3]int{1, 2, 3}}
		req3 := arrayReq{Values: [3]int{1, 2, 4}}

		hash1 := keyHash(req1, nil)
		hash2 := keyHash(req2, nil)
		hash3 := keyHash(req3, nil)

		assert.Equal(t, hash1, hash2, "expected same hash for identical arrays")
		assert.NotEqual(t, hash1, hash3, "expected different hash for different arrays")
	})

	t.Run("nested structs", func(t *testing.T) {
		req1 := testRequest{
			ID:       1,
			JSONBody: &testBody{Field1: "nested", Field2: 99},
		}
		req2 := testRequest{
			ID:       1,
			JSONBody: &testBody{Field1: "nested", Field2: 99},
		}
		req3 := testRequest{
			ID:       1,
			JSONBody: &testBody{Field1: "nested", Field2: 100},
		}

		hash1 := keyHash(req1, nil)
		hash2 := keyHash(req2, nil)
		hash3 := keyHash(req3, nil)

		assert.Equal(t, hash1, hash2, "expected same hash for identical nested structs")
		assert.NotEqual(t, hash1, hash3, "expected different hash for different nested structs")
	})
}

func TestExpectations(t *testing.T) {
	t.Run("basic expect and get", func(t *testing.T) {
		tb := testutil.NewTB(t)
		exp := &expectResponses[testRequest, testResponse]{}

		req := testRequest{ID: 1, Name: "test"}
		resp := testResponse{Status: 200, Message: "ok"}

		exp.expect(tb, req, nil, resp)

		got, err := exp.getResponse(tb, req, nil)
		require.NoError(t, err)
		assert.Equal(t, resp.Status, got.Status)
		assert.Equal(t, resp.Message, got.Message)

		// Cleanup should pass since expectation was met
		tb.RunCleanups()
		tb.AssertNoErrors()
	})

	t.Run("unmet expectation triggers error on cleanup", func(t *testing.T) {
		tb := testutil.NewTB(t)
		exp := &expectResponses[testRequest, testResponse]{}

		req := testRequest{ID: 1, Name: "test"}
		resp := testResponse{Status: 200, Message: "ok"}

		exp.expect(tb, req, nil, resp)

		// Don't call getResponse - expectation remains unmet

		tb.RunCleanups()
		tb.AssertErrors()
	})

	t.Run("Times option", func(t *testing.T) {
		tb := testutil.NewTB(t)
		exp := &expectResponses[testRequest, testResponse]{}

		req := testRequest{ID: 1, Name: "test"}
		resp := testResponse{Status: 200, Message: "ok"}

		exp.expect(tb, req, nil, resp, Times(3))

		for i := range 3 {
			got, err := exp.getResponse(tb, req, nil)
			require.NoError(t, err, "call %d", i+1)
			assert.Equal(t, resp.Status, got.Status, "call %d", i+1)
		}

		tb.RunCleanups()
		tb.AssertNoErrors()
	})

	t.Run("Times option exceeded", func(t *testing.T) {
		tb := testutil.NewTB(t)
		exp := &expectResponses[testRequest, testResponse]{}

		req := testRequest{ID: 1, Name: "test"}
		resp := testResponse{Status: 200, Message: "ok"}

		exp.expect(tb, req, nil, resp, Times(2))

		// Call twice successfully
		for i := range 2 {
			_, err := exp.getResponse(tb, req, nil)
			require.NoError(t, err, "call %d", i+1)
		}

		// Third call should fail
		_, err := exp.getResponse(tb, req, nil)
		assert.Error(t, err, "expected error on third call")
		tb.AssertErrors()
	})

	t.Run("WithError option", func(t *testing.T) {
		tb := testutil.NewTB(t)
		exp := &expectResponses[testRequest, testResponse]{}

		req := testRequest{ID: 1, Name: "test"}
		resp := testResponse{Status: 500, Message: "error"}
		expectedErr := errors.New("test error")

		exp.expect(tb, req, nil, resp, WithError(expectedErr))

		got, err := exp.getResponse(tb, req, nil)
		assert.Equal(t, expectedErr, err)
		assert.Equal(t, resp.Status, got.Status)

		tb.RunCleanups()
		tb.AssertNoErrors()
	})

	t.Run("no expectation found", func(t *testing.T) {
		tb := testutil.NewTB(t)
		exp := &expectResponses[testRequest, testResponse]{}

		req := testRequest{ID: 1, Name: "test"}

		_, err := exp.getResponse(tb, req, nil)
		assert.Error(t, err, "expected error when no expectation found")
		tb.AssertErrors()
	})

	t.Run("FIFO matching", func(t *testing.T) {
		tb := testutil.NewTB(t)
		exp := &expectResponses[testRequest, testResponse]{}

		req := testRequest{ID: 1, Name: "test"}
		resp1 := testResponse{Status: 200, Message: "first"}
		resp2 := testResponse{Status: 201, Message: "second"}

		exp.expect(tb, req, nil, resp1)
		exp.expect(tb, req, nil, resp2)

		// First call should return first expectation
		got1, err := exp.getResponse(tb, req, nil)
		require.NoError(t, err)
		assert.Equal(t, "first", got1.Message)

		// Second call should return second expectation
		got2, err := exp.getResponse(tb, req, nil)
		require.NoError(t, err)
		assert.Equal(t, "second", got2.Message)

		tb.RunCleanups()
		tb.AssertNoErrors()
	})

	t.Run("concurrent access", func(t *testing.T) {
		tb := testutil.NewTB(t)
		exp := &expectResponses[testRequest, testResponse]{}

		req := testRequest{ID: 1, Name: "test"}
		resp := testResponse{Status: 200, Message: "ok"}

		exp.expect(tb, req, nil, resp, Times(100))

		var wg sync.WaitGroup
		for range 100 {
			wg.Go(func() {
				_, err := exp.getResponse(tb, req, nil)
				assert.NoError(t, err)
			})
		}
		wg.Wait()

		tb.RunCleanups()
		tb.AssertNoErrors()
	})

	t.Run("with rawRequestBody - same body bytes produce same hash", func(t *testing.T) {
		req := testRequest{ID: 1, Name: "test"}
		bodyBytes := []byte(`{"field": "value"}`)

		hash1 := keyHash(req, bodyBytes)
		hash2 := keyHash(req, bodyBytes)

		assert.Equal(t, hash1, hash2, "expected same hash for same raw body bytes")
	})

	t.Run("with rawRequestBody - different body bytes produce different hashes", func(t *testing.T) {
		req := testRequest{ID: 1, Name: "test"}
		bodyBytes1 := []byte(`{"field": "value1"}`)
		bodyBytes2 := []byte(`{"field": "value2"}`)

		hash1 := keyHash(req, bodyBytes1)
		hash2 := keyHash(req, bodyBytes2)

		assert.NotEqual(t, hash1, hash2, "expected different hashes for different raw body bytes")
	})

	t.Run("with rawRequestBody - expect and get", func(t *testing.T) {
		tb := testutil.NewTB(t)
		exp := &expectResponses[testRequest, testResponse]{}

		req := testRequest{ID: 1, Name: "test", Body: strings.NewReader("body content")}
		resp := testResponse{Status: 200, Message: "ok"}
		bodyBytes := []byte(`{"field": "value"}`)

		// Set expectation with raw body bytes
		exp.expect(tb, req, strings.NewReader(string(bodyBytes)), resp)

		// Get response with same raw body bytes
		got, err := exp.getResponse(tb, req, strings.NewReader(string(bodyBytes)))
		require.NoError(t, err)
		assert.Equal(t, resp.Status, got.Status)
		assert.Equal(t, resp.Message, got.Message)

		tb.RunCleanups()
		tb.AssertNoErrors()
	})

	t.Run("with rawRequestBody - mismatched body bytes", func(t *testing.T) {
		tb := testutil.NewTB(t)
		exp := &expectResponses[testRequest, testResponse]{}

		req := testRequest{ID: 1, Name: "test"}
		resp := testResponse{Status: 200, Message: "ok"}
		bodyBytes1 := []byte(`{"field": "value1"}`)
		bodyBytes2 := []byte(`{"field": "value2"}`)

		// Set expectation with one body
		exp.expect(tb, req, strings.NewReader(string(bodyBytes1)), resp)

		// Try to get with different body - should fail
		_, err := exp.getResponse(tb, req, strings.NewReader(string(bodyBytes2)))
		assert.Error(t, err, "expected error when raw body bytes don't match")
		tb.AssertErrors()
	})

	t.Run("with rawRequestBody - FIFO matching with different bodies", func(t *testing.T) {
		tb := testutil.NewTB(t)
		exp := &expectResponses[testRequest, testResponse]{}

		req := testRequest{ID: 1, Name: "test"}
		bodyBytes1 := []byte(`{"field": "value1"}`)
		bodyBytes2 := []byte(`{"field": "value2"}`)
		resp1 := testResponse{Status: 200, Message: "first"}
		resp2 := testResponse{Status: 201, Message: "second"}

		// Set expectations for different body bytes
		exp.expect(tb, req, strings.NewReader(string(bodyBytes1)), resp1)
		exp.expect(tb, req, strings.NewReader(string(bodyBytes2)), resp2)

		// Get with first body
		got1, err := exp.getResponse(tb, req, strings.NewReader(string(bodyBytes1)))
		require.NoError(t, err)
		assert.Equal(t, "first", got1.Message)

		// Get with second body
		got2, err := exp.getResponse(tb, req, strings.NewReader(string(bodyBytes2)))
		require.NoError(t, err)
		assert.Equal(t, "second", got2.Message)

		tb.RunCleanups()
		tb.AssertNoErrors()
	})

	t.Run("with rawRequestBody - Times option", func(t *testing.T) {
		tb := testutil.NewTB(t)
		exp := &expectResponses[testRequest, testResponse]{}

		req := testRequest{ID: 1, Name: "test"}
		bodyBytes := []byte(`{"field": "value"}`)
		resp := testResponse{Status: 200, Message: "ok"}

		exp.expect(tb, req, strings.NewReader(string(bodyBytes)), resp, Times(3))

		for i := range 3 {
			got, err := exp.getResponse(tb, req, strings.NewReader(string(bodyBytes)))
			require.NoError(t, err, "call %d", i+1)
			assert.Equal(t, resp.Status, got.Status, "call %d", i+1)
		}

		tb.RunCleanups()
		tb.AssertNoErrors()
	})

	t.Run("MinTimes option - exactly n calls", func(t *testing.T) {
		tb := testutil.NewTB(t)
		exp := &expectResponses[testRequest, testResponse]{}

		req := testRequest{ID: 1, Name: "test"}
		resp := testResponse{Status: 200, Message: "ok"}

		exp.expect(tb, req, nil, resp, MinTimes(3))

		for i := range 3 {
			got, err := exp.getResponse(tb, req, nil)
			require.NoError(t, err, "call %d", i+1)
			assert.Equal(t, resp.Status, got.Status, "call %d", i+1)
		}

		tb.RunCleanups()
		tb.AssertNoErrors()
	})

	t.Run("MinTimes option - more than n calls", func(t *testing.T) {
		tb := testutil.NewTB(t)
		exp := &expectResponses[testRequest, testResponse]{}

		req := testRequest{ID: 1, Name: "test"}
		resp := testResponse{Status: 200, Message: "ok"}

		exp.expect(tb, req, nil, resp, MinTimes(3))

		// Call 5 times (more than the minimum of 3)
		for i := range 5 {
			got, err := exp.getResponse(tb, req, nil)
			require.NoError(t, err, "call %d", i+1)
			assert.Equal(t, resp.Status, got.Status, "call %d", i+1)
		}

		tb.RunCleanups()
		tb.AssertNoErrors()
	})

	t.Run("MinTimes option - fewer than n calls triggers error", func(t *testing.T) {
		tb := testutil.NewTB(t)
		exp := &expectResponses[testRequest, testResponse]{}

		req := testRequest{ID: 1, Name: "test"}
		resp := testResponse{Status: 200, Message: "ok"}

		exp.expect(tb, req, nil, resp, MinTimes(3))

		// Only call 2 times (less than the minimum of 3)
		for i := range 2 {
			got, err := exp.getResponse(tb, req, nil)
			require.NoError(t, err, "call %d", i+1)
			assert.Equal(t, resp.Status, got.Status, "call %d", i+1)
		}

		tb.RunCleanups()
		tb.AssertErrors() // Should have error because minimum not met
	})

	t.Run("MinTimes(0) acts as stub - unlimited calls", func(t *testing.T) {
		tb := testutil.NewTB(t)
		exp := &expectResponses[testRequest, testResponse]{}

		req := testRequest{ID: 1, Name: "test"}
		resp := testResponse{Status: 200, Message: "ok"}

		exp.expect(tb, req, nil, resp, MinTimes(0))

		// Call multiple times with no limit
		for i := range 10 {
			got, err := exp.getResponse(tb, req, nil)
			require.NoError(t, err, "call %d", i+1)
			assert.Equal(t, resp.Status, got.Status, "call %d", i+1)
		}

		tb.RunCleanups()
		tb.AssertNoErrors()
	})

	t.Run("MinTimes(0) acts as stub - zero calls", func(t *testing.T) {
		tb := testutil.NewTB(t)
		exp := &expectResponses[testRequest, testResponse]{}

		req := testRequest{ID: 1, Name: "test"}
		resp := testResponse{Status: 200, Message: "ok"}

		exp.expect(tb, req, nil, resp, MinTimes(0))

		// Don't call at all - should still pass cleanup
		tb.RunCleanups()
		tb.AssertNoErrors()
	})

	t.Run("Times panics on negative value", func(t *testing.T) {
		assert.PanicsWithValue(t, "Times: n must be non-negative", func() {
			Times(-1)
		})
	})

	t.Run("MinTimes panics on negative value", func(t *testing.T) {
		assert.PanicsWithValue(t, "MinTimes: n must be non-negative", func() {
			MinTimes(-1)
		})
	})
}
