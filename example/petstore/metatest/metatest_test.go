package metatest

import (
	"context"
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
		handler.ExpectGetPetById(
			petstoretest.GetPetByIdRequestObject{PetId: 1},
			petstoretest.GetPetById200JSONResponse(*pet),
		)
		handler.ExpectGetPetById(
			petstoretest.GetPetByIdRequestObject{PetId: 1},
			petstoretest.GetPetById200JSONResponse(*pet),
		)

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

		handler.ExpectGetPetById(
			petstoretest.GetPetByIdRequestObject{PetId: 1},
			petstoretest.GetPetById200JSONResponse(*marco),
		)
		handler.ExpectGetPetById(
			petstoretest.GetPetByIdRequestObject{PetId: 2},
			petstoretest.GetPetById200JSONResponse(*dolly),
		)

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

		handler.ExpectGetPetRegistration(
			petstoretest.GetPetRegistrationRequestObject{
				PetId: 1,
			},
			petstoretest.GetPetRegistration200JSONResponse(petWithCustomTypes),
		)

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

		handler.ExpectGetPetMetadata(
			petstoretest.GetPetMetadataRequestObject{
				PetId: 1,
			},
			petstoretest.GetPetMetadata200JSONResponse(metadata),
		)

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
		handler.ExpectGetPetById(
			petstoretest.GetPetByIdRequestObject{PetId: 1},
			petstoretest.GetPetById200JSONResponse(*marco),
		)

		handler.ExpectFindPetsByStatus(
			petstoretest.FindPetsByStatusRequestObject{
				Params: petstoretest.FindPetsByStatusParams{
					Status: petstoretest.FindPetsByStatusParamsStatusAvailable,
				},
			},
			petstoretest.FindPetsByStatus200JSONResponse([]petstoretest.Pet{*dolly}),
		)

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
		handler.ExpectGetPetById(
			petstoretest.GetPetByIdRequestObject{PetId: 1},
			petstoretest.GetPetById200JSONResponse(*pet),
			petstoretest.Times(3),
		)

		// Make three calls
		for i := 0; i < 3; i++ {
			resp, callErr := client.GetPetByIdWithResponse(context.Background(), 1)
			require.NoError(t, callErr)
			require.Equal(t, 200, resp.StatusCode())
		}
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
		handler.ExpectGetPetById(
			petstoretest.GetPetByIdRequestObject{PetId: 1},
			petstoretest.GetPetById200JSONResponse{
				Id:   ptr(int32(1)),
				Name: "Marco",
			},
		)

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

		handler.ExpectGetPetById(
			petstoretest.GetPetByIdRequestObject{PetId: 1},
			petstoretest.GetPetById200JSONResponse{
				Id:   ptr(int32(1)),
				Name: "Marco",
			},
			petstoretest.Times(1),
		)

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
		handler.ExpectGetPetById(
			petstoretest.GetPetByIdRequestObject{PetId: 1},
			petstoretest.GetPetById200JSONResponse{
				Id:   ptr(int32(1)),
				Name: "Marco",
			},
			petstoretest.Times(3),
		)

		// Only make 2 calls
		for i := 0; i < 2; i++ {
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
		handler.ExpectGetPetById(
			petstoretest.GetPetByIdRequestObject{PetId: 1},
			petstoretest.GetPetById200JSONResponse{},
			petstoretest.WithError(assert.AnError),
		)

		// The HTTP request succeeds but gets a 500 Internal Server Error status
		resp, err := client.GetPetByIdWithResponse(context.Background(), 1)
		require.NoError(t, err)
		require.Equal(t, 500, resp.StatusCode())
	})
}

func ptr[T any](v T) *T {
	return &v
}
