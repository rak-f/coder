package integrationtest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"cdr.dev/slog/v3"

	"github.com/coder/coder/v2/aibridge/fixtures"
	"github.com/coder/coder/v2/aibridge/internal/testutil"
	"github.com/coder/coder/v2/aibridge/recorder"
)

const testAPIKeyID = "api-key-123"

// bufferSink collects log entries as JSON, so that a test can assert on the
// structured interception records the bridge emitted.
type bufferSink struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *bufferSink) LogEntry(_ context.Context, e slog.SinkEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fields := make(map[string]any, len(e.Fields))
	for _, f := range e.Fields {
		fields[f.Name] = f.Value
	}
	line, err := json.Marshal(map[string]any{"msg": e.Message, "fields": fields})
	if err != nil {
		return
	}
	_, _ = s.buf.Write(line)
	_ = s.buf.WriteByte('\n')
}

func (*bufferSink) Sync() {}

// recordTypes returns the record_type of every interception record logged, in
// order.
func (s *bufferSink) recordTypes(t *testing.T) []string {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()

	var types []string
	scanner := bufio.NewScanner(bytes.NewReader(s.buf.Bytes()))
	for scanner.Scan() {
		var line struct {
			Msg    string         `json:"msg"`
			Fields map[string]any `json:"fields"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if line.Msg != recorder.InterceptionLogMarker {
			continue
		}
		recordType, _ := line.Fields["record_type"].(string)
		types = append(types, recordType)
	}
	return types
}

// TestContentRecording_DisabledStillExportsRecords is the end-to-end guard for
// the design: with content recording disabled and the gateway emitting
// structured logs, prompts, tool calls and model thoughts reach the logs but
// never the recorder, while the records cost control depends on are unaffected
// and the request still succeeds.
func TestContentRecording_DisabledStillExportsRecords(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), testutil.WaitLong)
	t.Cleanup(cancel)

	fix := fixtures.Parse(t, fixtures.AntMultiThinkingBuiltinTool)
	upstream := testutil.NewMockUpstream(ctx, t, testutil.NewFixtureResponse(fix))

	sink := &bufferSink{}
	bridgeServer := newBridgeTestServer(ctx, t, upstream.URL,
		withStructuredLogging(testAPIKeyID),
		withLogSink(sink),
		withRecorderMiddleware(recorder.WithoutRecords(recorder.DisabledRecords{
			PromptUsage:  true,
			ToolUsage:    true,
			ModelThought: true,
		})),
	)

	resp, err := bridgeServer.makeRequest(t, http.MethodPost, pathAnthropicMessages, fix.Request())
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	bridgeServer.Close()

	rec := bridgeServer.Recorder
	require.Empty(t, rec.RecordedPromptUsages(), "prompts must not be recorded")
	require.Empty(t, rec.RecordedToolUsages(), "tool usage must not be recorded")
	require.Empty(t, rec.RecordedModelThoughts(), "model thoughts must not be recorded")

	// The records cost control depends on are untouched.
	require.Len(t, rec.RecordedInterceptions(), 1)
	require.NotEmpty(t, rec.RecordedTokenUsages(), "token usage must still be recorded")
	rec.VerifyAllInterceptionsEnded(t)

	// Every record still reached the logs, including the dropped ones.
	types := sink.recordTypes(t)
	require.Contains(t, types, recorder.RecordTypeInterceptionStart)
	require.Contains(t, types, recorder.RecordTypeInterceptionEnd)
	require.Contains(t, types, recorder.RecordTypeTokenUsage)
	require.Contains(t, types, recorder.RecordTypePromptUsage)
	require.Contains(t, types, recorder.RecordTypeToolUsage)
	require.Contains(t, types, recorder.RecordTypeModelThought)
}

// TestContentRecording_EnabledRecordsEverything pins the default: without the
// middleware, all six record types are recorded as before.
func TestContentRecording_EnabledRecordsEverything(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), testutil.WaitLong)
	t.Cleanup(cancel)

	fix := fixtures.Parse(t, fixtures.AntMultiThinkingBuiltinTool)
	upstream := testutil.NewMockUpstream(ctx, t, testutil.NewFixtureResponse(fix))

	bridgeServer := newBridgeTestServer(ctx, t, upstream.URL)

	resp, err := bridgeServer.makeRequest(t, http.MethodPost, pathAnthropicMessages, fix.Request())
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	bridgeServer.Close()

	rec := bridgeServer.Recorder
	require.NotEmpty(t, rec.RecordedPromptUsages())
	require.NotEmpty(t, rec.RecordedToolUsages())
	require.NotEmpty(t, rec.RecordedModelThoughts())
	require.NotEmpty(t, rec.RecordedTokenUsages())
	require.Len(t, rec.RecordedInterceptions(), 1)
}

// TestStructuredLogging_DisabledByDefault ensures the gateway stays silent
// unless the deployment asks it to emit, so that coderd remains the only
// emitter for existing deployments.
func TestStructuredLogging_DisabledByDefault(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), testutil.WaitLong)
	t.Cleanup(cancel)

	fix := fixtures.Parse(t, fixtures.AntSimple)
	upstream := testutil.NewMockUpstream(ctx, t, testutil.NewFixtureResponse(fix))

	sink := &bufferSink{}
	bridgeServer := newBridgeTestServer(ctx, t, upstream.URL, withLogSink(sink))

	resp, err := bridgeServer.makeRequest(t, http.MethodPost, pathAnthropicMessages, fix.Request())
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	bridgeServer.Close()

	require.Empty(t, sink.recordTypes(t))
}
