package transport

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	pb "reconciliation-workbench/gen/reconcile/v1"
	rpc "reconciliation-workbench/gen/reconcile/v1/reconcilev1connect"
	"reconciliation-workbench/internal/store"
	"testing"
)

func TestRealProtocols(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := httptest.NewUnstartedServer(Handler(db))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	for index, option := range []connect.ClientOption{connect.WithGRPCWeb(), connect.WithGRPC()} {
		client := rpc.NewWorkbenchServiceClient(server.Client(), server.URL, option)
		listed, err := client.ListRuns(context.Background(), connect.NewRequest(&pb.Empty{}))
		if err != nil || len(listed.Msg.Runs) != index {
			t.Fatalf("list: %v", err)
		}
		_, err = client.CreateRun(context.Background(), connect.NewRequest(&pb.CreateRunRequest{OperationId: "invalid", Internal: []byte("bad")}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatal(err)
		}
		ce := new(connect.Error)
		if !errors.As(err, &ce) || len(ce.Details()) != 1 {
			t.Fatal("missing validation detail", err)
		}
		created, err := client.CreateRun(context.Background(), connect.NewRequest(&pb.CreateRunRequest{
			OperationId:   fmt.Sprintf("create-%d", index),
			Internal:      []byte("instrument_id,instrument_name,as_of,currency,units,market_value\nA,Equity,2026-10-01,ZAR,1000000000,120000"),
			Administrator: []byte("provider_code,security_id,description,valuation_date,ccy,quantity,value\nEXT,A,Equity A,2026-10-01,ZAR,999999999.999999,117600"),
		}))
		if err != nil {
			t.Fatal(err)
		}
		if len(created.Msg.Items) != 1 || created.Msg.Items[0].UnitsDelta != -1 || created.Msg.Items[0].Internal.Units != 1_000_000_000_000000 {
			t.Fatal("wire precision lost", created.Msg)
		}
		got, err := client.GetRun(context.Background(), connect.NewRequest(&pb.GetRunRequest{Id: created.Msg.Id}))
		if err != nil || got.Msg.Items[0].Kind != "both_difference" {
			t.Fatal("wire roundtrip", err)
		}
	}
}
