package hosting

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
	"strings"
)

const cookieName = "__Host-reconcile-workspace"

var validID = regexp.MustCompile(`^[a-f0-9]{64}$`)

// Workspace never accepts an identity from the URL or RPC body. The random
// bearer cookie selects one private demo snapshot, including for read requests.
func Workspace(next func(string) http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != "GET" && r.Method != "HEAD" {
			origin := r.Header.Get("Origin")
			if (origin != "" && origin != "https://"+r.Host) || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				http.Error(w, "Cross-site requests are not allowed", http.StatusForbidden)
				return
			}
		}
		key := ""
		if cookie, err := r.Cookie(cookieName); err == nil && validID.MatchString(cookie.Value) {
			key = cookie.Value
		}
		if key == "" {
			if strings.HasPrefix(r.URL.Path, "/reconcile.v1.") {
				http.Error(w, "Open the demo homepage to start your workspace", http.StatusUnauthorized)
				return
			}
			b := make([]byte, 32)
			if _, err := rand.Read(b); err != nil {
				http.Error(w, "Unable to create workspace", 500)
				return
			}
			key = hex.EncodeToString(b)
			http.SetCookie(w, &http.Cookie{Name: cookieName, Value: key, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: 7 * 24 * 60 * 60})
		}
		next(key).ServeHTTP(w, r)
	})
}
