package main

import (
	connection "github.com/simplecloudapp/simplecloud-gate/plugins/connection-plugin"
	"go.minekube.com/gate/cmd/gate"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

func main() {
	proxy.Plugins = append(proxy.Plugins, connection.Plugin)
	gate.Execute()
}
