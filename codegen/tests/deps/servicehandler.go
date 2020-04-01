// Code generated by sysl DO NOT EDIT.
package deps

import (
	"net/http"

	"github.com/anz-bank/sysl-go/common"
	"github.com/anz-bank/sysl-go/core"
	"github.com/anz-bank/sysl-go/restlib"
	"github.com/anz-bank/sysl-go/validator"
)

// Handler interface for Deps
type Handler interface {
	GetApiDocsListHandler(w http.ResponseWriter, r *http.Request)
}

// ServiceHandler for Deps API
type ServiceHandler struct {
	genCallback      core.RestGenCallback
	serviceInterface *ServiceInterface
}

// NewServiceHandler for Deps
func NewServiceHandler(genCallback core.RestGenCallback, serviceInterface *ServiceInterface) *ServiceHandler {
	return &ServiceHandler{genCallback, serviceInterface}
}

// GetApiDocsListHandler ...
func (s *ServiceHandler) GetApiDocsListHandler(w http.ResponseWriter, r *http.Request) {
	if s.serviceInterface.GetApiDocsList == nil {
		s.genCallback.HandleError(r.Context(), w, common.InternalError, "not implemented", nil)
		return
	}

	ctx := common.RequestHeaderToContext(r.Context(), r.Header)
	ctx = common.RespHeaderAndStatusToContext(ctx, make(http.Header), http.StatusOK)
	var req GetApiDocsListRequest

	ctx, cancel := s.genCallback.DownstreamTimeoutContext(ctx)
	defer cancel()
	valErr := validator.Validate(&req)
	if valErr != nil {
		s.genCallback.HandleError(ctx, w, common.BadRequestError, "Invalid request", valErr)
		return
	}

	client := GetApiDocsListClient{}

	apidoc, err := s.serviceInterface.GetApiDocsList(ctx, &req, client)
	if err != nil {
		s.genCallback.HandleError(ctx, w, common.DownstreamUnexpectedResponseError, "Downstream failure", err)
		return
	}

	headermap, httpstatus := common.RespHeaderAndStatusFromContext(ctx)
	restlib.SetHeaders(w, headermap)
	restlib.SendHTTPResponse(w, httpstatus, apidoc)
}
