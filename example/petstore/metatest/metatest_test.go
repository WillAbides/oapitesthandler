package metatest

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/willabides/oapitesthandler/example/petstore/internal/oapi"
	"github.com/willabides/oapitesthandler/example/petstore/internal/petstoretest"
	"github.com/willabides/oapitesthandler/internal/testutil"
)

// TestGeneratedHandler tests various aspects of the generated TestHandler code
// to ensure the code generator produces correct and functional handlers.
func TestGeneratedHandler(t *testing.T) {
	t.Run("multiple calls with same key", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		client, err := oapi.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		pet := &petstoretest.Pet{
			Id:     ptr(int32(1)),
			Name:   "Marco",
			Status: ptr(petstoretest.PetStatusSold),
		}

		// Expect the same request twice
		handler.ExpectGetPetById(1).RespondJSON200(petstoretest.GetPetById200JSONResponse(*pet))
		handler.ExpectGetPetById(1).RespondJSON200(petstoretest.GetPetById200JSONResponse(*pet))

		// Make two calls with the same parameters
		resp1, err := client.GetPetByIdWithResponse(context.Background(), 1)
		require.NoError(t, err)
		require.Equal(t, 200, resp1.StatusCode())
		require.NotNil(t, resp1.JSON200)

		resp2, err := client.GetPetByIdWithResponse(context.Background(), 1)
		require.NoError(t, err)
		require.Equal(t, 200, resp2.StatusCode())
		require.NotNil(t, resp2.JSON200)
	})

	t.Run("multiple calls with different keys", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		client, err := oapi.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		marco := &petstoretest.Pet{
			Id:     ptr(int32(1)),
			Name:   "Marco",
			Status: ptr(petstoretest.PetStatusSold),
		}

		dolly := &petstoretest.Pet{
			Id:   ptr(int32(2)),
			Name: "Dolly",
			Category: &petstoretest.Category{
				Name: ptr("Puppy"),
			},
		}

		handler.ExpectGetPetById(1).RespondJSON200(petstoretest.GetPetById200JSONResponse(*marco))
		handler.ExpectGetPetById(2).RespondJSON200(petstoretest.GetPetById200JSONResponse(*dolly))

		resp1, err := client.GetPetByIdWithResponse(context.Background(), 1)
		require.NoError(t, err)
		require.Equal(t, 200, resp1.StatusCode())
		require.Equal(t, marco.Name, resp1.JSON200.Name)

		resp2, err := client.GetPetByIdWithResponse(context.Background(), 2)
		require.NoError(t, err)
		require.Equal(t, 200, resp2.StatusCode())
		require.Equal(t, dolly.Name, resp2.JSON200.Name)
	})

	t.Run("custom types - uuid and time", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		client, err := oapi.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		registrationID := uuid.New()
		createdAt := time.Now().Add(-24 * time.Hour).UTC().Truncate(time.Second)
		updatedAt := time.Now().UTC().Truncate(time.Second)

		petWithCustomTypes := petstoretest.PetWithCustomTypes{
			Id:             ptr(int64(1)),
			Name:           ptr("Fluffy"),
			RegistrationId: &registrationID,
			CreatedAt:      &createdAt,
			UpdatedAt:      &updatedAt,
		}

		handler.ExpectGetPetRegistration(1).RespondJSON200(petstoretest.GetPetRegistration200JSONResponse(petWithCustomTypes))

		resp, err := client.GetPetRegistrationWithResponse(context.Background(), 1)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		require.Equal(t, registrationID, *resp.JSON200.RegistrationId)
		require.Equal(t, createdAt, *resp.JSON200.CreatedAt)
		require.Equal(t, updatedAt, *resp.JSON200.UpdatedAt)
	})

	t.Run("additionalProperties map types", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		client, err := oapi.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		metadata := map[string]string{
			"breed":  "Golden Retriever",
			"color":  "golden",
			"weight": "30kg",
		}

		handler.ExpectGetPetMetadata(1, nil).RespondJSON200(petstoretest.GetPetMetadata200JSONResponse(metadata))

		resp, err := client.GetPetMetadataWithResponse(context.Background(), 1, nil)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode())
		require.Equal(t, &metadata, resp.JSON200)
	})

	t.Run("multiple operations on same handler", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		client, err := oapi.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		marco := &petstoretest.Pet{
			Id:     ptr(int32(1)),
			Name:   "Marco",
			Status: ptr(petstoretest.PetStatusSold),
		}

		dolly := &petstoretest.Pet{
			Id:   ptr(int32(2)),
			Name: "Dolly",
			Category: &petstoretest.Category{
				Name: ptr("Puppy"),
			},
		}

		// Test multiple different operations on the same handler
		handler.ExpectGetPetById(1).RespondJSON200(petstoretest.GetPetById200JSONResponse(*marco))

		handler.ExpectFindPetsByStatus(&petstoretest.FindPetsByStatusParams{
			Status: petstoretest.FindPetsByStatusParamsStatusAvailable,
		}).RespondJSON200(petstoretest.FindPetsByStatus200JSONResponse([]petstoretest.Pet{*dolly}))

		// Call GetPetById
		resp1, err := client.GetPetByIdWithResponse(context.Background(), 1)
		require.NoError(t, err)
		require.Equal(t, 200, resp1.StatusCode())
		require.NotNil(t, resp1.JSON200)
		require.Equal(t, *marco.Id, *resp1.JSON200.Id)
		require.Equal(t, marco.Name, resp1.JSON200.Name)

		// Call FindPetsByStatus
		resp2, err := client.FindPetsByStatusWithResponse(
			context.Background(),
			&oapi.FindPetsByStatusParams{
				Status: oapi.FindPetsByStatusParamsStatusAvailable,
			},
		)
		require.NoError(t, err)
		require.Equal(t, 200, resp2.StatusCode())
		require.Len(t, *resp2.JSON200, 1)
	})

	t.Run("Times option", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		client, err := oapi.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		pet := &petstoretest.Pet{
			Id:     ptr(int32(1)),
			Name:   "Marco",
			Status: ptr(petstoretest.PetStatusSold),
		}

		// Expect the same request 3 times
		handler.ExpectGetPetById(1, petstoretest.Times(3)).RespondJSON200(petstoretest.GetPetById200JSONResponse(*pet))

		// Make three calls
		for range 3 {
			resp, callErr := client.GetPetByIdWithResponse(context.Background(), 1)
			require.NoError(t, callErr)
			require.Equal(t, 200, resp.StatusCode())
		}
	})

	t.Run("WithBody method - generic body with []byte", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		client, err := oapi.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		// XML body content
		xmlBody := []byte(`<Pet><id>1</id><name>Fluffy</name><status>available</status></Pet>`)

		// Set expectation using WithBody method
		handler.ExpectAddPetWithBody("application/xml", xmlBody).RespondJSON201(petstoretest.AddPet201JSONResponse{
			Id:     ptr(int32(1)),
			Name:   "Fluffy",
			Status: ptr(petstoretest.PetStatusAvailable),
		})

		// Make request with XML body
		resp, err := client.AddPetWithBodyWithResponse(
			context.Background(),
			"application/xml",
			bytes.NewReader(xmlBody),
		)
		require.NoError(t, err)
		require.Equal(t, 201, resp.StatusCode())
		require.NotNil(t, resp.JSON201)
		require.Equal(t, "Fluffy", resp.JSON201.Name)
	})

	t.Run("WithBody vs typed body methods", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		client, err := oapi.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		// Set up expectation using typed method
		handler.ExpectUpdateUser("john", petstoretest.UpdateUserJSONRequestBody{
			Id:       ptr(int64(1)),
			Username: ptr("john"),
		}).Respond200()

		// Set up expectation using WithBody method
		xmlBody := []byte(`<User><id>1</id><username>john</username></User>`)
		handler.ExpectUpdateUserWithBody("john", "application/xml", xmlBody).Respond200()

		// Make request using typed JSON method
		resp1, err := client.UpdateUserWithResponse(
			context.Background(),
			"john",
			oapi.UpdateUserJSONRequestBody{
				Id:       ptr(int64(1)),
				Username: ptr("john"),
			},
		)
		require.NoError(t, err)
		require.Equal(t, 200, resp1.StatusCode())

		// Make request using generic WithBody method
		resp2, err := client.UpdateUserWithBodyWithResponse(
			context.Background(),
			"john",
			"application/xml",
			bytes.NewReader(xmlBody),
		)
		require.NoError(t, err)
		require.Equal(t, 200, resp2.StatusCode())
	})
}

