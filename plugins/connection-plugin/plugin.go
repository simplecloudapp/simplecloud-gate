package connection

import (
	"context"
	"fmt"
	"time"

	controllerv2 "buf.build/gen/go/simplecloud/controller/protocolbuffers/go"
	"github.com/go-logr/logr"
	"github.com/nats-io/nats.go"
	"github.com/robinbraemer/event"
	cloudapi "github.com/simplecloudapp/cloud-api/go"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

const (
	initialSyncRetry = 5 * time.Second
	serverPageSize   = int32(500)
)

var Plugin = proxy.Plugin{
	Name: "SimpleCloud Connection",
	Init: initialize,
}

type application struct {
	ctx       context.Context
	gate      *proxy.Proxy
	configs   *configStore
	client    *cloudapi.Client
	registrar *registrar
	log       logr.Logger
}

func initialize(ctx context.Context, gate *proxy.Proxy) error {
	log := logr.FromContextOrDiscard(ctx).WithName("simplecloud-connection")
	configStore, err := newConfigStore(configDir())
	if err != nil {
		return err
	}

	app := &application{ctx: ctx, gate: gate, configs: configStore, log: log}
	app.registrar = newRegistrar(gate, func() RegistrationConfig {
		return app.configs.get().Connection.Registration
	}, log)

	loaded := configStore.get()
	if loaded.Connection.Registration.Enabled {
		if err := app.registrar.reset(loaded.Connection.Registration.AdditionalServers); err != nil {
			return err
		}

		options := cloudapi.DefaultOptions()
		if options.Component == "" {
			options.Component = "simplecloud-gate"
		}
		client, err := cloudapi.NewClient(options)
		if err != nil {
			return fmt.Errorf("create SimpleCloud client: %w", err)
		}
		app.client = client
		client.Events.SetErrorHandler(func(subject string, err error) {
			log.Error(err, "could not decode SimpleCloud event", "subject", subject)
		})
	}

	app.registerListeners()
	app.registerCommands(loaded.Commands)
	event.Subscribe(gate.Event(), 0, func(*proxy.ShutdownEvent) {
		if app.client != nil {
			if err := app.client.Close(); err != nil {
				log.Error(err, "could not close SimpleCloud client")
			}
		}
	})
	if app.client != nil {
		go app.run()
	}
	log.Info("SimpleCloud connection plugin initialized")
	return nil
}

func (a *application) run() {
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
		}

		if err := a.subscribe(); err != nil {
			a.log.Error(err, "could not subscribe to SimpleCloud server events; retrying")
			if !a.waitForRetry() {
				return
			}
			continue
		}
		a.syncInitialServers()
		return
	}
}

func (a *application) subscribe() error {
	subscriptions := make([]*nats.Subscription, 0, 2)
	subscribe := func(subscription *nats.Subscription, err error) error {
		if err != nil {
			for _, existing := range subscriptions {
				_ = existing.Unsubscribe()
			}
			return err
		}
		subscriptions = append(subscriptions, subscription)
		return nil
	}

	if err := subscribe(a.client.Events.OnServerStateChanged(a.onServerStateChanged)); err != nil {
		return fmt.Errorf("subscribe to SimpleCloud server-state events: %w", err)
	}
	if err := subscribe(a.client.Events.OnServerStopped(func(event *cloudapi.ServerStoppedEvent) {
		a.registrar.unregister(event.GetServerId())
	})); err != nil {
		return fmt.Errorf("subscribe to SimpleCloud server-stop events: %w", err)
	}
	return nil
}

func (a *application) onServerStateChanged(event *cloudapi.ServerStateChangedEvent) {
	if event.GetNewState() != controllerv2.ServerState_SERVER_STATE_AVAILABLE ||
		event.GetOldState() == controllerv2.ServerState_SERVER_STATE_AVAILABLE {
		return
	}
	server, err := cloudServerFromEvent(event)
	if err != nil {
		a.log.Error(err, "could not read SimpleCloud server-state event")
		return
	}
	if !stringsEqualServerType(server.typeName) {
		return
	}
	if _, err := a.registrar.register(server); err != nil {
		a.log.Error(err, "could not register available SimpleCloud server", "id", event.GetServerId())
	}
}

func stringsEqualServerType(serverType string) bool {
	return serverType == "SERVER"
}

func (a *application) syncInitialServers() {
	for {
		if err := a.syncInitialServersOnce(); err == nil {
			return
		} else {
			a.log.Error(err, "could not perform initial SimpleCloud server sync; retrying")
		}
		if !a.waitForRetry() {
			return
		}
	}
}

func (a *application) waitForRetry() bool {
	timer := time.NewTimer(initialSyncRetry)
	defer timer.Stop()
	select {
	case <-a.ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (a *application) syncInitialServersOnce() error {
	registered := 0
	for offset := int32(0); ; offset += serverPageSize {
		limit := serverPageSize
		pageOffset := offset
		servers, _, err := a.client.Servers.List(a.ctx, &cloudapi.ServerQuery{
			States: []string{"AVAILABLE"},
			Types:  []string{"SERVER"},
			Limit:  &limit,
			Offset: &pageOffset,
		})
		if err != nil {
			return err
		}

		for _, summary := range servers {
			server, err := cloudServerFromSummary(summary)
			if err != nil {
				a.log.Error(err, "could not read SimpleCloud server from initial sync")
				continue
			}
			added, err := a.registrar.register(server)
			if err != nil {
				a.log.Error(err, "could not register SimpleCloud server from initial sync", "id", server.id)
				continue
			}
			if added {
				registered++
			}
		}
		if int32(len(servers)) < serverPageSize {
			break
		}
	}
	a.log.Info("completed initial SimpleCloud server sync", "registered", registered)
	return nil
}
