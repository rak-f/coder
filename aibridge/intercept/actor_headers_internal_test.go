package intercept

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coder/coder/v2/aibridge/context"
	"github.com/coder/coder/v2/aibridge/recorder"
)

func TestHeadersFromActorUsesConfiguredNames(t *testing.T) {
	t.Parallel()

	actor := &context.Actor{
		ID: "user-123",
		Metadata: recorder.Metadata{
			"Username": "alice",
			"Email":    "alice@example.com",
			"Plan":     "pro",
		},
	}

	require.Equal(t, map[string]string{
		"X-Downstream-User-Id":            "user-123",
		"X-Downstream-Username":           "alice",
		"X-Downstream-Email":              "alice@example.com",
		"X-AI-Bridge-Actor-Metadata-Plan": "pro",
	}, headersFromActor(actor, map[string]string{
		"id":       "X-Downstream-User-Id",
		"username": "X-Downstream-Username",
		"email":    "X-Downstream-Email",
	}))
}

func TestHeadersFromActorOmitsMissingEmail(t *testing.T) {
	t.Parallel()

	require.Equal(t, map[string]string{
		ActorIDHeader(): "user-123",
	}, headersFromActor(&context.Actor{ID: "user-123"}, nil))
}
