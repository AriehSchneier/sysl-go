// Code generated by sysl DO NOT EDIT.
package simple

import (
	"github.com/anz-bank/sysl-go/codegen/tests/deps"
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
	enabledHandlers     []handlerinitialiser.HandlerInitialiser
	enabledGrpcHandlers []handlerinitialiser.GrpcHandlerInitialiser
}

// LoadServices ...
func (a *App) InitialiseHandler(h *HandlerManager, serviceInterface ServiceInterface, serviceCallback core.RestGenCallback) error {
	depsHTTPClient, depsErr := core.BuildDownstreamHTTPClient("deps", &h.coreCfg.GenCode.Downstream.(*DownstreamConfig).Deps)
	if depsErr != nil {
		return depsErr
	}

	depsClient := deps.NewClient(depsHTTPClient, h.coreCfg.GenCode.Downstream.(*DownstreamConfig).Deps.ServiceURL)
	serviceHandler := NewServiceHandler(serviceCallback, &serviceInterface, depsClient)
	serviceRouter := NewServiceRouter(serviceCallback, serviceHandler)
	h.enabledHandlers = append(h.enabledHandlers, serviceRouter)
	return nil
}
