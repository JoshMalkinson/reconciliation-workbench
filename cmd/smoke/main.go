// Smoke exercises the real hosted gRPC-Web API using fictional data only.
package main

import (
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	pb "reconciliation-workbench/gen/reconcile/v1"
	rpc "reconciliation-workbench/gen/reconcile/v1/reconcilev1connect"
	"strings"
	"time"
)

type saved struct {
	URL     string
	RunID   string
	Cookies []*http.Cookie
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func session(base string) (*http.Client, *url.URL) {
	u, e := url.Parse(base)
	must(e)
	if u.Scheme != "https" {
		must(fmt.Errorf("live smoke requires HTTPS"))
	}
	jar, e := cookiejar.New(nil)
	must(e)
	client := &http.Client{Jar: jar, Timeout: 60 * time.Second}
	return client, u
}
func main() {
	base := flag.String("url", "", "HTTPS service URL")
	state := flag.String("state", "data/live-smoke.json", "private local session file")
	verify := flag.Bool("verify", false, "verify saved run after a new service revision")
	flag.Parse()
	client, u := session(*base)
	ctx := context.Background()
	api := rpc.NewWorkbenchServiceClient(client, *base, connect.WithGRPCWeb())
	if *verify {
		data, e := os.ReadFile(*state)
		must(e)
		var s saved
		must(json.Unmarshal(data, &s))
		if s.URL != *base {
			must(fmt.Errorf("saved session belongs to a different URL"))
		}
		client.Jar.SetCookies(u, s.Cookies)
		got, e := api.GetRun(ctx, connect.NewRequest(&pb.GetRunRequest{Id: s.RunID}))
		must(e)
		found := false
		for _, item := range got.Msg.Items {
			if item.Internal.GetId() == "FUND-A" && item.UnitsDelta == -20_000000 && item.Reviewed && len(item.Events) == 1 {
				found = true
			}
		}
		if !found {
			must(fmt.Errorf("saved mapping and review not restored"))
		}
		fmt.Println("PASS: mapping, exact deltas and review survived a new service revision")
		return
	}
	resp, e := client.Get(*base + "/")
	must(e)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		must(fmt.Errorf("homepage status %d", resp.StatusCode))
	}
	a, e := os.ReadFile("web/public/samples/internal.csv")
	must(e)
	b, e := os.ReadFile("web/public/samples/administrator.csv")
	must(e)
	created, e := api.CreateRun(ctx, connect.NewRequest(&pb.CreateRunRequest{OperationId: "smoke-import", Internal: a, Administrator: b, InternalName: "internal.csv", AdministratorName: "administrator.csv"}))
	must(e)
	_, e = api.ApproveMapping(ctx, connect.NewRequest(&pb.MappingRequest{OperationId: "smoke-map", RunId: created.Msg.Id, ProviderCode: "EXT-001", InternalId: "FUND-A", Reviewer: "Demo verification", Note: "Confirmed fictional provider identity"}))
	must(e)
	rerun, e := api.CreateRun(ctx, connect.NewRequest(&pb.CreateRunRequest{OperationId: "smoke-rerun", ParentId: created.Msg.Id}))
	must(e)
	var target *pb.Item
	for _, item := range rerun.Msg.Items {
		if item.Internal.GetId() == "FUND-A" {
			target = item
		}
	}
	if target == nil || target.UnitsDelta != -20_000000 || target.ValueDelta != -240000 {
		must(fmt.Errorf("unexpected live comparison"))
	}
	_, e = api.ReviewItem(ctx, connect.NewRequest(&pb.ReviewRequest{OperationId: "smoke-review", RunId: rerun.Msg.Id, ItemId: target.Id, Reviewer: "Demo verification", Reason: "timing_difference", Note: "Fictional settlement timing confirmed", Reviewed: true}))
	must(e)
	report, e := api.ExportRun(ctx, connect.NewRequest(&pb.GetRunRequest{Id: rerun.Msg.Id}))
	must(e)
	if !strings.Contains(string(report.Msg.Csv), "-2400.00") {
		must(fmt.Errorf("export is missing expected delta"))
	}
	other, _ := session(*base)
	res, e := other.Get(*base + "/")
	must(e)
	res.Body.Close()
	otherAPI := rpc.NewWorkbenchServiceClient(other, *base, connect.WithGRPCWeb())
	_, e = otherAPI.GetRun(ctx, connect.NewRequest(&pb.GetRunRequest{Id: rerun.Msg.Id}))
	if connect.CodeOf(e) != connect.CodeInvalidArgument {
		must(fmt.Errorf("workspace isolation failed: %v", e))
	}
	listed, e := otherAPI.ListRuns(ctx, connect.NewRequest(&pb.Empty{}))
	must(e)
	if len(listed.Msg.Runs) != 0 {
		must(fmt.Errorf("other workspace is not empty"))
	}
	// Cookie jar omits attributes on returned cookies; restore the intended scope
	// when persisting this private verification session for the next command.
	cookies := client.Jar.Cookies(u)
	for _, c := range cookies {
		c.Path = "/"
		c.Secure = true
	}
	data, e := json.Marshal(saved{URL: *base, RunID: rerun.Msg.Id, Cookies: cookies})
	must(e)
	must(os.MkdirAll(filepath.Dir(*state), 0700))
	must(os.WriteFile(*state, data, 0600))
	fmt.Println("PASS: HTTPS import, mapping, rerun, exact differences, review, CSV export and workspace isolation")
}