// TestFailureScenarios tests that the generated handler properly reports errors
// when expectations are not met or requests don't match expectations.
func TestFailureScenarios(t *testing.T) {
	t.Run("unmet expectation", func(t *testing.T) {
		// Use testutil.TB to capture errors instead of failing immediately
		tb := testutil.NewTB(t)
		handler := petstoretest.NewTestHandler(tb)
		server := httptest.NewServer(handler)
		defer server.Close()

		// Set an expectation but never make the call
		handler.ExpectGetPetById(1).RespondJSON200(petstoretest.GetPetById200JSONResponse{
			Id:   ptr(int32(1)),
			Name: "Marco",
		})

		// Run cleanups which should report the unmet expectation
		tb.RunCleanups()

		// Verify that an error was reported
		tb.AssertErrors()
	})

	t.Run("unexpected request", func(t *testing.T) {
		tb := testutil.NewTB(t)
		handler := petstoretest.NewTestHandler(tb)
		server := httptest.NewServer(handler)
		defer server.Close()

		client, err := oapi.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		// Make a request without setting any expectation
		_, err = client.GetPetByIdWithResponse(context.Background(), 1)
		require.NoError(t, err)

		// Verify that an error was reported about no matching expectation
		tb.AssertErrors()
	})

	t.Run("too many calls", func(t *testing.T) {
		tb := testutil.NewTB(t)
		handler := petstoretest.NewTestHandler(tb)
		server := httptest.NewServer(handler)
		defer server.Close()

		client, err := oapi.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		handler.ExpectGetPetById(1, petstoretest.Times(1)).RespondJSON200(petstoretest.GetPetById200JSONResponse{
			Id:   ptr(int32(1)),
			Name: "Marco",
		})

		// Make the expected call
		resp, err := client.GetPetByIdWithResponse(context.Background(), 1)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode())

		// Make an additional unexpected call
		_, err = client.GetPetByIdWithResponse(context.Background(), 1)
		require.NoError(t, err)

		// Verify that an error was reported about no matching expectation
		tb.AssertErrors()
	})

	t.Run("partial expectations met", func(t *testing.T) {
		tb := testutil.NewTB(t)
		handler := petstoretest.NewTestHandler(tb)
		server := httptest.NewServer(handler)
		defer server.Close()

		client, err := oapi.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		// Set expectation for 3 calls
		handler.ExpectGetPetById(1, petstoretest.Times(3)).RespondJSON200(petstoretest.GetPetById200JSONResponse{
			Id:   ptr(int32(1)),
			Name: "Marco",
		})

		// Only make 2 calls
		for range 2 {
			resp, callErr := client.GetPetByIdWithResponse(context.Background(), 1)
			require.NoError(t, callErr)
			require.Equal(t, 200, resp.StatusCode())
		}

		// Run cleanups which should report the unmet expectation
		tb.RunCleanups()

		// Verify that an error was reported
		tb.AssertErrors()
	})

	t.Run("WithError option", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		client, err := oapi.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		// Set an expectation that returns an error
		// When the handler returns an error, the StrictHandler converts it to HTTP 500
		handler.ExpectGetPetById(1).RespondWithError(assert.AnError)

		// The HTTP request succeeds but gets a 500 Internal Server Error status
		resp, err := client.GetPetByIdWithResponse(context.Background(), 1)
		require.NoError(t, err)
		require.Equal(t, 500, resp.StatusCode())
	})
}

