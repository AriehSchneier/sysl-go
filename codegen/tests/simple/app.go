// Code generated by sysl DO NOT EDIT.
package simple

import (
	"github.com/anz-bank/sysl-go/codegen/tests/deps"
	"github.com/anz-bank/sysl-go/codegen/tests/downstream"
	"github.com/anz-bank/sysl-go/config"
	"github.com/anz-bank/sysl-go/core"
	"github.com/anz-bank/sysl-go/handlerinitialiser"
)

// App for Simple
type App struct {
}

// HandlerManager for Simple
type HandlerManager struct {
	coreCfg             *config.DefaultConfig
	enabledHandlers     []handlerinitialiser.RestHandlerInitialiser
	enabledGrpcHandlers []handlerinitialiser.GrpcHandlerInitialiser
}

// InitialiseHandler ...
func (a *App) InitialiseHandler(h *HandlerManager, serviceInterface ServiceInterface, serviceCallback core.RestGenCallback) error {
	var err error = nil
	depsHTTPClient, depsErr := core.BuildDownstreamHTTPClient("deps", &h.coreCfg.GenCode.Downstream.(*DownstreamConfig).Deps)
	downstreamHTTPClient, downstreamErr := core.BuildDownstreamHTTPClient("downstream", &h.coreCfg.GenCode.Downstream.(*DownstreamConfig).Downstream)
	if depsErr != nil {
		return depsErr
	}

	if downstreamErr != nil {
		return downstreamErr
	}

	depsClient := deps.NewClient(depsHTTPClient, h.coreCfg.GenCode.Downstream.(*DownstreamConfig).Deps.ServiceURL)
	downstreamClient := downstream.NewClient(downstreamHTTPClient, h.coreCfg.GenCode.Downstream.(*DownstreamConfig).Downstream.ServiceURL)
	serviceHandler := NewServiceHandler(serviceCallback, &serviceInterface, depsClient, downstreamClient)
	serviceRouter := NewServiceRouter(serviceCallback, serviceHandler)
	h.enabledHandlers = append(h.enabledHandlers, serviceRouter)
	return err
}
