package connection

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.minekube.com/common/minecraft/component"
	"gopkg.in/yaml.v3"
)

// The field spellings and defaults are from the original connection plugin's
// v2 config classes and docs. Non-default values catch silently ignored keys.
func TestOriginalV2ConfigCompatibility(t *testing.T) {
	files := map[string]string{
		"config.yml": `version: "2"
registration:
  enabled: false
  server-name-pattern: '<region>-<group>-<numerical_id>'
  persistent-server-name-pattern: 'persistent-<name>'
  ignore-server-groups-and-persistent-servers: [private]
  additional-servers:
    - {name: external, address: localhost, port: 25566}
address:
  routes:
    - {subdomain: vip.example.com, target-connection: vip}
connections:
  - name: vip
    server-name-matcher: {value: vip}
    rules:
      - {name: network.vip, value: 'true', bypass-permission: network.admin}
      - {type: ENV, name: REGION, value: eu, operation: EQUALS, negate: true}
network-join-targets:
  enabled: true
  target-connections: [{name: vip, priority: 17}]
fallback:
  enabled: false
  target-connections: [{name: vip, priority: 8, from: [games]}]
`,
		"commands.yml": `version: 2
commands:
  - name: vip
    aliases: [v]
    permission: command.vip
    messages:
      already-connected: '<red>Already here</red>'
      no-target-connection-found: missing
    target-connections: [{name: vip, priority: 9, from: [games]}]
  - name: custom
    target-connections: [{name: vip}]
`,
		"messages.yml": `version: 2
variables:
  custom-prefix: '<gold>VIP</gold>'
  targetConnection: untouched
kick:
  no-fallback-servers: fallback missing
  no-target-connection: join missing
command:
  command-usage: usage
  config-reloading: loading
  config-reloaded-success: success
  config-reloaded-failed: failed
`,
	}
	directory := t.TempDir()
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := loadConfigs(directory)
	if err != nil {
		t.Fatal(err)
	}
	c := loaded.Connection
	if c.Registration.Enabled || c.Registration.ServerNamePattern != "<region>-<group>-<numerical_id>" || c.Registration.PersistentServerNamePattern != "persistent-<name>" || c.Registration.IgnoreServerGroupsAndPersistentServers[0] != "private" || c.Registration.AdditionalServers[0].Port != 25566 {
		t.Fatalf("registration fields lost: %+v", c.Registration)
	}
	if c.Address.Routes[0].TargetConnection != "vip" || c.NetworkJoinTargets.TargetConnections[0].Priority != 17 || c.Fallback.Enabled || c.Fallback.TargetConnections[0].From[0] != "games" {
		t.Fatalf("routing fields lost: %+v", c)
	}
	connection := c.Connections[0]
	if connection.ServerNameMatcher.Operation != OperationStartsWith || !operationMatches(connection.ServerNameMatcher.Operation, "vip-1", connection.ServerNameMatcher.Value, false) {
		t.Fatalf("matcher default differs from Java: %+v", connection)
	}
	if connection.Rules[0].Type != RulePermission || connection.Rules[0].Operation != OperationEquals || connection.Rules[0].BypassPermission != "network.admin" || !connection.Rules[1].Negate {
		t.Fatalf("rules lost: %+v", connection.Rules)
	}
	command := loaded.Commands.Commands[0]
	if command.Aliases[0] != "v" || command.Permission != "command.vip" || command.Messages.AlreadyConnected != "<red>Already here</red>" || command.Messages.NoTargetConnectionFound != "missing" || command.TargetConnections[0].Priority != 9 || command.TargetConnections[0].From[0] != "games" {
		t.Fatalf("command fields lost: %+v", command)
	}
	if loaded.Commands.Commands[1].Messages != defaultCommandMessages() {
		t.Fatalf("missing command messages must use Java defaults: %+v", loaded.Commands.Commands[1])
	}
	m := loaded.Messages
	if m.Kick.NoFallbackServers != "fallback missing" || m.Kick.NoTargetConnection != "join missing" || m.Command != (ConnectionCommandMessages{"usage", "loading", "success", "failed"}) || m.Variables["custom-prefix"] != "<gold>VIP</gold>" || m.Variables["targetConnection"] != "untouched" {
		t.Fatalf("message fields lost: %+v", m)
	}
	for name, original := range files {
		contents, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil || string(contents) != original {
			t.Fatalf("loading rewrote %s: %v", name, err)
		}
	}
}

