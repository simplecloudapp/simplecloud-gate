package connection

import (
	"fmt"

	controllerv2 "buf.build/gen/go/simplecloud/controller/protocolbuffers/go"
)

type serverSnapshotEvent interface {
	GetServerId() string
	GetConfig() *controllerv2.BaseServerConfig
	GetRuntimeInfo() *controllerv2.ServerRuntimeInfo
	GetGroupConfig() *controllerv2.ServerGroupConfig
	GetPersistentServerConfig() *controllerv2.PersistentServerConfig
}

func cloudServerFromEvent(event serverSnapshotEvent) (cloudServer, error) {
	config := event.GetConfig()
	runtime := event.GetRuntimeInfo()
	if config == nil {
		return cloudServer{}, fmt.Errorf("SimpleCloud server event %s has no config", event.GetServerId())
	}
	if runtime == nil {
		return cloudServer{}, fmt.Errorf("SimpleCloud server event %s has no runtime info", event.GetServerId())
	}

	baseConfig := config
	baseName := ""
	persistent := false
	if groupConfig := event.GetGroupConfig(); groupConfig != nil && groupConfig.GetBaseConfig() != nil {
		baseConfig = groupConfig.GetBaseConfig()
		baseName = baseConfig.GetName()
	} else if persistentConfig := event.GetPersistentServerConfig(); persistentConfig != nil && persistentConfig.GetBaseConfig() != nil {
		baseConfig = persistentConfig.GetBaseConfig()
		baseName = baseConfig.GetName()
		persistent = true
	}
	if baseName == "" {
		baseName = config.GetName()
	}

	properties := make(map[string]any)
	for key, value := range baseConfig.GetProperties() {
		properties[key] = value
	}
	for key, value := range config.GetProperties() {
		properties[key] = value
	}

	typeName := config.GetType().String()
	if config.GetType() == controllerv2.ServerType_SERVER_TYPE_UNSPECIFIED {
		typeName = baseConfig.GetType().String()
	}
	typeName = enumSuffix(typeName)

	serverConfigurator := sourceConfigurator(config)
	if serverConfigurator == "" {
		serverConfigurator = sourceConfigurator(baseConfig)
	}
	if serverConfigurator == "" {
		serverConfigurator = stringProperty(properties, "configurator")
	}

	serverID := event.GetServerId()
	if serverID == "" {
		serverID = runtime.GetServerId()
	}
	return cloudServer{
		id:           serverID,
		baseName:     baseName,
		typeName:     typeName,
		ip:           runtime.GetIp(),
		port:         runtime.GetPort(),
		numericalID:  runtime.GetNumericalId(),
		properties:   properties,
		configurator: serverConfigurator,
		persistent:   persistent,
	}, nil
}

func sourceConfigurator(config *controllerv2.BaseServerConfig) string {
	if config == nil || config.GetSourceConfig() == nil || config.GetSourceConfig().GetBlueprint() == nil {
		return ""
	}
	return config.GetSourceConfig().GetBlueprint().GetConfigurator()
}

func enumSuffix(value string) string {
	const prefix = "SERVER_TYPE_"
	if len(value) >= len(prefix) && value[:len(prefix)] == prefix {
		return value[len(prefix):]
	}
	return value
}
