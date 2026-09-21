package connection

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"go.minekube.com/gate/pkg/util/validation"
	"gopkg.in/yaml.v3"
)

const (
	configVersion    = 2
	defaultConfigDir = "simplecloud-connection"
)

type OperationType string

const (
	OperationRegex       OperationType = "REGEX"
	OperationPattern     OperationType = "PATTERN"
	OperationEquals      OperationType = "EQUALS"
	OperationContains    OperationType = "CONTAINS"
	OperationStartsWith  OperationType = "STARTS_WITH"
	OperationEndsWith    OperationType = "ENDS_WITH"
	OperationGreaterThan OperationType = "GREATER_THAN"
)

type RuleType string

const (
	RulePermission RuleType = "PERMISSION"
	RuleEnv        RuleType = "ENV"
)

type ConnectionConfig struct {
	Version            int                      `yaml:"version"`
	Registration       RegistrationConfig       `yaml:"registration"`
	Address            AddressConfig            `yaml:"address"`
	Connections        []ConnectionEntry        `yaml:"connections"`
	NetworkJoinTargets NetworkJoinTargetsConfig `yaml:"network-join-targets"`
	Fallback           FallbackConfig           `yaml:"fallback"`
}

type RegistrationConfig struct {
	Enabled                                bool                 `yaml:"enabled"`
	ServerNamePattern                      string               `yaml:"server-name-pattern"`
	PersistentServerNamePattern            string               `yaml:"persistent-server-name-pattern"`
	IgnoreServerGroupsAndPersistentServers []string             `yaml:"ignore-server-groups-and-persistent-servers"`
	AdditionalServers                      []RegistrationServer `yaml:"additional-servers"`
}

type RegistrationServer struct {
	Name    string `yaml:"name"`
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
}

type AddressConfig struct {
	Routes []SubdomainRoute `yaml:"routes"`
}

type SubdomainRoute struct {
	Subdomain        string `yaml:"subdomain"`
	TargetConnection string `yaml:"target-connection"`
}

type ConnectionEntry struct {
	Name              string                     `yaml:"name"`
	ServerNameMatcher ServerMatcherConfiguration `yaml:"server-name-matcher"`
	Rules             []ConnectionRule           `yaml:"rules"`
}

type ServerMatcherConfiguration struct {
	Operation OperationType `yaml:"operation"`
	Value     string        `yaml:"value"`
	Negate    bool          `yaml:"negate"`
}

type ConnectionRule struct {
	Type             RuleType      `yaml:"type"`
	Name             string        `yaml:"name"`
	Value            string        `yaml:"value"`
	Operation        OperationType `yaml:"operation"`
	Negate           bool          `yaml:"negate"`
	BypassPermission string        `yaml:"bypass-permission"`
}

type NetworkJoinTargetsConfig struct {
	Enabled           bool               `yaml:"enabled"`
	TargetConnections []TargetConnection `yaml:"target-connections"`
}

type TargetConnection struct {
	Name     string `yaml:"name"`
	Priority int    `yaml:"priority"`
}

type FallbackConfig struct {
	Enabled           bool                       `yaml:"enabled"`
	TargetConnections []FallbackTargetConnection `yaml:"target-connections"`
}

type FallbackTargetConnection struct {
	Name     string   `yaml:"name"`
	Priority int      `yaml:"priority"`
	From     []string `yaml:"from"`
}

type MessageConfig struct {
	Version   int                       `yaml:"version"`
	Variables map[string]string         `yaml:"variables"`
	Kick      KickMessages              `yaml:"kick"`
	Command   ConnectionCommandMessages `yaml:"command"`
}

type KickMessages struct {
	NoFallbackServers  string `yaml:"no-fallback-servers"`
	NoTargetConnection string `yaml:"no-target-connection"`
}

type ConnectionCommandMessages struct {
	CommandUsage          string `yaml:"command-usage"`
	ConfigReloading       string `yaml:"config-reloading"`
	ConfigReloadedSuccess string `yaml:"config-reloaded-success"`
	ConfigReloadedFailed  string `yaml:"config-reloaded-failed"`
}

type CommandConfig struct {
	Version  int            `yaml:"version"`
	Commands []CommandEntry `yaml:"commands"`
}

type CommandEntry struct {
	Name              string                     `yaml:"name"`
	Aliases           []string                   `yaml:"aliases"`
	Permission        string                     `yaml:"permission"`
	Messages          CommandMessages            `yaml:"messages"`
	TargetConnections []FallbackTargetConnection `yaml:"target-connections"`
}

type CommandMessages struct {
	AlreadyConnected        string `yaml:"already-connected"`
	NoTargetConnectionFound string `yaml:"no-target-connection-found"`
}

type configs struct {
	Connection ConnectionConfig
	Messages   MessageConfig
	Commands   CommandConfig
}

type configStore struct {
	directory string
	mu        sync.RWMutex
	value     configs
}

