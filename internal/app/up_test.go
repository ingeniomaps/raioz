package app

import (
	"context"
	"fmt"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/mocks"
)

func TestNewUpUseCase(t *testing.T) {
	deps := newFullMockDeps()
	// Need proxy and discovery managers since upcase requires them
	deps.ProxyManager = nil
	deps.DiscoveryManager = nil
	uc := NewUpUseCase(deps)
	if uc == nil {
		t.Fatal("expected non-nil UpUseCase")
	}
	if uc.useCase == nil {
		t.Error("expected non-nil inner useCase")
	}
}

func TestUpUseCase_stopOtherProjects_NoState(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.StateManager = &mocks.MockStateManager{
		LoadGlobalStateFunc: func() (*models.GlobalState, error) {
			return nil, fmt.Errorf("no state")
		},
	}
	uc := NewUpUseCase(deps)
	// Should not panic and just return silently
	uc.stopOtherProjects(context.Background(), "")
}

func TestUpUseCase_stopOtherProjects_NoActiveProjects(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.StateManager = &mocks.MockStateManager{
		LoadGlobalStateFunc: func() (*models.GlobalState, error) {
			return &models.GlobalState{ActiveProjects: []string{}}, nil
		},
	}
	uc := NewUpUseCase(deps)
	uc.stopOtherProjects(context.Background(), "")
}
