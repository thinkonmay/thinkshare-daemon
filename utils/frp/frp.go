package proxy

import (
	"context"
	"fmt"

	"github.com/fatedier/frp/client"
	"github.com/fatedier/frp/pkg/config/types"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/fatedier/frp/pkg/config/v1/validation"
	"github.com/fatedier/frp/server"
)

func Server() {
	config := &v1.ServerConfig{
		Auth: v1.AuthServerConfig{
			Method: "token",
			Token:  "test",
		},
		Log: v1.LogConfig{
			Level: "info",
		},
		Transport: v1.ServerTransportConfig{
			QUIC: v1.QUICOptions{
				10, 300, 10000,
			},
		},
		AllowPorts: []types.PortsRange{
			{
				Start: 60001,
				End:   65535,
			},
		},
		QUICBindPort: 60000,
	}
	w, err := validation.ValidateServerConfig(config)
	if err != nil {
		panic(err)
	}
	if w != nil {
		fmt.Printf("%s\n", w.Error())
	}
	service, err := server.NewService(config)
	if err != nil {
		panic(err)
	}
	service.Run(context.Background())
}

func Client() {
	config := client.ServiceOptions{
		Common: &v1.ClientCommonConfig{
			ServerAddr: "localhost",
			ServerPort: 60000,
			Auth: v1.AuthClientConfig{
				Method: "token",
				Token:  "test",
			},
			Log: v1.LogConfig{
				Level: "info",
			},
			Transport: v1.ClientTransportConfig{
				Protocol: "quic",
				QUIC: v1.QUICOptions{
					10, 300, 10000,
				},
			},
		},
		ProxyCfgs:   []v1.ProxyConfigurer{},
		VisitorCfgs: []v1.VisitorConfigurer{},
	}
	w, err := validation.ValidateAllClientConfig(config.Common, config.ProxyCfgs, config.VisitorCfgs)
	if err != nil {
		panic(err)
	}
	if w != nil {
		fmt.Printf("%s\n", w.Error())
	}
	service, err := client.NewService(config)
	if err != nil {
		panic(err)
	}
	service.Run(context.Background())
}
