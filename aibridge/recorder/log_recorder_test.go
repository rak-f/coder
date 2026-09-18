package recorder_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/xerrors"

	"cdr.dev/slog/v3"
	"cdr.dev/slog/v3/sloggers/slogjson"

	"github.com/coder/coder/v2/aibridge/recorder"
)

// logLine is a parsed JSON log entry.
type logLine struct {
	Msg    string         `json:"msg"`
	Level  string         `json:"level"`
	Fields map[string]any `json:"fields"`
}

func parseLogLines(t *testing.T, buf *bytes.Buffer) []logLine {
	t.Helper()
	var lines []logLine
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		var line logLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err == nil {
			lines = append(lines, line)
		}
	}
	return lines
}

// structuredLines returns the interception record lines in buf.
func structuredLines(t *testing.T, buf *bytes.Buffer) []logLine {
	t.Helper()
	var result []logLine
	for _, line := range parseLogLines(t, buf) {
		if line.Msg == recorder.InterceptionLogMarker {
			result = append(result, line)
		}
	}
	return result
}

// newStructuredRecorder builds a LogRecorder that emits structured lines into
// buf and delegates to wrapped.
func newStructuredRecorder(buf *bytes.Buffer, wrapped recorder.Recorder) *recorder.LogRecorder {
	logger := slog.Make(slogjson.Sink(buf)).Leveled(slog.LevelDebug)
	return recorder.NewLogRecorder(logger, "api-key-123", true, wrapped)
}

func TestLogRecorder_StructuredFieldsPerRecordType(t *testing.T) {
	t.Parallel()

	sessionID := "session-id"
	toolCallID := "correlating-tool-call"
	serverURL := "https://mcp.example.com"

	cases := []struct {
		name     string
		record   func(context.Context, recorder.Recorder) error
		wantType string
		want     map[string]any
	}{
		{
			name: "interception",
			record: func(ctx context.Context, r recorder.Recorder) error {
				return r.RecordInterception(ctx, &recorder.InterceptionRecord{
					ID:                    "interception-id",
					InitiatorID:           "initiator-id",
					Provider:              "anthropic",
					Model:                 "claude",
					Client:                "claude-code",
					ClientSessionID:       &sessionID,
					UserAgent:             "curl/8",
					CorrelatingToolCallID: &toolCallID,
					Metadata:              recorder.Metadata{"key": "value"},
				})
			},
			wantType: recorder.RecordTypeInterceptionStart,
			want: map[string]any{
				"interception_id":          "interception-id",
				"initiator_id":             "initiator-id",
				"api_key_id":               "api-key-123",
				"provider":                 "anthropic",
				"model":                    "claude",
				"client":                   "claude-code",
				"client_session_id":        sessionID,
				"correlating_tool_call_id": toolCallID,
				"metadata": map[string]any{
					"key":                         "value",
					recorder.MetadataUserAgentKey: "curl/8",
				},
			},
		},
		{
			name: "interception ended",
			record: func(ctx context.Context, r recorder.Recorder) error {
				return r.RecordInterceptionEnded(ctx, &recorder.InterceptionRecordEnded{ID: "interception-id"})
			},
			wantType: recorder.RecordTypeInterceptionEnd,
			want:     map[string]any{"interception_id": "interception-id"},
		},
		{
			name: "prompt usage",
			record: func(ctx context.Context, r recorder.Recorder) error {
				return r.RecordPromptUsage(ctx, &recorder.PromptUsageRecord{
					InterceptionID: "interception-id",
					MsgID:          "msg-id",
					Prompt:         "why is the sky blue?",
					Metadata:       recorder.Metadata{"key": "value"},
				})
			},
			wantType: recorder.RecordTypePromptUsage,
			want: map[string]any{
				"interception_id": "interception-id",
				"msg_id":          "msg-id",
				"prompt":          "why is the sky blue?",
				"metadata":        map[string]any{"key": "value"},
			},
		},
		{
			name: "token usage",
			record: func(ctx context.Context, r recorder.Recorder) error {
				return r.RecordTokenUsage(ctx, &recorder.TokenUsageRecord{
					InterceptionID:        "interception-id",
					MsgID:                 "msg-id",
					Input:                 11,
					Output:                22,
					CacheReadInputTokens:  33,
					CacheWriteInputTokens: 44,
				})
			},
			wantType: recorder.RecordTypeTokenUsage,
			want: map[string]any{
				"interception_id":          "interception-id",
				"msg_id":                   "msg-id",
				"input_tokens":             float64(11),
				"output_tokens":            float64(22),
				"cache_read_input_tokens":  float64(33),
				"cache_write_input_tokens": float64(44),
			},
		},
		{
			name: "tool usage",
			record: func(ctx context.Context, r recorder.Recorder) error {
				return r.RecordToolUsage(ctx, &recorder.ToolUsageRecord{
					InterceptionID:  "interception-id",
					MsgID:           "msg-id",
					Tool:            "coder_whoami",
					ToolCallID:      "tool-call-id",
					ItemID:          "item-id",
					ServerURL:       &serverURL,
					Args:            map[string]any{"arg": "value"},
					Injected:        true,
					InvocationError: xerrors.New("boom"),
				})
			},
			wantType: recorder.RecordTypeToolUsage,
			want: map[string]any{
				"interception_id":  "interception-id",
				"msg_id":           "msg-id",
				"tool":             "coder_whoami",
				"tool_call_id":     "tool-call-id",
				"item_id":          "item-id",
				"server_url":       serverURL,
				"input":            `{"arg":"value"}`,
				"injected":         true,
				"invocation_error": "boom",
			},
		},
		{
			name: "model thought",
			record: func(ctx context.Context, r recorder.Recorder) error {
				return r.RecordModelThought(ctx, &recorder.ModelThoughtRecord{
					InterceptionID: "interception-id",
					Content:        "thinking",
				})
			},
			wantType: recorder.RecordTypeModelThought,
			want: map[string]any{
				"interception_id": "interception-id",
				"content":         "thinking",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			require.NoError(t, tc.record(t.Context(), newStructuredRecorder(buf, &stubRecorder{})))

			lines := structuredLines(t, buf)
			require.Len(t, lines, 1)
			line := lines[0]

			require.Equal(t, "INFO", line.Level)
			require.Equal(t, tc.wantType, line.Fields["record_type"])
			for field, want := range tc.want {
				require.Equal(t, want, line.Fields[field], "field %q", field)
			}

			// Every record carries the time it was stamped with, which is
			// what makes moving the emission above the terminal worthwhile.
			timeField := "created_at"
			switch tc.wantType {
			case recorder.RecordTypeInterceptionStart:
				timeField = "started_at"
			case recorder.RecordTypeInterceptionEnd:
				timeField = "ended_at"
			}
			require.NotEmpty(t, line.Fields[timeField], "%q is missing", timeField)
			require.NotContains(t, line.Fields[timeField], "0001-01-01", "%q was never stamped", timeField)

			// Lineage is resolved from the database by coderd, so the gateway
			// must not claim to know it.
			require.NotContains(t, line.Fields, "thread_parent_id")
			require.NotContains(t, line.Fields, "thread_root_id")
		})
	}
}