func defaultConnectionConfig() ConnectionConfig {
	return ConnectionConfig{
		Version: configVersion,
		Registration: RegistrationConfig{
			Enabled:                                true,
			ServerNamePattern:                      "<group>-<numerical_id>",
			PersistentServerNamePattern:            "<name>",
			IgnoreServerGroupsAndPersistentServers: []string{},
			AdditionalServers:                      []RegistrationServer{},
		},
		Address: AddressConfig{Routes: []SubdomainRoute{}},
		Connections: []ConnectionEntry{{
			Name: "lobby",
			ServerNameMatcher: ServerMatcherConfiguration{
				Operation: OperationStartsWith,
				Value:     "lobby",
			},
			Rules: []ConnectionRule{},
		}},
		NetworkJoinTargets: NetworkJoinTargetsConfig{
			Enabled:           true,
			TargetConnections: []TargetConnection{{Name: "lobby", Priority: 0}},
		},
		Fallback: FallbackConfig{
			Enabled: true,
			TargetConnections: []FallbackTargetConnection{{
				Name: "lobby", Priority: 0, From: []string{},
			}},
		},
	}
}

func defaultMessageConfig() MessageConfig {
	return MessageConfig{
		Version:   configVersion,
		Variables: map[string]string{"prefix": "<color:#38bdf8><bold>⚡</bold></color>"},
		Kick: KickMessages{
			NoFallbackServers:  "<color:#dc2626>There is no fallback server available.",
			NoTargetConnection: "<color:#dc2626>You have been disconnected from the network<br>because there are no fallback servers available.",
		},
		Command: ConnectionCommandMessages{
			CommandUsage:          "<prefix> <color:#ffffff>Usage: /connection reload",
			ConfigReloading:       "<prefix> <color:#ffffff>Reloading Connection configurations...",
			ConfigReloadedSuccess: "<prefix> <color:#22c55e>Successfully reloaded all Connection configurations.",
			ConfigReloadedFailed:  "<prefix> <color:#dc2626>Failed to reload Connection configurations.",
		},
	}
}

func defaultCommandConfig() CommandConfig {
	return CommandConfig{
		Version: configVersion,
		Commands: []CommandEntry{{
			Name:       "lobby",
			Aliases:    []string{"l", "hub", "quit", "leave"},
			Permission: "",
			Messages: CommandMessages{
				AlreadyConnected:        "<color:#dc2626>You are already connected to this lobby!",
				NoTargetConnectionFound: "<color:#dc2626>Couldn't find a target server!",
			},
			TargetConnections: []FallbackTargetConnection{{
				Name: "lobby", Priority: 0, From: []string{},
			}},
		}},
	}
}

func configDir() string {
	if directory := strings.TrimSpace(os.Getenv("SIMPLECLOUD_GATE_CONFIG_DIR")); directory != "" {
		return directory
	}
	return defaultConfigDir
}

func newConfigStore(directory string) (*configStore, error) {
	store := &configStore{directory: directory}
	if err := store.reload(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *configStore) reload() error {
	loaded, err := loadConfigs(s.directory)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.value = loaded
	s.mu.Unlock()
	return nil
}

func (s *configStore) get() configs {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.value
}

func loadConfigs(directory string) (configs, error) {
	connection := defaultConnectionConfig()
	if err := loadOrCreateYAML(filepath.Join(directory, "config.yml"), &connection); err != nil {
		return configs{}, fmt.Errorf("load connection config: %w", err)
	}
	messages := defaultMessageConfig()
	if err := loadOrCreateYAML(filepath.Join(directory, "messages.yml"), &messages); err != nil {
		return configs{}, fmt.Errorf("load message config: %w", err)
	}
	commands := defaultCommandConfig()
	if err := loadOrCreateYAML(filepath.Join(directory, "commands.yml"), &commands); err != nil {
		return configs{}, fmt.Errorf("load command config: %w", err)
	}
	normalizeConfigs(&connection, &messages, &commands)
	if err := validateConfigs(connection, commands); err != nil {
		return configs{}, err
	}
	return configs{Connection: connection, Messages: messages, Commands: commands}, nil
}

func loadOrCreateYAML(path string, target any) error {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return writeYAML(path, target)
	}
	if err != nil {
		return err
	}
	if err := decodeConfigYAML(contents, target); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func writeYAML(path string, value any) error {
	contents, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, contents, 0o644)
}