func TestCamelCaseConfigsStillLoad(t *testing.T) {
	for _, original := range []any{defaultConnectionConfig(), defaultMessageConfig(), defaultCommandConfig()} {
		contents, err := yaml.Marshal(original)
		if err != nil {
			t.Fatal(err)
		}
		var node yaml.Node
		if err := yaml.Unmarshal(contents, &node); err != nil {
			t.Fatal(err)
		}
		var rename func(*yaml.Node)
		rename = func(node *yaml.Node) {
			if node.Kind == yaml.MappingNode {
				for i := 0; i < len(node.Content); i += 2 {
					node.Content[i].Value = camelConfigKey(node.Content[i].Value)
				}
			}
			for _, child := range node.Content {
				rename(child)
			}
		}
		rename(&node)
		contents, err = yaml.Marshal(&node)
		if err != nil {
			t.Fatal(err)
		}
		target := reflect.New(reflect.TypeOf(original))
		if err := decodeConfigYAML(contents, target.Interface()); err != nil {
			t.Fatal(err)
		}
		// Normalization of nil lists is normally done by loadConfigs.
		roundTrip, _ := yaml.Marshal(target.Interface())
		want, _ := yaml.Marshal(original)
		if string(roundTrip) != string(want) {
			t.Fatalf("camelCase fields changed for %T:\n%s", original, roundTrip)
		}
	}
}

func TestConfigVersionAndAliasErrorsPreserveReloadedConfig(t *testing.T) {
	store, err := newConfigStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	original := store.get()
	for _, contents := range []string{
		"version: 99", "version: 1", "version: broken", "version: 0", "version: 2\nversion: 2",
		"version: 2\nnetwork-join-targets: {}\nnetworkJoinTargets: {}",
	} {
		if err := os.WriteFile(filepath.Join(store.directory, "config.yml"), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := store.reload(); err == nil {
			t.Fatalf("accepted invalid config: %s", contents)
		}
		if !reflect.DeepEqual(original, store.get()) {
			t.Fatal("failed reload replaced working configuration")
		}
	}
}

func TestConfigMigrationsRunInOrder(t *testing.T) {
	var node yaml.Node
	if err := yaml.Unmarshal([]byte("version: '2'\nconnections: []"), &node); err != nil {
		t.Fatal(err)
	}
	var calls []int
	err := migrateConfig(node.Content[0], 4, map[int]configMigration{
		2: func(root *yaml.Node) error { calls = append(calls, 2); return nil },
		3: func(root *yaml.Node) error { calls = append(calls, 3); return nil },
	})
	if err != nil || !reflect.DeepEqual(calls, []int{2, 3}) || node.Content[0].Content[1].Value != "4" {
		t.Fatalf("migration failed: calls=%v err=%v", calls, err)
	}
	err = migrateConfig(node.Content[0], 5, map[int]configMigration{
		4: func(root *yaml.Node) error { return errors.New("cannot convert") },
	})
	if err == nil || !strings.Contains(err.Error(), "cannot convert") || node.Content[0].Content[1].Value != "4" {
		t.Fatalf("failed migration advanced version: %v", err)
	}
}

func TestExplicitEmptyTargetsDoNotInheritLobby(t *testing.T) {
	config := defaultConnectionConfig()
	if err := decodeConfigYAML([]byte("version: 2\nnetwork-join-targets: {}\nfallback: {}"), &config); err != nil {
		t.Fatal(err)
	}
	if !config.NetworkJoinTargets.Enabled || len(config.NetworkJoinTargets.TargetConnections) != 0 || !config.Fallback.Enabled || len(config.Fallback.TargetConnections) != 0 {
		t.Fatalf("explicit sections should use their own defaults: %+v", config)
	}
}

func TestYAMLAnchorsAndMergesPreserveConfigAliases(t *testing.T) {
	var config ConnectionConfig
	err := decodeConfigYAML([]byte(`version: 2
defaults: &defaults
  serverNamePattern: '<name>-<numerical_id>'
  persistentServerNamePattern: '<name>'
  enabled: true
registration:
  <<: *defaults
  server-name-pattern: 'custom-<name>'
connections:
  - name: vip
    server-name-matcher: &matcher
      operation: STARTS_WITH
      value: vip
  - name: second
    serverNameMatcher: *matcher
`), &config)
	if err != nil {
		t.Fatal(err)
	}
	if !config.Registration.Enabled || config.Registration.ServerNamePattern != "custom-<name>" || config.Registration.PersistentServerNamePattern != "<name>" || config.Connections[1].ServerNameMatcher.Value != "vip" {
		t.Fatalf("YAML merge or alias changed values: %+v", config)
	}
}

func TestNamedMessageColorsRestoreNestedStyle(t *testing.T) {
	root := messageComponent("<red>A<gold>B</gold>C</red>D", nil).(*component.Text)
	want := []string{"#ff5555", "#ffaa00", "#ff5555", "#ffffff"}
	if len(root.Extra) != len(want) {
		t.Fatalf("unexpected text: %+v", root)
	}
	for i, item := range root.Extra {
		if item.(*component.Text).S.Color.Hex() != want[i] {
			t.Fatalf("segment %d has wrong color", i)
		}
	}
}
