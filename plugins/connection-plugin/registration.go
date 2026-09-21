package connection

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	cloudapi "github.com/simplecloudapp/cloud-api/go"
	"go.minekube.com/gate/pkg/edition/java/proxy"
	"go.minekube.com/gate/pkg/util/netutil"
)

var (
	invalidPlaceholderCharacters = regexp.MustCompile(`[^a-z0-9_]`)
	miniTagPattern               = regexp.MustCompile(`<[^>]+>`)
)

type serverRegistry interface {
	Server(string) proxy.RegisteredServer
	Servers() []proxy.RegisteredServer
	Register(proxy.ServerInfo) (proxy.RegisteredServer, error)
	Unregister(proxy.ServerInfo) bool
}

type cloudServer struct {
	id           string
	baseName     string
	typeName     string
	ip           string
	port         int32
	numericalID  int32
	properties   map[string]any
	configurator string
	persistent   bool
}

type registration struct {
	server cloudServer
	info   proxy.ServerInfo
}

type registrar struct {
	registry           serverRegistry
	registrationConfig func() RegistrationConfig
	log                logr.Logger

	mu         sync.Mutex
	registered map[string]registration
}

func newRegistrar(registry serverRegistry, registrationConfig func() RegistrationConfig, log logr.Logger) *registrar {
	return &registrar{
		registry:           registry,
		registrationConfig: registrationConfig,
		log:                log,
		registered:         make(map[string]registration),
	}
}

func (r *registrar) reset(additional []RegistrationServer) error {
	for _, server := range r.registry.Servers() {
		r.registry.Unregister(server.ServerInfo())
	}

	for _, server := range additional {
		address, err := backendAddress(server.Address, int32(server.Port))
		if err != nil {
			return fmt.Errorf("parse additional server %q: %w", server.Name, err)
		}
		if _, err := r.registry.Register(proxy.NewServerInfo(server.Name, address)); err != nil {
			return fmt.Errorf("register additional server %q: %w", server.Name, err)
		}
		r.log.Info("registered additional server", "name", server.Name, "address", address.String())
	}
	return nil
}

func (r *registrar) register(server cloudServer) (bool, error) {
	if err := validateCloudServer(server); err != nil {
		return false, err
	}
	config := r.registrationConfig()
	if ignoredServer(server, config) {
		return false, nil
	}

	name := resolveServerName(server, config)
	address, err := backendAddress(server.ip, server.port)
	if err != nil {
		return false, err
	}
	info := proxy.NewServerInfo(name, address)

	r.mu.Lock()
	defer r.mu.Unlock()

	previous, hadPrevious := r.registered[server.id]
	if hadPrevious && previous.info.Name() != name {
		r.registry.Unregister(previous.info)
		delete(r.registered, server.id)
		r.log.Info("replacing stale SimpleCloud registration", "id", server.id, "oldName", previous.info.Name(), "name", name)
	} else if hadPrevious {
		r.log.Info("refreshing SimpleCloud server", "id", server.id, "name", name)
	} else {
		r.log.Info("registering SimpleCloud server", "id", server.id, "name", name)
	}

	// Velocity's registry replaces any registration with the same proxy name.
	if existing := r.registry.Server(name); existing != nil {
		r.registry.Unregister(existing.ServerInfo())
	}
	if _, err := r.registry.Register(info); err != nil {
		if hadPrevious {
			if _, rollbackErr := r.registry.Register(previous.info); rollbackErr == nil {
				r.registered[server.id] = previous
			}
		}
		return false, fmt.Errorf("register %s as %s: %w", server.id, name, err)
	}

	r.registered[server.id] = registration{server: server, info: info}
	r.log.Info("registered SimpleCloud server", "id", server.id, "name", name, "address", address.String())
	return true, nil
}

func (r *registrar) unregister(serverID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	registered, ok := r.registered[serverID]
	if !ok {
		return false
	}
	delete(r.registered, serverID)
	removed := r.registry.Unregister(registered.info)
	r.log.Info("unregistered SimpleCloud server", "id", serverID, "name", registered.info.Name())
	return removed
}

func validateCloudServer(server cloudServer) error {
	switch {
	case strings.TrimSpace(server.id) == "":
		return errors.New("SimpleCloud server has no ID")
	case strings.TrimSpace(server.baseName) == "":
		return fmt.Errorf("SimpleCloud server %s has no base name", server.id)
	case !strings.EqualFold(server.typeName, "SERVER"):
		return fmt.Errorf("SimpleCloud server %s is not a game server", server.id)
	case strings.TrimSpace(server.ip) == "":
		return fmt.Errorf("SimpleCloud server %s has no IP address", server.id)
	case server.port < 1 || server.port > 65535:
		return fmt.Errorf("SimpleCloud server %s has invalid port %d", server.id, server.port)
	}
	return nil
}

func ignoredServer(server cloudServer, config RegistrationConfig) bool {
	if server.configurator == "standalone" {
		return true
	}
	for _, name := range config.IgnoreServerGroupsAndPersistentServers {
		if strings.EqualFold(strings.TrimSpace(name), server.baseName) {
			return true
		}
	}
	return false
}

func resolveServerName(server cloudServer, config RegistrationConfig) string {
	pattern := config.ServerNamePattern
	if server.persistent {
		pattern = config.PersistentServerNamePattern
	}
	placeholders := map[string]string{
		"group":        server.baseName,
		"name":         server.baseName,
		"numerical_id": fmt.Sprint(server.numericalID),
		"id":           server.id,
	}
	for key, value := range server.properties {
		name := invalidPlaceholderCharacters.ReplaceAllString(strings.ToLower(key), "_")
		placeholders[name] = fmt.Sprint(value)
	}
	// The source plugin uses unparsed placeholders and serializes the resulting
	// MiniMessage component as plain text. Replace tags from the original
	// pattern in one pass so markup in a property value remains literal.
	return miniTagPattern.ReplaceAllStringFunc(pattern, func(tag string) string {
		name := strings.TrimSuffix(strings.TrimPrefix(tag, "<"), ">")
		if value, ok := placeholders[name]; ok {
			return value
		}
		return ""
	})
}

func backendAddress(host string, port int32) (net.Addr, error) {
	return netutil.Parse(net.JoinHostPort(host, fmt.Sprint(port)), "tcp")
}

func cloudServerFromSummary(server cloudapi.Server) (cloudServer, error) {
	result := cloudServer{
		id:          server.GetServerId(),
		ip:          server.GetIp(),
		port:        server.GetPort(),
		numericalID: server.GetNumericalId(),
		properties:  make(map[string]any),
	}

	if group := server.ServerGroup; group != nil {
		result.baseName = group.GetName()
		result.typeName = group.GetType()
		mergeProperties(result.properties, group.GetProperties())
		if source := group.Source; source != nil && source.Blueprint != nil {
			result.configurator = source.Blueprint.GetConfigurator()
		}
	} else if persistent := server.PersistentServer; persistent != nil {
		result.baseName = persistent.GetName()
		result.typeName = persistent.GetType()
		result.persistent = true
		mergeProperties(result.properties, persistent.GetProperties())
	}
	mergeProperties(result.properties, server.GetProperties())
	if result.configurator == "" {
		result.configurator = stringProperty(result.properties, "configurator")
	}
	return result, nil
}

func mergeProperties(destination map[string]any, properties map[string]any) {
	for key, value := range properties {
		destination[key] = value
	}
}

func stringProperty(properties map[string]any, key string) string {
	value, ok := properties[key]
	if !ok || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
