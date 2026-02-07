package httpx

import (
	"os"
	"strings"
	"testing"
)

func TestOpenAPIExists(t *testing.T) {
	data, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("openapi missing: %v", err)
	}
	if !strings.Contains(string(data), "openapi:") {
		t.Fatalf("openapi spec invalid")
	}
}
