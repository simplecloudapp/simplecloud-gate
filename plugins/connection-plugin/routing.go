package connection

import (
	"github.com/robinbraemer/event"
	"go.minekube.com/gate/pkg/edition/java/proxy"
	"go.minekube.com/gate/pkg/util/netutil"
)

func (a *application) registerListeners() {
	event.Subscribe(a.gate.Event(), 0, a.onPlayerChooseInitialServer)
	event.Subscribe(a.gate.Event(), 0, a.onKickedFromServer)
}

func (a *application) onPlayerChooseInitialServer(event *proxy.PlayerChooseInitialServerEvent) {
	loaded := a.configs.get()
	config := loaded.Connection
	player := event.Player()
	servers := a.gate.Servers()

	if virtualHost := player.VirtualHost(); virtualHost != nil {
		host := netutil.Host(virtualHost)
		for _, route := range config.Address.Routes {
			if route.Subdomain != host {
				continue
			}
			connection := findConnection(route.TargetConnection, config.Connections)
			if connection != nil {
				if server := leastPopulated(matchingServers(*connection, servers), ""); server != nil {
					event.SetInitialServer(server)
					return
				}
			}
			break
		}
	}

	if !config.NetworkJoinTargets.Enabled {
		return
	}
	for _, target := range sortedJoinTargets(config.NetworkJoinTargets.TargetConnections) {
		connection := findConnection(target.Name, config.Connections)
		if connection == nil || failedRule(*connection, player.HasPermission) != nil {
			continue
		}
		if server := leastPopulated(matchingServers(*connection, servers), ""); server != nil {
			event.SetInitialServer(server)
			return
		}
	}

	event.SetInitialServer(nil)
	player.Disconnect(messageComponent(loaded.Messages.Kick.NoTargetConnection, loaded.Messages.Variables))
}

func (a *application) onKickedFromServer(event *proxy.KickedFromServerEvent) {
	loaded := a.configs.get()
	config := loaded.Connection
	if !config.Fallback.Enabled {
		return
	}

	servers := a.gate.Servers()
	kickedServerName := event.Server().ServerInfo().Name()
	for _, target := range sortedTargets(config.Fallback.TargetConnections) {
		if len(target.From) > 0 {
			allowed := false
			for _, connectionName := range target.From {
				if isServerInConnection(kickedServerName, connectionName, config.Connections, servers) {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}

		connection := findConnection(target.Name, config.Connections)
		if connection == nil || failedRule(*connection, event.Player().HasPermission) != nil {
			continue
		}
		if server := leastPopulated(matchingServers(*connection, servers), kickedServerName); server != nil {
			event.SetResult(&proxy.RedirectPlayerKickResult{Server: server})
			return
		}
	}

	event.SetResult(&proxy.DisconnectPlayerKickResult{
		Reason: messageComponent(loaded.Messages.Kick.NoFallbackServers, loaded.Messages.Variables),
	})
}