func TestHandleMethod(t *testing.T) {
	t.Run("basic handler invocation", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		client, err := oapi.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		handler.ExpectGetPetById(1).Handle(func(req petstoretest.GetPetByIdRequestObject, w http.ResponseWriter) error {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, writeErr := w.Write([]byte(`{"id":1,"name":"Dynamic Pet"}`))
			return writeErr
		})

		resp, err := client.GetPetByIdWithResponse(context.Background(), 1)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		require.Equal(t, "Dynamic Pet", resp.JSON200.Name)
	})

	t.Run("handler with Times option", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		client, err := oapi.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		callCount := 0
		handler.ExpectGetPetById(1, petstoretest.Times(3)).Handle(func(
			req petstoretest.GetPetByIdRequestObject,
			w http.ResponseWriter,
		) error {
			callCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, writeErr := w.Write([]byte(`{"id":1,"name":"Pet ` + string(rune('0'+callCount)) + `"}`))
			return writeErr
		})

		for i := 1; i <= 3; i++ {
			resp, callErr := client.GetPetByIdWithResponse(context.Background(), 1)
			require.NoError(t, callErr)
			require.Equal(t, 200, resp.StatusCode())
		}

		require.Equal(t, 3, callCount)
	})

	t.Run("handler returning different responses based on request", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		client, err := oapi.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		// Set up handler that returns different status codes based on pet ID
		handler.ExpectGetPetById(1).Handle(func(req petstoretest.GetPetByIdRequestObject, w http.ResponseWriter) error {
			if req.PetId == 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				_, writeErr := w.Write([]byte(`{"id":1,"name":"Found Pet"}`))
				return writeErr
			}
			w.WriteHeader(404)
			return nil
		})

		handler.ExpectGetPetById(999).Handle(func(
			req petstoretest.GetPetByIdRequestObject,
			w http.ResponseWriter,
		) error {
			w.WriteHeader(404)
			return nil
		})

		// Call with ID 1 should succeed
		resp1, err := client.GetPetByIdWithResponse(context.Background(), 1)
		require.NoError(t, err)
		require.Equal(t, 200, resp1.StatusCode())
		require.NotNil(t, resp1.JSON200)

		// Call with ID 999 should return 404
		resp2, err := client.GetPetByIdWithResponse(context.Background(), 999)
		require.NoError(t, err)
		require.Equal(t, 404, resp2.StatusCode())
	})

	t.Run("handler returning errors", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		client, err := oapi.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		// Handler that returns an error
		handler.ExpectGetPetById(1).Handle(func(req petstoretest.GetPetByIdRequestObject, w http.ResponseWriter) error {
			return assert.AnError
		})

		// The error should result in HTTP 500
		resp, err := client.GetPetByIdWithResponse(context.Background(), 1)
		require.NoError(t, err)
		require.Equal(t, 500, resp.StatusCode())
	})

	t.Run("handler with request body", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		client, err := oapi.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		// Use Handle with an operation that has a request body
		handler.ExpectUpdateUser("john", petstoretest.UpdateUserJSONRequestBody{
			Id:       ptr(int64(1)),
			Username: ptr("john"),
		}).Handle(func(req petstoretest.UpdateUserRequestObject, w http.ResponseWriter) error {
			// Verify the request contains the expected data
			require.NotNil(t, req.JSONBody)
			require.Equal(t, int64(1), *req.JSONBody.Id)
			require.Equal(t, "john", *req.JSONBody.Username)
			require.Equal(t, "john", req.Username)

			w.WriteHeader(200)
			return nil
		})

		resp, err := client.UpdateUserWithResponse(
			context.Background(),
			"john",
			oapi.UpdateUserJSONRequestBody{
				Id:       ptr(int64(1)),
				Username: ptr("john"),
			},
		)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode())
	})

	t.Run("handler with generic Body io.Reader from WithBody", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		client, err := oapi.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		// XML body content
		xmlBody := []byte(`<Pet><id>42</id><name>XML Pet</name><status>available</status></Pet>`)

		// Use Handle with WithBody method - this populates the Body io.Reader field
		handler.ExpectAddPetWithBody("application/xml", xmlBody).Handle(func(
			req petstoretest.AddPetRequestObject,
			w http.ResponseWriter,
		) error {
			// Verify that the Body field is populated
			require.NotNil(t, req.Body, "Body should be populated from WithBody")

			// Read the body to verify content
			bodyBytes := bytes.NewBuffer(nil)
			if req.Body != nil {
				_, readErr := bodyBytes.ReadFrom(req.Body)
				require.NoError(t, readErr)
			}

			// Verify the body contains expected XML
			require.Contains(t, bodyBytes.String(), "XML Pet")
			require.Contains(t, bodyBytes.String(), "<id>42</id>")

			// Return a custom response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			_, writeErr := w.Write([]byte(`{"id":42,"name":"XML Pet Created"}`))
			return writeErr
		})

		// Make request with XML body
		resp, err := client.AddPetWithBodyWithResponse(
			context.Background(),
			"application/xml",
			bytes.NewReader(xmlBody),
		)
		require.NoError(t, err)
		require.Equal(t, 201, resp.StatusCode())
		require.NotNil(t, resp.JSON201)
		require.Equal(t, "XML Pet Created", resp.JSON201.Name)
	})
}

func ptr[T any](v T) *T {
	return &v
}
