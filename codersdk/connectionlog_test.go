package codersdk_test

import (
	"go/constant"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"

	"github.com/coder/coder/v2/codersdk"
)

func TestConnectionTypeMatchingTypes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		typ  codersdk.ConnectionType
		want []string
	}{
		{"FamilyMatchesItsApps", codersdk.ConnectionTypeJetBrains, []string{"jetbrains"}},
		{"SSHFamilyCoversZed", codersdk.ConnectionTypeSSH, []string{"ssh", "zed"}},
		{"WebTypeMatchesItself", codersdk.ConnectionTypeTunnel, []string{"tunnel"}},
		{"UnknownMatchesByExclusion", codersdk.ConnectionTypeUnknown, nil},
		{"AppNameIsNotAFamily", "cursor", nil},
		{"Unrecognized", "no_such_type", nil},
		{"Empty", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.typ.MatchingTypes())
		})
	}

	// The VS Code family grows as forks are registered, so pin the behavior
	// rather than the list.
	vscode := codersdk.ConnectionTypeVSCode.MatchingTypes()
	require.Contains(t, vscode, "vscode")
	require.Contains(t, vscode, "cursor")
	require.NotContains(t, vscode, "jetbrains")
}

func TestConnectionLogTypeDisplayName(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		logType string
		want    string
	}{
		{"AppNamedLikeItsFamily", "vscode", "VS Code"},
		{"WebType", "workspace_app", "Workspace App"},
		{"App", "cursor", "Cursor"},
		{"AppKeepsItsPunctuation", "code_server", "code-server"},
		{"UnregisteredAppPresentsAsItself", "an_unregistered_ide", "an_unregistered_ide"},
		{"Unknown", "unknown", "Unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, codersdk.ConnectionLogTypeDisplayName(tc.logType))
		})
	}
}

func TestConnectionTypeDisplayName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "Visual Studio Code", codersdk.ConnectionTypeVSCode.DisplayName())
	require.Equal(t, "JetBrains", codersdk.ConnectionTypeJetBrains.DisplayName())
	require.Equal(t, "Workspace App", codersdk.ConnectionTypeWorkspaceApp.DisplayName())
}

func TestConnectionLogTypeFamily(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		logType string
		want    codersdk.ConnectionType
	}{
		{"App", "cursor", codersdk.ConnectionTypeVSCode},
		{"Family", "jetbrains", codersdk.ConnectionTypeJetBrains},
		{"WebType", "port_forwarding", codersdk.ConnectionTypePortForwarding},
		{"UnregisteredApp", "an_unregistered_ide", codersdk.ConnectionTypeUnknown},
		{"UnfilterableFamily", "sftp", codersdk.ConnectionTypeUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, codersdk.ConnectionLogTypeFamily(tc.logType))
		})
	}
}

// Pins the ConnectionType constants, read from source, to
// FilterableConnectionTypes.
func TestConnectionTypesCoverFilterableTypes(t *testing.T) {
	t.Parallel()

	pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedTypes}, ".")
	require.NoError(t, err)
	require.Len(t, pkgs, 1)
	require.Empty(t, pkgs[0].Errors)

	scope := pkgs[0].Types.Scope()
	connType := scope.Lookup("ConnectionType").Type()
	var declared []codersdk.ConnectionType
	for _, name := range scope.Names() {
		if c, ok := scope.Lookup(name).(*types.Const); ok && types.Identical(c.Type(), connType) {
			declared = append(declared, codersdk.ConnectionType(constant.StringVal(c.Val())))
		}
	}
	require.ElementsMatch(t, declared, codersdk.FilterableConnectionTypes())

	for _, typ := range declared {
		require.NotEqual(t, string(typ), typ.DisplayName(), "ConnectionType %q has no display name", typ)
		if typ != codersdk.ConnectionTypeUnknown {
			require.NotEmpty(t, typ.MatchingTypes(), "ConnectionType %q matches nothing", typ)
		}
	}

	// Unknown matches by exclusion, so no stored value may resolve to it.
	for _, logType := range codersdk.KnownConnectionLogTypes() {
		require.NotEqual(t, codersdk.ConnectionTypeUnknown, codersdk.ConnectionLogTypeFamily(logType), logType)
	}
}
