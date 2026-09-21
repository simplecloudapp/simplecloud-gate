package connection

import (
	"os"
	"regexp"
	"sort"
	"strings"

	"go.minekube.com/gate/pkg/edition/java/proxy"
)

func operationMatches(operation OperationType, key, value string, negate bool) bool {
	var matches bool
	switch operation {
	case OperationRegex:
		expression, err := regexp.Compile("^(?:" + value + ")$")
		matches = err == nil && expression.MatchString(key)
	case OperationPattern:
		// Matches the plugin-api behavior: the key is the pattern and the
		// configured value is the input.
		expression, err := regexp.Compile("^(?:" + key + ")$")
		matches = err == nil && expression.MatchString(value)
	case OperationEquals:
		matches = strings.EqualFold(key, value)
	case OperationContains:
		matches = strings.Contains(strings.ToLower(key), strings.ToLower(value))
	case OperationStartsWith:
		matches = strings.HasPrefix(strings.ToLower(key), strings.ToLower(value))
	case OperationEndsWith:
		matches = strings.HasSuffix(strings.ToLower(key), strings.ToLower(value))
	case OperationGreaterThan:
		// Connection matcher and ENV rule operands are strings. The shared
		// Kotlin matcher only applies GREATER_THAN to numeric operands.
		matches = false
	}
	if negate {
		return !matches
	}
	return matches
}

func findConnection(name string, connections []ConnectionEntry) *ConnectionEntry {
	for i := range connections {
		if strings.EqualFold(connections[i].Name, name) {
			return &connections[i]
		}
	}
	return nil
}

func matchingServers(connection ConnectionEntry, servers []proxy.RegisteredServer) []proxy.RegisteredServer {
	matching := make([]proxy.RegisteredServer, 0)
	for _, server := range servers {
		matcher := connection.ServerNameMatcher
		if operationMatches(matcher.Operation, server.ServerInfo().Name(), matcher.Value, matcher.Negate) {
			matching = append(matching, server)
		}
	}
	return matching
}

func failedRule(connection ConnectionEntry, permissionChecker func(string) bool) *ConnectionRule {
	for i := range connection.Rules {
		rule := &connection.Rules[i]
		if rule.BypassPermission != "" && permissionChecker(rule.BypassPermission) {
			continue
		}

		failed := false
		switch rule.Type {
		case RulePermission:
			required := strings.EqualFold(rule.Value, "true")
			failed = permissionChecker(rule.Name) != required
		case RuleEnv:
			failed = !operationMatches(rule.Operation, os.Getenv(rule.Name), rule.Value, rule.Negate)
		}
		if failed {
			return rule
		}
	}
	return nil
}

func isServerInConnection(serverName, connectionName string, connections []ConnectionEntry, servers []proxy.RegisteredServer) bool {
	connection := findConnection(connectionName, connections)
	if connection == nil {
		return false
	}
	for _, server := range matchingServers(*connection, servers) {
		if strings.EqualFold(server.ServerInfo().Name(), serverName) {
			return true
		}
	}
	return false
}

func leastPopulated(servers []proxy.RegisteredServer, excludedName string) proxy.RegisteredServer {
	var selected proxy.RegisteredServer
	for _, server := range servers {
		if excludedName != "" && server.ServerInfo().Name() == excludedName {
			continue
		}
		if selected == nil || server.Players().Len() < selected.Players().Len() {
			selected = server
		}
	}
	return selected
}

func sortedTargets(targets []FallbackTargetConnection) []FallbackTargetConnection {
	sorted := append([]FallbackTargetConnection(nil), targets...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})
	return sorted
}

func sortedJoinTargets(targets []TargetConnection) []TargetConnection {
	sorted := append([]TargetConnection(nil), targets...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})
	return sorted
}
