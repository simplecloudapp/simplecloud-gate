package connection

import (
	"context"
	"strings"
	"time"

	"go.minekube.com/brigodier"
	"go.minekube.com/gate/pkg/command"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

const reloadPermission = "simplecloud.connection.reload"

func (a *application) registerCommands(config CommandConfig) {
	for _, entry := range config.Commands {
		configured := entry
		builder := brigodier.Literal(configured.Name).
			Requires(command.Requires(func(ctx *command.RequiresContext) bool {
				return configured.Permission == "" || ctx.Source.HasPermission(configured.Permission)
			})).
			Executes(command.Command(func(ctx *command.Context) error {
				return a.executeConnectionCommand(ctx, configured)
			}))
		a.gate.Command().RegisterWithAliases(builder, configured.Aliases...)
	}

	usage := func(ctx *command.Context) error {
		messages := a.configs.get().Messages
		return ctx.Source.SendMessage(messageComponent(messages.Command.CommandUsage, messages.Variables))
	}
	reload := func(ctx *command.Context) error {
		messages := a.configs.get().Messages
		if err := ctx.Source.SendMessage(messageComponent(messages.Command.ConfigReloading, messages.Variables)); err != nil {
			return err
		}
		if err := a.configs.reload(); err != nil {
			a.log.Error(err, "failed to reload SimpleCloud connection configurations")
			messages = a.configs.get().Messages
			return ctx.Source.SendMessage(messageComponent(messages.Command.ConfigReloadedFailed, messages.Variables))
		}
		messages = a.configs.get().Messages
		return ctx.Source.SendMessage(messageComponent(messages.Command.ConfigReloadedSuccess, messages.Variables))
	}
	execute := command.Command(func(ctx *command.Context) error {
		arguments := strings.Fields(ctx.String("arguments"))
		if len(arguments) == 0 || !strings.EqualFold(arguments[0], "reload") {
			return usage(ctx)
		}
		return reload(ctx)
	})

	builder := brigodier.Literal("connection").
		Requires(command.Requires(func(ctx *command.RequiresContext) bool {
			return ctx.Source.HasPermission(reloadPermission)
		})).
		Executes(command.Command(usage)).
		Then(brigodier.Argument("arguments", brigodier.StringPhrase).Executes(execute))
	a.gate.Command().Register(builder)
}

func (a *application) executeConnectionCommand(ctx *command.Context, configured CommandEntry) error {
	player, ok := ctx.Source.(proxy.Player)
	if !ok {
		a.log.Info("ignoring connection command because source is not a player", "command", configured.Name)
		return nil
	}

	loaded := a.configs.get()
	connections := loaded.Connection.Connections
	servers := a.gate.Servers()
	currentServerName := ""
	if current := player.CurrentServer(); current != nil {
		currentServerName = current.Server().ServerInfo().Name()
	}

	for _, target := range sortedTargets(configured.TargetConnections) {
		if len(target.From) > 0 && currentServerName != "" {
			allowed := false
			for _, connectionName := range target.From {
				if isServerInConnection(currentServerName, connectionName, connections, servers) {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}

		connection := findConnection(target.Name, connections)
		if connection == nil || failedRule(*connection, player.HasPermission) != nil {
			continue
		}
		server := leastPopulated(matchingServers(*connection, servers), "")
		if server == nil {
			continue
		}
		if currentServerName != "" && server.ServerInfo().Name() == currentServerName {
			return player.SendMessage(messageComponent(configured.Messages.AlreadyConnected, loaded.Messages.Variables))
		}

		a.log.Info("sending player through connection command",
			"player", player.Username(), "command", configured.Name,
			"from", currentServerName, "server", server.ServerInfo().Name(),
			"connection", connection.Name)
		go a.connectPlayer(player, server, configured.Name)
		return nil
	}

	a.log.Info("connection command found no usable target", "player", player.Username(), "command", configured.Name, "currentServer", currentServerName)
	return player.SendMessage(messageComponent(configured.Messages.NoTargetConnectionFound, loaded.Messages.Variables))
}

func (a *application) connectPlayer(player proxy.Player, server proxy.RegisteredServer, commandName string) {
	ctx, cancel := context.WithTimeout(player.Context(), 15*time.Second)
	defer cancel()
	result, err := player.CreateConnectionRequest(server).Connect(ctx)
	if err != nil {
		a.log.Error(err, "connection command failed", "player", player.Username(), "command", commandName, "server", server.ServerInfo().Name())
		return
	}
	if !result.Status().Successful() {
		a.log.Info("connection command did not connect player", "player", player.Username(), "command", commandName, "server", server.ServerInfo().Name(), "status", result.Status())
		return
	}
	a.log.Info("connection command connected player", "player", player.Username(), "command", commandName, "server", server.ServerInfo().Name())
}