func normalizeConfigs(connection *ConnectionConfig, messages *MessageConfig, commands *CommandConfig) {
	if connection.Registration.IgnoreServerGroupsAndPersistentServers == nil {
		connection.Registration.IgnoreServerGroupsAndPersistentServers = []string{}
	}
	if connection.Registration.AdditionalServers == nil {
		connection.Registration.AdditionalServers = []RegistrationServer{}
	}
	if connection.Address.Routes == nil {
		connection.Address.Routes = []SubdomainRoute{}
	}
	if connection.Connections == nil {
		connection.Connections = []ConnectionEntry{}
	}
	if connection.NetworkJoinTargets.TargetConnections == nil {
		connection.NetworkJoinTargets.TargetConnections = []TargetConnection{}
	}
	if connection.Fallback.TargetConnections == nil {
		connection.Fallback.TargetConnections = []FallbackTargetConnection{}
	}
	if messages.Variables == nil {
		messages.Variables = map[string]string{}
	}
	if commands.Commands == nil {
		commands.Commands = []CommandEntry{}
	}
	for i := range connection.Connections {
		if connection.Connections[i].ServerNameMatcher.Operation == "" {
			connection.Connections[i].ServerNameMatcher.Operation = OperationStartsWith
		}
		if connection.Connections[i].Rules == nil {
			connection.Connections[i].Rules = []ConnectionRule{}
		}
		for j := range connection.Connections[i].Rules {
			if connection.Connections[i].Rules[j].Type == "" {
				connection.Connections[i].Rules[j].Type = RulePermission
			}
			if connection.Connections[i].Rules[j].Operation == "" {
				connection.Connections[i].Rules[j].Operation = OperationEquals
			}
		}
	}
	for i := range commands.Commands {
		if commands.Commands[i].Aliases == nil {
			commands.Commands[i].Aliases = []string{}
		}
		if commands.Commands[i].TargetConnections == nil {
			commands.Commands[i].TargetConnections = []FallbackTargetConnection{}
		}
	}
}

func validateConfigs(connection ConnectionConfig, commands CommandConfig) error {
	if strings.TrimSpace(connection.Registration.ServerNamePattern) == "" {
		return errors.New("registration.serverNamePattern must not be empty")
	}
	if strings.TrimSpace(connection.Registration.PersistentServerNamePattern) == "" {
		return errors.New("registration.persistentServerNamePattern must not be empty")
	}

	registeredNames := map[string]struct{}{}
	for i, server := range connection.Registration.AdditionalServers {
		if !validation.ValidServerName(server.Name) {
			return fmt.Errorf("registration.additionalServers[%d].name is not a valid Gate server name", i)
		}
		if strings.TrimSpace(server.Address) == "" || server.Port < 1 || server.Port > 65535 {
			return fmt.Errorf("registration.additionalServers[%d] has an invalid address or port", i)
		}
		if _, _, err := net.SplitHostPort(net.JoinHostPort(server.Address, fmt.Sprint(server.Port))); err != nil {
			return fmt.Errorf("registration.additionalServers[%d] has an invalid address: %w", i, err)
		}
		name := strings.ToLower(server.Name)
		if _, duplicate := registeredNames[name]; duplicate {
			return fmt.Errorf("additional server name %q is configured more than once", server.Name)
		}
		registeredNames[name] = struct{}{}
	}

	connectionNames := map[string]struct{}{}
	for i, entry := range connection.Connections {
		name := strings.ToLower(strings.TrimSpace(entry.Name))
		if name == "" {
			return fmt.Errorf("connections[%d].name must not be empty", i)
		}
		if _, duplicate := connectionNames[name]; duplicate {
			return fmt.Errorf("connection name %q is configured more than once", entry.Name)
		}
		connectionNames[name] = struct{}{}
		if err := validateOperation(entry.ServerNameMatcher.Operation, entry.ServerNameMatcher.Value); err != nil {
			return fmt.Errorf("connections[%d].serverNameMatcher: %w", i, err)
		}
		for j, rule := range entry.Rules {
			if rule.Type != RulePermission && rule.Type != RuleEnv {
				return fmt.Errorf("connections[%d].rules[%d].type is invalid", i, j)
			}
			if err := validateOperation(rule.Operation, rule.Value); err != nil {
				return fmt.Errorf("connections[%d].rules[%d]: %w", i, j, err)
			}
		}
	}

	commandNames := map[string]struct{}{"connection": {}}
	for i, entry := range commands.Commands {
		allNames := append([]string{entry.Name}, entry.Aliases...)
		for _, configuredName := range allNames {
			name := strings.ToLower(strings.TrimSpace(configuredName))
			if name == "" || strings.ContainsAny(name, " \t\r\n") {
				return fmt.Errorf("commands[%d] contains an invalid command name %q", i, configuredName)
			}
			if _, duplicate := commandNames[name]; duplicate {
				return fmt.Errorf("command or alias %q is configured more than once", configuredName)
			}
			commandNames[name] = struct{}{}
		}
	}
	return nil
}

func validateOperation(operation OperationType, value string) error {
	switch operation {
	case OperationRegex:
		if _, err := regexp.Compile("^(?:" + value + ")$"); err != nil {
			return fmt.Errorf("invalid REGEX value: %w", err)
		}
	case OperationPattern, OperationEquals, OperationContains, OperationStartsWith, OperationEndsWith, OperationGreaterThan:
	default:
		return fmt.Errorf("unknown operation %q", operation)
	}
	return nil
}
