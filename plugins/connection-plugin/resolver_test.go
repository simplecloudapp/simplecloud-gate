package connection

import (
	"testing"

	"go.minekube.com/common/minecraft/component"
)

func TestOperationMatchesSharedSemantics(t *testing.T) {
	tests := []struct {
		name      string
		operation OperationType
		key       string
		value     string
		negate    bool
		want      bool
	}{
		{name: "equals ignores case", operation: OperationEquals, key: "Lobby-1", value: "lobby-1", want: true},
		{name: "starts with ignores case", operation: OperationStartsWith, key: "Lobby-1", value: "lobby", want: true},
		{name: "regex is a full match", operation: OperationRegex, key: "lobby-1", value: `lobby-\d+`, want: true},
		{name: "regex does not partially match", operation: OperationRegex, key: "eu-lobby-1", value: `lobby-\d+`, want: false},
		{name: "pattern uses key as regex", operation: OperationPattern, key: `lobby-\d+`, value: "lobby-2", want: true},
		{name: "negation", operation: OperationContains, key: "lobby", value: "game", negate: true, want: true},
		{name: "greater than rejects string operands", operation: OperationGreaterThan, key: "2", value: "1", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := operationMatches(test.operation, test.key, test.value, test.negate); got != test.want {
				t.Fatalf("operationMatches() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestFailedRuleSupportsPermissionAndBypass(t *testing.T) {
	connection := ConnectionEntry{Rules: []ConnectionRule{{
		Type: RulePermission, Name: "network.vip", Value: "true", BypassPermission: "network.admin",
	}}}
	permissions := map[string]bool{"network.admin": true}
	if failed := failedRule(connection, func(name string) bool { return permissions[name] }); failed != nil {
		t.Fatalf("bypass permission did not bypass rule: %#v", failed)
	}
	delete(permissions, "network.admin")
	if failed := failedRule(connection, func(name string) bool { return permissions[name] }); failed == nil {
		t.Fatal("missing required permission did not fail rule")
	}
}

func TestMessageComponentExpandsMiniMessageSubset(t *testing.T) {
	parsed := messageComponent("<prefix> <color:#dc2626>Hello<br><bold>World</bold>", map[string]string{
		"prefix": "<color:#38bdf8>⚡</color>",
	})
	root, ok := parsed.(*component.Text)
	if !ok || len(root.Extra) < 4 {
		t.Fatalf("unexpected parsed component: %#v", parsed)
	}
	first := root.Extra[0].(*component.Text)
	if first.Content != "⚡" || first.S.Color.Hex() != "#38bdf8" {
		t.Fatalf("variable color was not parsed: %#v", first)
	}
	last := root.Extra[len(root.Extra)-1].(*component.Text)
	if last.Content != "World" || last.S.Bold != component.True || last.S.Color.Hex() != "#dc2626" {
		t.Fatalf("nested style was not parsed: %#v", last)
	}
}
