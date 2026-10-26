package openai_test

import (
	"context"
	"encoding/json"
	"errors"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	mapjournal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution/implemented/map"
	provimpl "github.com/dejitarudemon/mundarium/internal/providers/implemented"
	"github.com/dejitarudemon/mundarium/internal/providers/implemented/openai"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
)

type testTool struct{}

func (testTool) Definition() definition.Definition {
	return definition.Definition{
		Name:        "create_location",
		Description: "create",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
	}
}

func (testTool) IsIdempotent() bool { return false }

func (testTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	return nil, errors.New("not used")
}

func newTestExecutor(t *testing.T) *executor.Executor {
	t.Helper()
	ex, err := executor.NewExecutor(mapjournal.NewMapJournal(), nil)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	if err := ex.Register(testTool{}, []types.Role{types.Role_Master}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return ex
}

func TestNewOpenAICompatibleProvider_Validation(t *testing.T) {
	if _, err := openai.NewOpenAICompatibleProvider(nil, "http://x", "m", "k", "p", types.Role_Master, 1); !errors.Is(err, provimpl.ErrNoExecutor) {
		t.Fatalf("want ErrNoExecutor, got %v", err)
	}

	ex, err := executor.NewExecutor(mapjournal.NewMapJournal(), nil)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	if _, err := openai.NewOpenAICompatibleProvider(ex, "http://x", "m", "k", "p", types.Role_Master, 1); !errors.Is(err, provimpl.ErrNoTools) {
		t.Fatalf("want ErrNoTools, got %v", err)
	}
}

func TestOpenAICompatibleProvider_Available(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("Authorization: %q", got)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-test","object":"model"}]}`))
	}))
	defer srv.Close()

	p, err := openai.NewOpenAICompatibleProvider(newTestExecutor(t), srv.URL, "gpt-test", "secret", "hi", types.Role_Master, 1)
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider: %v", err)
	}
	if err := p.Available(context.Background()); err != nil {
		t.Fatalf("Available: %v", err)
	}
}

func TestOpenAICompatibleProvider_Available_NoModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"other","object":"model"}]}`))
	}))
	defer srv.Close()

	p, err := openai.NewOpenAICompatibleProvider(newTestExecutor(t), srv.URL, "gpt-test", "secret", "hi", types.Role_Master, 1)
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider: %v", err)
	}
	err = p.Available(context.Background())
	var noModel provimpl.NoModelError
	if !errors.As(err, &noModel) {
		t.Fatalf("want NoModelError, got %v", err)
	}
}

func TestOpenAICompatibleProvider_Available_NotOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	p, err := openai.NewOpenAICompatibleProvider(newTestExecutor(t), srv.URL, "gpt-test", "secret", "hi", types.Role_Master, 1)
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider: %v", err)
	}
	err = p.Available(context.Background())
	var unavailable provimpl.ProviderNotAvailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("want ProviderNotAvailableError, got %v", err)
	}
}
