// Code generated by sysl DO NOT EDIT.
package dbendpoints

import (
	"database/sql"
	"net/http"

	"github.com/anz-bank/sysl-go-comms/common"
	"github.com/anz-bank/sysl-go-comms/convert"
	"github.com/anz-bank/sysl-go-comms/database"
	"github.com/anz-bank/sysl-go-comms/restlib"
	"github.com/anz-bank/sysl-go-comms/validator"
)

// Handler interface for DbEndpoints
type Handler interface {
	GetCompanyLocationListHandler(w http.ResponseWriter, r *http.Request)
}

// ServiceHandler for DbEndpoints API
type ServiceHandler struct {
	genCallback      GenCallback
	serviceInterface *ServiceInterface
	DB               *sql.DB
}

// NewServiceHandler for DbEndpoints
func NewServiceHandler(genCallback GenCallback, serviceInterface *ServiceInterface) *ServiceHandler {
	db, err := database.GetDBHandle()
	if err != nil {
		return nil
	}

	return &ServiceHandler{genCallback, serviceInterface, db}
}

// GetCompanyLocationListHandler ...
func (s *ServiceHandler) GetCompanyLocationListHandler(w http.ResponseWriter, r *http.Request) {
	if s.serviceInterface.GetCompanyLocationList == nil {
		s.genCallback.HandleError(r.Context(), w, common.InternalError, "not implemented", nil)
		return
	}

	ctx := common.RequestHeaderToContext(r.Context(), r.Header)
	ctx = common.RespHeaderAndStatusToContext(ctx, make(http.Header), http.StatusOK)
	var req GetCompanyLocationListRequest

	req.DeptLoc = restlib.GetQueryParam(r, "deptLoc")
	var CompanyNameParam string

	var convErr error

	CompanyNameParam = restlib.GetQueryParam(r, "companyName")
	req.CompanyName, convErr = convert.StringToStringPtr(ctx, CompanyNameParam)
	if convErr != nil {
		s.genCallback.HandleError(ctx, w, common.BadRequestError, "Invalid request", convErr)
		return
	}

	ctx, cancel := s.genCallback.DownstreamTimeoutContext(ctx)
	defer cancel()
	valErr := validator.Validate(&req)
	if valErr != nil {
		s.genCallback.HandleError(ctx, w, common.BadRequestError, "Invalid request", valErr)
		return
	}

	conn, err := s.DB.Conn(ctx)
	if err != nil {
		s.genCallback.HandleError(ctx, w, common.InternalError, "Database connection could not be retrieved", err)
		return
	}

	defer conn.Close()
	retrievebycompanyandlocationStmt, err_retrievebycompanyandlocation := conn.PrepareContext(ctx, "select company.abnnumber, company.companyname, company.companycountry, department.deptid, department.deptname, department.deptloc from company JOIN department ON company.abnnumber=department.abn WHERE department.deptloc=? and company.companyname=? order by company.abnnumber;")
	if err_retrievebycompanyandlocation != nil {
		s.genCallback.HandleError(ctx, w, common.InternalError, "could not parse the sql query with the name sql_retrieveByCompanyAndLocation", err_retrievebycompanyandlocation)
		return
	}

	tx, err := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		s.genCallback.HandleError(ctx, w, common.DownstreamUnavailableError, "DB Transaction could not be created", err)
		return
	}

	client := GetCompanyLocationListClient{
		conn:                         conn,
		retrievebycompanyandlocation: retrievebycompanyandlocationStmt,
	}

	getcompanylocationresponse, err := s.serviceInterface.GetCompanyLocationList(ctx, &req, client)
	if err != nil {
		tx.Rollback()
		s.genCallback.HandleError(ctx, w, common.DownstreamUnexpectedResponseError, "Downstream failure", err)
		return
	}

	commitErr := tx.Commit()
	if commitErr != nil {
		s.genCallback.HandleError(ctx, w, common.InternalError, "Failed to commit the transaction", commitErr)
		return
	}

	headermap, httpstatus := common.RespHeaderAndStatusFromContext(ctx)
	restlib.SetHeaders(w, headermap)
	restlib.SendHTTPResponse(w, httpstatus, getcompanylocationresponse, err)
}