func TestLogRecorder_NoStructuredLinesWhenDisabled(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	logger := slog.Make(slogjson.Sink(buf)).Leveled(slog.LevelDebug)
	rec := recorder.NewLogRecorder(logger, "api-key-123", false, &stubRecorder{})

	require.NoError(t, rec.RecordPromptUsage(t.Context(), &recorder.PromptUsageRecord{Prompt: "secret"}))

	require.Empty(t, structuredLines(t, buf))
}

// TestLogRecorder_StructuredLineSurvivesADroppedRecord is the regression guard
// for the whole design: a record dropped below the logging middleware must
// still reach the SIEM.
func TestLogRecorder_StructuredLineSurvivesADroppedRecord(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	stub := &stubRecorder{}
	filtered := recorder.NewFilterRecorder(recorder.DisabledRecords{PromptUsage: true}, stub)
	rec := newStructuredRecorder(buf, filtered)

	require.NoError(t, rec.RecordPromptUsage(t.Context(), &recorder.PromptUsageRecord{Prompt: "why is the sky blue?"}))

	lines := structuredLines(t, buf)
	require.Len(t, lines, 1)
	require.Equal(t, "why is the sky blue?", lines[0].Fields["prompt"])
	require.Empty(t, stub.calls, "the record should not have been delegated")
}

// TestLogRecorder_DoesNotMutateRecordMetadata guards against the gateway
// writing the user agent into the record itself: coderd merges it too, and
// warns when the key is already set.
func TestLogRecorder_DoesNotMutateRecordMetadata(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	req := &recorder.InterceptionRecord{
		Metadata:  recorder.Metadata{"key": "value"},
		UserAgent: "curl/8",
	}
	require.NoError(t, newStructuredRecorder(buf, &stubRecorder{}).RecordInterception(t.Context(), req))

	require.Equal(t, recorder.Metadata{"key": "value"}, req.Metadata)

	lines := structuredLines(t, buf)
	require.Len(t, lines, 1)
	require.Equal(t, "curl/8", lines[0].Fields["metadata"].(map[string]any)[recorder.MetadataUserAgentKey])
}

// TestLogRecorder_FailureMessagesUnchanged pins the Warn messages, which are
// reproduced from WrappedRecorder and AsyncRecorder so that log-based alerting
// keeps working.
func TestLogRecorder_FailureMessagesUnchanged(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	rec := newStructuredRecorder(buf, &stubRecorder{err: xerrors.New("terminal failed")})

	require.Error(t, rec.RecordPromptUsage(t.Context(), &recorder.PromptUsageRecord{}))

	var messages []string
	for _, line := range parseLogLines(t, buf) {
		if line.Level == "WARN" {
			messages = append(messages, line.Msg)
		}
	}
	require.Equal(t, []string{"failed to record prompt usage", "failed to record usage"}, messages)
}
