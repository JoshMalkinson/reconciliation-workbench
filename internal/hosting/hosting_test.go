package hosting

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkspaceCookiesAndCrossSiteRequests(t *testing.T) {
	var key string
	handler := Workspace(func(id string) http.Handler {
		key = id
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "https://demo.test/", nil))
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || len(key) != 64 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatal("unsafe cookie", cookies)
	}
	old := key
	r := httptest.NewRequest("POST", "https://demo.test/reconcile.v1.WorkbenchService/ListRuns", nil)
	r.AddCookie(cookies[0])
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if key != old || w.Code != 204 {
		t.Fatal("lost workspace")
	}
	r.Header.Set("Origin", "https://attacker.test")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-site request accepted")
	}
	r = httptest.NewRequest("POST", "https://demo.test/reconcile.v1.WorkbenchService/ListRuns", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("RPC without session accepted")
	}
	r = httptest.NewRequest("GET", "https://demo.test/", nil)
	r.AddCookie(&http.Cookie{Name: cookieName, Value: "../../victim"})
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if len(key) != 64 || strings.Contains(key, ".") {
		t.Fatal("unsafe workspace key")
	}
}
