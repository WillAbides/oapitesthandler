package petstore

import (
	"context"
	"fmt"

	"github.com/willabides/oapitesthandler/example/petstore/internal/oapi"
)

//go:generate go tool oapi-codegen -config ./oapi-codegen.yaml ./openapi.yaml
//go:generate go tool oapitesthandler --config=./oapi-codegen.yaml --out=internal/petstoretest ./openapi.yaml

type petStore struct {
	client oapi.ClientWithResponsesInterface
}

func (s *petStore) getPetByID(ctx context.Context, id int64) (_ *pet, found bool, _ error) {
	resp, err := s.client.GetPetByIdWithResponse(ctx, id)
	if err != nil {
		return nil, false, err
	}
	switch resp.StatusCode() {
	case 200:
	case 404:
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}
	if resp.JSON200 == nil {
		return nil, false, fmt.Errorf("expected pet in response body")
	}
	return petFromOapi(resp.JSON200), true, nil
}

// getPetsByIDs is a classic n+1 problem, but it's an easy way to demonstrate multiple calls to the same method.
func (s *petStore) getPetsByIDs(ctx context.Context, ids ...int64) ([]*pet, error) {
	var pets []*pet
	for _, id := range ids {
		p, found, err := s.getPetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		pets = append(pets, p)
	}
	return pets, nil
}

type pet struct {
	Name     string
	Category string
	ID       int64
	Status   petStatus
}

func petFromOapi(op *oapi.Pet) *pet {
	if op == nil {
		return nil
	}
	return &pet{
		ID:       deref(op.Id),
		Name:     op.Name,
		Status:   petStatusFromOapi(op.Status),
		Category: deref(deref(op.Category).Name),
	}
}

type petStatus int

const (
	petStatusInvalid petStatus = iota
	petStatusAvailable
	petStatusPending
	petStatusSold
	petStatusUnknown
)

func petStatusFromOapi(op *oapi.PetStatus) petStatus {
	if op == nil {
		return petStatusUnknown
	}
	switch *op {
	case oapi.PetStatusAvailable:
		return petStatusAvailable
	case oapi.PetStatusPending:
		return petStatusPending
	case oapi.PetStatusSold:
		return petStatusSold
	default:
		return petStatusInvalid
	}
}

func ptr[T any](v T) *T {
	return &v
}

func deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}
