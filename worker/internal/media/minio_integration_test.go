//go:build integration

package media_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/canove/whaticket-community/worker/internal/testenv"
)

func TestMinioUploadAndHealth(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	env := testenv.StartMinIO(ctx, t)

	if err := env.Client.Health(ctx); err != nil {
		t.Fatalf("Health: %v", err)
	}

	data := []byte("\x89PNG\r\n\x1a\nfake-payload-bytes")
	publicURL, err := env.Client.Upload(ctx, "tests/object.png", data, "image/png")
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if publicURL == "" {
		t.Fatal("Upload returned empty public URL")
	}
	if !strings.Contains(publicURL, env.Bucket) {
		t.Fatalf("public URL %q does not include bucket %q", publicURL, env.Bucket)
	}
	if !strings.Contains(publicURL, "tests/object.png") {
		t.Fatalf("public URL %q does not include object key", publicURL)
	}
}
