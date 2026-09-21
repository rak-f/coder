package codersdk

import (
	"context"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ConnectionLog struct {
	ID                     uuid.UUID           `json:"id" format:"uuid"`
	ConnectTime            time.Time           `json:"connect_time" format:"date-time"`
	Organization           MinimalOrganization `json:"organization"`
	WorkspaceOwnerID       uuid.UUID           `json:"workspace_owner_id" format:"uuid"`
	WorkspaceOwnerUsername string              `json:"workspace_owner_username"`
	WorkspaceID            uuid.UUID           `json:"workspace_id" format:"uuid"`
	WorkspaceName          string              `json:"workspace_name"`
	AgentName              string              `json:"agent_name"`
	IP                     *netip.Addr         `json:"ip,omitempty"`
	// Type is the app that connected, such as "cursor", or a web
	// ConnectionType, such as "port_forwarding".
	Type            string         `json:"type"`
	TypeDisplayName string         `json:"type_display_name"`
	TypeFamily      ConnectionType `json:"type_family"`

	// WebInfo is only set when `type` is one of:
	// - `ConnectionTypePortForwarding`
	// - `ConnectionTypeWorkspaceApp`
	// - `ConnectionTypeTunnel`
	WebInfo *ConnectionLogWebInfo `json:"web_info,omitempty"`

	// SSHInfo is set for every other `type`.
	SSHInfo *ConnectionLogSSHInfo `json:"ssh_info,omitempty"`
}

// ConnectionType is a value the connection log `type` filter accepts.
type ConnectionType string

const (
	// App families.
	ConnectionTypeSSH             = ConnectionType(AppFamilySSH)
	ConnectionTypeVSCode          = ConnectionType(AppFamilyVSCode)
	ConnectionTypeJetBrains       = ConnectionType(AppFamilyJetBrains)
	ConnectionTypeReconnectingPTY = ConnectionType(AppFamilyReconnectingPTY)
	ConnectionTypeUnknown         = ConnectionType(AppFamilyUnknown)

	// Web connection types.
	ConnectionTypeWorkspaceApp   ConnectionType = "workspace_app"
	ConnectionTypePortForwarding ConnectionType = "port_forwarding"
	// ConnectionTypeTunnel records accepted and denied tailnet tunnel
	// requests made by authenticated users.
	ConnectionTypeTunnel ConnectionType = "tunnel"
)

var webTypeDisplayNames = map[ConnectionType]string{
	ConnectionTypeWorkspaceApp:   "Workspace App",
	ConnectionTypePortForwarding: "Port Forwarding",
	ConnectionTypeTunnel:         "Tunnel",
}

// notConnectionFamilies are app families the `type` filter does not accept.
var notConnectionFamilies = map[AppFamilyName]struct{}{
	// Recorded only by usage tracking, never by a connection log.
	AppFamilySFTP: {},
}

// FilterableConnectionTypes lists the values the `type` filter accepts.
func FilterableConnectionTypes() []ConnectionType {
	types := make([]ConnectionType, 0, len(webTypeDisplayNames)+len(sessionApps)+1)
	types = append(types, ConnectionTypeUnknown)
	for typ := range webTypeDisplayNames {
		types = append(types, typ)
	}
	for _, family := range SessionCountAppFamilies() {
		if _, skip := notConnectionFamilies[family]; !skip {
			types = append(types, ConnectionType(family))
		}
	}
	slices.Sort(types)
	return slices.Compact(types)
}

// DisplayName returns the human-readable name of t.
func (t ConnectionType) DisplayName() string {
	return ConnectionLogTypeDisplayName(string(t))
}

// ConnectionLogTypeDisplayName returns the human-readable name of a
// ConnectionLog.Type value, or the value itself if it is unregistered.
func ConnectionLogTypeDisplayName(logType string) string {
	if logType == string(ConnectionTypeUnknown) {
		return "Unknown"
	}
	if name, ok := webTypeDisplayNames[ConnectionType(logType)]; ok {
		return name
	}
	return AppDisplayName(logType)
}

// ConnectionLogTypeFamily returns the filter that matches a ConnectionLog.Type
// value.
func ConnectionLogTypeFamily(logType string) ConnectionType {
	if _, ok := webTypeDisplayNames[ConnectionType(logType)]; ok {
		return ConnectionType(logType)
	}
	family := AppNameFamily(logType)
	if _, skip := notConnectionFamilies[family]; skip {
		return ConnectionTypeUnknown
	}
	return ConnectionType(family)
}

// KnownConnectionLogTypes lists the values ConnectionTypeUnknown excludes.
func KnownConnectionLogTypes() []string {
	var known []string
	for _, t := range FilterableConnectionTypes() {
		known = append(known, t.MatchingTypes()...)
	}
	return known
}

// MatchingTypes lists the ConnectionLog.Type values a filter on t matches.
// It returns nil for ConnectionTypeUnknown and for an invalid t.
func (t ConnectionType) MatchingTypes() []string {
	if _, ok := webTypeDisplayNames[t]; ok {
		return []string{string(t)}
	}
	return AppNamesInFamily(AppFamilyName(t))
}

// ConnectionLogStatus is the status of a connection log entry.
// It's the argument to the `status` filter when fetching connection logs.
type ConnectionLogStatus string

const (
	ConnectionLogStatusOngoing   ConnectionLogStatus = "ongoing"
	ConnectionLogStatusCompleted ConnectionLogStatus = "completed"
)

func (s ConnectionLogStatus) Valid() bool {
	switch s {
	case ConnectionLogStatusOngoing, ConnectionLogStatusCompleted:
		return true
	default:
		return false
	}
}

type ConnectionLogWebInfo struct {
	UserAgent string `json:"user_agent"`
	// User is omitted if the connection event was unauthenticated.
	User       *User  `json:"user"`
	SlugOrPort string `json:"slug_or_port"`
	// StatusCode is the HTTP status code or tunnel authorization outcome.
	StatusCode int32 `json:"status_code"`
}

type ConnectionLogSSHInfo struct {
	ConnectionID uuid.UUID `json:"connection_id" format:"uuid"`
	// DisconnectTime is omitted if a disconnect event with the same connection ID
	// has not yet been seen.
	DisconnectTime *time.Time `json:"disconnect_time,omitempty" format:"date-time"`
	// DisconnectReason is omitted if a disconnect event with the same connection ID
	// has not yet been seen.
	DisconnectReason string `json:"disconnect_reason,omitempty"`
	// ExitCode is the exit code of the SSH session. It is omitted if a
	// disconnect event with the same connection ID has not yet been seen.
	ExitCode *int32 `json:"exit_code,omitempty"`
}

type ConnectionLogsRequest struct {
	SearchQuery string `json:"q,omitempty"`
	Pagination
}

type ConnectionLogResponse struct {
	ConnectionLogs []ConnectionLog `json:"connection_logs"`
	Count          int64           `json:"count"`
	CountCap       int64           `json:"count_cap"`
}

func (c *Client) ConnectionLogs(ctx context.Context, req ConnectionLogsRequest) (ConnectionLogResponse, error) {
	res, err := c.Request(ctx, http.MethodGet, "/api/v2/connectionlog", nil, req.asRequestOption(), func(r *http.Request) {
		q := r.URL.Query()
		var params []string
		if req.SearchQuery != "" {
			params = append(params, req.SearchQuery)
		}
		q.Set("q", strings.Join(params, " "))
		r.URL.RawQuery = q.Encode()
	})
	if err != nil {
		return ConnectionLogResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return ConnectionLogResponse{}, ReadBodyAsError(res)
	}

	var logRes ConnectionLogResponse
	err = ReadBodyAsJSON(res, &logRes)
	if err != nil {
		return ConnectionLogResponse{}, err
	}
	return logRes, nil
}
