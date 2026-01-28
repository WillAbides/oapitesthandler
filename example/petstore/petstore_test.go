package petstore

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/willabides/oapitesthandler/example/petstore/internal/oapi"
	"github.com/willabides/oapitesthandler/example/petstore/internal/petstoretest"
)

// Example tests demonstrating how to use TestHandler to test a service that uses an oapi-codegen client.
// The TestHandler acts as a mock HTTP server, allowing you to set expectations and verify behavior.
func TestPetStoreService(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		// Create TestHandler which acts as mock HTTP server
		handler := petstoretest.NewTestHandler(t)

		// Create a test server with the handler
		server := httptest.NewServer(handler)
		defer server.Close()

		// Create the service we want to test
		store := newTestStore(t, server.URL)

		// Set expectation on the mock server
		handler.ExpectGetPetById(1).RespondJSON200(petstoretest.GetPetById200JSONResponse{
			Id:     ptr(int32(1)),
			Name:   "Marco",
			Status: ptr(petstoretest.PetStatusSold),
		})

		// Call the service method - it will make HTTP request to our mock server
		got, found, err := store.getPetByID(t.Context(), 1)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, &pet{
			ID:     1,
			Name:   "Marco",
			Status: petStatusSold,
		}, got)
	})

	t.Run("not found response", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		store := newTestStore(t, server.URL)

		handler.ExpectGetPetById(999).RespondJSON404(petstoretest.GetPetById404JSONResponse{
			Message: ptr("Pet not found"),
		})

		got, found, err := store.getPetByID(t.Context(), 999)
		require.NoError(t, err)
		require.False(t, found)
		require.Nil(t, got)
	})

	t.Run("error response", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		store := newTestStore(t, server.URL)

		handler.ExpectGetPetById(3).RespondJSON400(petstoretest.GetPetById400JSONResponse{
			ErrorJSONResponse: petstoretest.ErrorJSONResponse{
				Message: ptr("Bad request"),
			},
		})

		got, found, err := store.getPetByID(t.Context(), 3)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected status code: 400")
		require.False(t, found)
		require.Nil(t, got)
	})

	t.Run("multiple expectations", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		store := newTestStore(t, server.URL)

		// Set up expectations for different pets
		handler.ExpectGetPetById(1).RespondJSON200(petstoretest.GetPetById200JSONResponse{
			Id:     ptr(int32(1)),
			Name:   "Marco",
			Status: ptr(petstoretest.PetStatusSold),
		})

		handler.ExpectGetPetById(2).RespondJSON200(petstoretest.GetPetById200JSONResponse{
			Id:   ptr(int32(2)),
			Name: "Dolly",
			Category: &petstoretest.Category{
				Name: ptr("Puppy"),
			},
		})

		handler.ExpectGetPetById(3).RespondJSON404(petstoretest.GetPetById404JSONResponse{
			Message: ptr("Not found"),
		})

		// Call method that makes multiple requests
		got, err := store.getPetsByIDs(t.Context(), 1, 2, 3)
		require.NoError(t, err)
		require.Len(t, got, 2) // Only 2 pets found (3 returned 404)
		require.Equal(t, int64(1), got[0].ID)
		require.Equal(t, "Marco", got[0].Name)
		require.Equal(t, int64(2), got[1].ID)
		require.Equal(t, "Dolly", got[1].Name)
	})

	t.Run("update pet status", func(t *testing.T) {
		handler := petstoretest.NewTestHandler(t)
		server := httptest.NewServer(handler)
		defer server.Close()

		store := newTestStore(t, server.URL)

		handler.ExpectUpdatePet(petstoretest.UpdatePetJSONRequestBody{
			Id:     ptr(int32(1)),
			Status: ptr(petstoretest.PetStatusSold),
		}).RespondJSON200(petstoretest.UpdatePet200JSONResponse{
			Id:     ptr(int32(1)),
			Name:   "Marco",
			Status: ptr(petstoretest.PetStatusSold),
		})

		err := store.updatePetStatus(t.Context(), 1, petStatusSold)
		require.NoError(t, err)
	})
}

func newTestStore(t *testing.T, serverURL string) *petStore {
	t.Helper()
	client, err := oapi.NewClientWithResponses(serverURL)
	require.NoError(t, err)
	return &petStore{client: client}
}
