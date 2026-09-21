package connection

import (
	"testing"

	controllerv2 "buf.build/gen/go/simplecloud/controller/protocolbuffers/go"
)

func TestCloudServerFromEvent(t *testing.T) {
	groupBase := &controllerv2.BaseServerConfig{
		Name:       "Lobby",
		Type:       controllerv2.ServerType_SERVER_TYPE_SERVER,
		Properties: map[string]string{"region": "global"},
	}
	config := &controllerv2.BaseServerConfig{
		Type:       controllerv2.ServerType_SERVER_TYPE_SERVER,
		Properties: map[string]string{"region": "eu"},
		SourceConfig: controllerv2.SourceConfig_builder{
			Blueprint: &controllerv2.BlueprintConfig{Configurator: "paper"},
		}.Build(),
	}
	event := &controllerv2.ServerStartedEvent{
		ServerId:    "server-id",
		Config:      config,
		GroupConfig: &controllerv2.ServerGroupConfig{BaseConfig: groupBase},
		RuntimeInfo: &controllerv2.ServerRuntimeInfo{
			Ip:          "10.0.0.1",
			Port:        25565,
			NumericalId: 4,
		},
	}

	server, err := cloudServerFromEvent(event)
	if err != nil {
		t.Fatal(err)
	}
	if server.id != "server-id" || server.baseName != "Lobby" || server.typeName != "SERVER" {
		t.Fatalf("unexpected event conversion: %#v", server)
	}
	if server.ip != "10.0.0.1" || server.port != 25565 || server.numericalID != 4 {
		t.Fatalf("unexpected runtime conversion: %#v", server)
	}
	if server.properties["region"] != "eu" || server.configurator != "paper" {
		t.Fatalf("effective configuration was not merged: %#v", server)
	}
}
