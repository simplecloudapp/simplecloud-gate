package connection

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigsCreatesDefaults(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "nested")
	loaded, err := loadConfigs(directory)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Connection.Registration.ServerNamePattern != "<group>-<numerical_id>" {
		t.Fatalf("unexpected default pattern %q", loaded.Connection.Registration.ServerNamePattern)
	}
	for _, name := range []string{"config.yml", "messages.yml", "commands.yml"} {
		if _, err := os.Stat(filepath.Join(directory, name)); err != nil {
			t.Fatalf("default %s was not written: %v", name, err)
		}
	}
}

func TestValidateConfigsRejectsDuplicateAdditionalServers(t *testing.T) {
	config := defaultConnectionConfig()
	config.Registration.AdditionalServers = []RegistrationServer{
		{Name: "Lobby", Address: "localhost", Port: 25565},
		{Name: "lobby", Address: "localhost", Port: 25566},
	}
	if err := validateConfigs(config, defaultCommandConfig()); err == nil {
		t.Fatal("expected duplicate server names to be rejected")
	}
}

func TestDefaultConfigsMatchConnectionPlugin(t *testing.T) {
	connection := defaultConnectionConfig()
	if !connection.NetworkJoinTargets.Enabled || !connection.Fallback.Enabled {
		t.Fatal("join targets and fallback should be enabled by default")
	}
	commands := defaultCommandConfig()
	if len(commands.Commands) != 1 || commands.Commands[0].Name != "lobby" {
		t.Fatalf("unexpected default commands: %#v", commands.Commands)
	}
}

func TestCommittedExampleConfigsLoad(t *testing.T) {
	directory := filepath.Join("..", "..", "simplecloud-connection")
	loaded, err := loadConfigs(directory)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Connection.Version != configVersion || loaded.Messages.Version != configVersion || loaded.Commands.Version != configVersion {
		t.Fatalf("unexpected example config versions: %#v", loaded)
	}
}
