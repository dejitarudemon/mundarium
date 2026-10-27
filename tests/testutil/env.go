// Package testutil provides shared helpers for integration tests under
// tests/executor and tests/invoke.
package testutil

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Env keys shared by LLM Studio Invoke integration tests.
const (
	EnvURL        = "TEST_LLM_STUDIO_URL"
	EnvModel      = "TEST_LLM_STUDIO_MODEL"
	EnvAPIKey     = "TEST_LLM_STUDIO_API_KEY"
	EnvMaxIters   = "TEST_LLM_STUDIO_MAX_ITERS"
	EnvTimeoutSec = "TEST_LLM_STUDIO_TIMEOUT_SEC"
)

type systemPromptFile struct {
	Prompt string `json:"prompt"`
}

// RepoRoot returns the repository root (parent of tests/).
func RepoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	// tests/testutil -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// LoadDotEnvFromRepo loads KEY=VALUE pairs from <repo>/.env into the process
// environment without overriding already-set variables.
func LoadDotEnvFromRepo() error {
	return LoadDotEnv(filepath.Join(RepoRoot(), ".env"))
}

// LoadDotEnv loads KEY=VALUE pairs from path.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
	return scanner.Err()
}

// LoadSystemPrompt reads a JSON file with a "prompt" field.
func LoadSystemPrompt(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var file systemPromptFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return "", err
	}
	if strings.TrimSpace(file.Prompt) == "" {
		return "", fmt.Errorf("%s: empty prompt", path)
	}
	return file.Prompt, nil
}

// EnvInt reads a positive int from the environment or returns fallback.
func EnvInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// RequireLLMStudioEnv loads .env and returns url, model, apiKey, maxIters.
// ok is false when the suite should be skipped (missing .env or required keys).
func RequireLLMStudioEnv() (url, model, apiKey string, maxIters int, ok bool, reason error) {
	if err := LoadDotEnvFromRepo(); err != nil {
		return "", "", "", 0, false, err
	}
	url = strings.TrimSpace(os.Getenv(EnvURL))
	model = strings.TrimSpace(os.Getenv(EnvModel))
	if url == "" || model == "" {
		return "", "", "", 0, false, fmt.Errorf("%s and %s are required in .env", EnvURL, EnvModel)
	}
	return url, model, os.Getenv(EnvAPIKey), EnvInt(EnvMaxIters, 8), true, nil
}
