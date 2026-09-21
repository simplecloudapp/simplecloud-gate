package connection

import (
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

func TestResolveServerName(t *testing.T) {
	server := cloudServer{
		id:          "server-id",
		baseName:    "Lobby",
		numericalID: 3,
		properties:  map[string]any{"region-name": "eu", "literal": "<not-a-tag>"},
	}
	name := resolveServerName(server, RegistrationConfig{
		ServerNamePattern:           "<group>-<numerical_id>-<region_name>-<id>",
		PersistentServerNamePattern: "<name>",
	})
	if name != "Lobby-3-eu-server-id" {
		t.Fatalf("unexpected resolved name %q", name)
	}
	if name := resolveServerName(server, RegistrationConfig{ServerNamePattern: "<literal>"}); name != "<not-a-tag>" {
		t.Fatalf("placeholder value was parsed as MiniMessage: %q", name)
	}
	server.persistent = true
	if name := resolveServerName(server, RegistrationConfig{PersistentServerNamePattern: "<name>"}); name != "Lobby" {
		t.Fatalf("unexpected persistent name %q", name)
	}
}

func TestRegistrarLifecycle(t *testing.T) {
	registry := newFakeRegistry()
	config := defaultConnectionConfig().Registration
	registrar := newRegistrar(registry, func() RegistrationConfig { return config }, logr.Discard())
	server := testCloudServer()

	added, err := registrar.register(server)
	if err != nil {
		t.Fatal(err)
	}
	if !added || registry.Server("Lobby-1") == nil {
		t.Fatal("server was not registered")
	}

	server.ip = "127.0.0.2"
	added, err = registrar.register(server)
	if err != nil {
		t.Fatal(err)
	}
	if !added || registry.Server("Lobby-1").ServerInfo().Addr().String() != "127.0.0.2:25565" {
		t.Fatal("changed server address was not refreshed")
	}

	if !registrar.unregister(server.id) || registry.Server("Lobby-1") != nil {
		t.Fatal("server was not unregistered")
	}
}

func TestRegistrarHonorsFilters(t *testing.T) {
	config := defaultConnectionConfig().Registration
	config.IgnoreServerGroupsAndPersistentServers = []string{"Ignored"}
	registry := newFakeRegistry()
	registrar := newRegistrar(registry, func() RegistrationConfig { return config }, logr.Discard())

	ignored := testCloudServer()
	ignored.baseName = "ignored"
	if added, err := registrar.register(ignored); err != nil || added {
		t.Fatalf("ignored group was registered: added=%v err=%v", added, err)
	}

	standalone := testCloudServer()
	standalone.id = "standalone-id"
	standalone.configurator = "standalone"
	if added, err := registrar.register(standalone); err != nil || added {
		t.Fatalf("standalone server was registered: added=%v err=%v", added, err)
	}

	if len(registry.Servers()) != 0 {
		t.Fatal("filtered servers reached the Gate registry")
	}
}

func TestRegistrarResetReplacesConfiguredServers(t *testing.T) {
	registry := newFakeRegistry()
	oldAddress, err := backendAddress("127.0.0.1", 25565)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Register(proxy.NewServerInfo("old", oldAddress)); err != nil {
		t.Fatal(err)
	}

	config := defaultConnectionConfig().Registration
	registrar := newRegistrar(registry, func() RegistrationConfig { return config }, logr.Discard())
	if err := registrar.reset([]RegistrationServer{{Name: "external", Address: "localhost", Port: 25566}}); err != nil {
		t.Fatal(err)
	}
	if registry.Server("old") != nil || registry.Server("external") == nil {
		t.Fatal("reset did not replace Gate's configured servers")
	}
}

func testCloudServer() cloudServer {
	return cloudServer{
		id:          "server-id",
		baseName:    "Lobby",
		typeName:    "SERVER",
		ip:          "127.0.0.1",
		port:        25565,
		numericalID: 1,
		properties:  map[string]any{},
	}
}

type fakeRegistry struct {
	servers map[string]proxy.RegisteredServer
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{servers: make(map[string]proxy.RegisteredServer)}
}

func (r *fakeRegistry) Server(name string) proxy.RegisteredServer {
	return r.servers[strings.ToLower(name)]
}

func (r *fakeRegistry) Servers() []proxy.RegisteredServer {
	servers := make([]proxy.RegisteredServer, 0, len(r.servers))
	for _, server := range r.servers {
		servers = append(servers, server)
	}
	return servers
}

func (r *fakeRegistry) Register(info proxy.ServerInfo) (proxy.RegisteredServer, error) {
	key := strings.ToLower(info.Name())
	if existing := r.servers[key]; existing != nil {
		return existing, proxy.ErrServerAlreadyExists
	}
	server := &fakeRegisteredServer{info: info, players: fakePlayers{}}
	r.servers[key] = server
	return server, nil
}

func (r *fakeRegistry) Unregister(info proxy.ServerInfo) bool {
	key := strings.ToLower(info.Name())
	existing := r.servers[key]
	if existing == nil || !proxy.ServerInfoEqual(existing.ServerInfo(), info) {
		return false
	}
	delete(r.servers, key)
	return true
}

type fakeRegisteredServer struct {
	info    proxy.ServerInfo
	players fakePlayers
}

func (s *fakeRegisteredServer) ServerInfo() proxy.ServerInfo { return s.info }
func (s *fakeRegisteredServer) Players() proxy.Players       { return s.players }

type fakePlayers struct{ count int }

func (p fakePlayers) Len() int                      { return p.count }
func (p fakePlayers) Range(func(proxy.Player) bool) {}

var _ serverRegistry = (*fakeRegistry)(nil)
var _ proxy.RegisteredServer = (*fakeRegisteredServer)(nil)
