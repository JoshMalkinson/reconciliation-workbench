package transport

import (
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"log"
	"net/http"
	pb "reconciliation-workbench/gen/reconcile/v1"
	rpc "reconciliation-workbench/gen/reconcile/v1/reconcilev1connect"
	"reconciliation-workbench/internal/domain"
	"reconciliation-workbench/internal/store"
)

type service struct{ db *store.Store }

func Handler(db *store.Store) http.Handler {
	mux := http.NewServeMux()
	path, h := rpc.NewWorkbenchServiceHandler(&service{db}, connect.WithReadMaxBytes(3<<20), connect.WithSendMaxBytes(16<<20))
	mux.Handle(path, h)
	return mux
}
func failure(err error) error {
	var validation *domain.Validation
	var fault *store.Fault
	if errors.As(err, &validation) {
		e := connect.NewError(connect.CodeInvalidArgument, err)
		detail, _ := connect.NewErrorDetail(&pb.ValidationDetail{File: validation.File, Row: int32(validation.Row), Field: validation.Field, Message: validation.Message})
		e.AddDetail(detail)
		return e
	}
	if errors.As(err, &fault) {
		code := connect.CodeInvalidArgument
		if fault.Conflict {
			code = connect.CodeFailedPrecondition
		}
		return connect.NewError(code, err)
	}
	log.Printf("request failed: %v", err)
	return connect.NewError(connect.CodeInternal, errors.New("could not complete this request; please retry"))
}

// Domain JSON uses the same documented field names as the wire contract.
// Conversion errors are checked, and protocol integration tests guard the boundary.
func response[T any, P interface {
	*T
	proto.Message
}](value any, err error, message P) (*connect.Response[T], error) {
	if err != nil {
		return nil, failure(err)
	}
	b, err := json.Marshal(value)
	if err == nil {
		err = protojson.Unmarshal(b, message)
	}
	if err != nil {
		return nil, failure(err)
	}
	return connect.NewResponse((*T)(message)), nil
}
func (s *service) CreateRun(_ context.Context, r *connect.Request[pb.CreateRunRequest]) (*connect.Response[pb.Run], error) {
	m := r.Msg
	v, e := s.db.Create(store.CreateRequest{OperationID: m.OperationId, ParentID: m.ParentId, Internal: m.Internal, Administrator: m.Administrator, InternalName: m.InternalName, AdministratorName: m.AdministratorName})
	return response(v, e, &pb.Run{})
}
func (s *service) GetRun(_ context.Context, r *connect.Request[pb.GetRunRequest]) (*connect.Response[pb.Run], error) {
	v, e := s.db.Get(r.Msg.Id)
	return response(v, e, &pb.Run{})
}
func (s *service) ListRuns(_ context.Context, _ *connect.Request[pb.Empty]) (*connect.Response[pb.Runs], error) {
	v, e := s.db.List()
	return response(map[string]any{"runs": v}, e, &pb.Runs{})
}
func (s *service) ListMappings(_ context.Context, _ *connect.Request[pb.Empty]) (*connect.Response[pb.Mappings], error) {
	v, e := s.db.Mappings()
	return response(map[string]any{"mappings": v}, e, &pb.Mappings{})
}
func (s *service) ApproveMapping(_ context.Context, r *connect.Request[pb.MappingRequest]) (*connect.Response[pb.Mapping], error) {
	m := r.Msg
	v, e := s.db.Map(store.MappingRequest{OperationID: m.OperationId, RunID: m.RunId, ProviderCode: m.ProviderCode, InternalID: m.InternalId, Reviewer: m.Reviewer, Note: m.Note})
	return response(v, e, &pb.Mapping{})
}
func (s *service) ReviewItem(_ context.Context, r *connect.Request[pb.ReviewRequest]) (*connect.Response[pb.Item], error) {
	m := r.Msg
	v, e := s.db.Review(store.ReviewRequest{OperationID: m.OperationId, RunID: m.RunId, ItemID: m.ItemId, Reviewer: m.Reviewer, Reason: m.Reason, Note: m.Note, Reviewed: m.Reviewed, ExpectedRevision: m.ExpectedRevision})
	return response(v, e, &pb.Item{})
}
func (s *service) ExportRun(_ context.Context, r *connect.Request[pb.GetRunRequest]) (*connect.Response[pb.Export], error) {
	run, e := s.db.Get(r.Msg.Id)
	if e != nil {
		return nil, failure(e)
	}
	data, e := store.ExportCSV(run)
	if e != nil {
		return nil, failure(e)
	}
	return connect.NewResponse(&pb.Export{Csv: data, Filename: "reconcile-" + run.ID + ".csv"}), nil
}
