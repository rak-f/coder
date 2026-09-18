package aibridge_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	"cdr.dev/slog/v3/sloggers/slogtest"

	"github.com/coder/coder/v2/aibridge"
	"github.com/coder/coder/v2/aibridge/recorder"
	"github.com/coder/coder/v2/testutil"
)

// acceptingRecorder terminates a chain under test, accepting every record.
type acceptingRecorder struct{}

func (*acceptingRecorder) RecordInterception(context.Context, *recorder.InterceptionRecord) error {
	return nil
}

func (*acceptingRecorder) RecordInterceptionEnded(context.Context, *recorder.InterceptionRecordEnded) error {
	return nil
}

func (*acceptingRecorder) RecordTokenUsage(context.Context, *recorder.TokenUsageRecord) error {
	return nil
}

func (*acceptingRecorder) RecordPromptUsage(context.Context, *recorder.PromptUsageRecord) error {
	return nil
}

func (*acceptingRecorder) RecordToolUsage(context.Context, *recorder.ToolUsageRecord) error {
	return nil
}

func (*acceptingRecorder) RecordModelThought(context.Context, *recorder.ModelThoughtRecord) error {
	return nil
}

// TestNewRecorder_StampsEveryRecord guards the decision to stamp record times
// in [recorder.LogRecorder] rather than in a dedicated decorator: the stamping
// is only guaranteed for as long as every chain includes the logging
// middleware. If a chain is ever built without it, this fails.
func TestNewRecorder_StampsEveryRecord(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t, testutil.WaitShort)
	logger := slogtest.Make(t, nil)
	term := &acceptingRecorder{}
	rec := aibridge.NewRecorder(logger, otel.Tracer("test"), func(context.Context) (aibridge.Recorder, error) {
		return term, nil
	})

	intc := &recorder.InterceptionRecord{ID: "interception"}
	require.NoError(t, rec.RecordInterception(ctx, intc))
	require.False(t, intc.StartedAt.IsZero(), "interception was not stamped")

	ended := &recorder.InterceptionRecordEnded{ID: "interception"}
	require.NoError(t, rec.RecordInterceptionEnded(ctx, ended))
	require.False(t, ended.EndedAt.IsZero(), "interception end was not stamped")

	prompt := &recorder.PromptUsageRecord{InterceptionID: "interception"}
	require.NoError(t, rec.RecordPromptUsage(ctx, prompt))
	require.False(t, prompt.CreatedAt.IsZero(), "prompt usage was not stamped")

	token := &recorder.TokenUsageRecord{InterceptionID: "interception"}
	require.NoError(t, rec.RecordTokenUsage(ctx, token))
	require.False(t, token.CreatedAt.IsZero(), "token usage was not stamped")

	tool := &recorder.ToolUsageRecord{InterceptionID: "interception"}
	require.NoError(t, rec.RecordToolUsage(ctx, tool))
	require.False(t, tool.CreatedAt.IsZero(), "tool usage was not stamped")

	thought := &recorder.ModelThoughtRecord{InterceptionID: "interception"}
	require.NoError(t, rec.RecordModelThought(ctx, thought))
	require.False(t, thought.CreatedAt.IsZero(), "model thought was not stamped")
}
