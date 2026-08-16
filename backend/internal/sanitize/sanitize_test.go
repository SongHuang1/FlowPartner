package sanitize

import (
	"fmt"
	"strings"
	"testing"
)

func TestError_BearerToken(t *testing.T) {
	err := fmt.Errorf("request failed with Authorization: Bearer sk-secret-key")
	sanitized := Error(err)

	if strings.Contains(sanitized, "sk-secret-key") {
		t.Error("sanitized error should not contain the API key")
	}
	if sanitized == err.Error() {
		t.Error("sanitized error should be different from original")
	}
}

func TestError_APIKey(t *testing.T) {
	err := fmt.Errorf("api_key=sk-1234567890abcdef")
	sanitized := Error(err)

	if strings.Contains(sanitized, "sk-1234567890abcdef") {
		t.Error("sanitized error should not contain the API key")
	}
}

func TestError_NoSensitiveData(t *testing.T) {
	err := fmt.Errorf("connection refused")
	sanitized := Error(err)

	if sanitized != "connection refused" {
		t.Errorf("expected 'connection refused', got %q", sanitized)
	}
}

func TestError_OpenAIKeyFormat(t *testing.T) {
	err := fmt.Errorf("authentication failed for sk-abcdefghijklmnopqrstuvwxyz123456")
	sanitized := Error(err)

	if strings.Contains(sanitized, "sk-abc") {
		t.Error("sanitized error should not contain OpenAI key format")
	}
}

func TestError_Token(t *testing.T) {
	err := fmt.Errorf("authentication failed with token=secret_token_value")
	sanitized := Error(err)

	if strings.Contains(sanitized, "secret_token_value") {
		t.Error("sanitized error should not contain the token value")
	}
}

func TestError_Secret(t *testing.T) {
	err := fmt.Errorf("config error: secret=my-secret-value")
	sanitized := Error(err)

	if strings.Contains(sanitized, "my-secret-value") {
		t.Error("sanitized error should not contain the secret value")
	}
}

func TestError_Password(t *testing.T) {
	err := fmt.Errorf("login failed: password=supersecret")
	sanitized := Error(err)

	if strings.Contains(sanitized, "supersecret") {
		t.Error("sanitized error should not contain the password")
	}
}

func TestError_AuthorizationHeader(t *testing.T) {
	err := fmt.Errorf("request failed: Authorization: Bearer sk-1234567890abcdef")
	sanitized := Error(err)

	if strings.Contains(sanitized, "sk-1234567890abcdef") {
		t.Error("sanitized error should not contain the API key")
	}
}

func TestError_APIKeyVariant(t *testing.T) {
	err := fmt.Errorf("api-key: sk-abcdefghijklmnopqrstuvwxyz123456")
	sanitized := Error(err)

	if strings.Contains(sanitized, "sk-abc") {
		t.Error("sanitized error should not contain the API key")
	}
}

func TestError_NoMatch(t *testing.T) {
	original := "connection timeout after 30 seconds"
	err := fmt.Errorf("%s", original)
	sanitized := Error(err)

	if sanitized != original {
		t.Errorf("expected %q, got %q", original, sanitized)
	}
}

func TestError_EmptyMessage(t *testing.T) {
	err := fmt.Errorf("")
	sanitized := Error(err)

	if sanitized != "" {
		t.Errorf("expected empty string, got %q", sanitized)
	}
}

func TestError_MultiplePatterns(t *testing.T) {
	err := fmt.Errorf("error: Bearer sk-key and api_key=sk-another-key")
	sanitized := Error(err)

	if strings.Contains(sanitized, "sk-key") || strings.Contains(sanitized, "sk-another-key") {
		t.Error("sanitized error should not contain any API keys")
	}
}
