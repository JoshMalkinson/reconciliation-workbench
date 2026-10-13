package main

import (
	"cloud.google.com/go/storage"
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reconciliation-workbench/internal/cloudstore"
	"reconciliation-workbench/internal/hosting"
	"reconciliation-workbench/internal/store"
	"reconciliation-workbench/internal/transport"
	"time"
)

func main() {
	defaultAddr := "127.0.0.1:8081"
	if port := os.Getenv("PORT"); port != "" {
		defaultAddr = "0.0.0.0:" + port
	}
	addr := flag.String("addr", defaultAddr, "listen address")
	dbPath := flag.String("db", "data/reconcile.db", "SQLite path")
	web := flag.String("web", "web/dist", "compiled frontend directory")
	flag.Parse()
	mux := http.NewServeMux()
	var app http.Handler = mux
	if bucket := os.Getenv("RECONCILE_BUCKET"); bucket != "" {
		client, err := storage.NewClient(context.Background())
		if err != nil {
			log.Fatal(err)
		}
		defer client.Close()
		objects := &cloudstore.GCS{Bucket: client.Bucket(bucket)}
		// Bound expensive snapshots in each instance, in addition to Cloud Run limits.
		slots := make(chan struct{}, 2)
		mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"hosted":true}`))
		})
		app = hosting.Workspace(func(key string) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if len(r.URL.Path) >= len("/reconcile.v1.WorkbenchService/") && r.URL.Path[:len("/reconcile.v1.WorkbenchService/")] == "/reconcile.v1.WorkbenchService/" {
					select {
					case slots <- struct{}{}:
						defer func() { <-slots }()
					case <-r.Context().Done():
						return
					}
					ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
					defer cancel()
					r.Body = http.MaxBytesReader(w, r.Body, 3<<20)
					transport.Handler(cloudstore.New(ctx, objects, key)).ServeHTTP(w, r.WithContext(ctx))
					return
				}
				mux.ServeHTTP(w, r)
			})
		})
	} else {
		if os.Getenv("K_SERVICE") != "" {
			log.Fatal("Cloud Run requires RECONCILE_BUCKET for durable isolated storage")
		}
		if err := os.MkdirAll(filepath.Dir(*dbPath), 0700); err != nil {
			log.Fatal(err)
		}
		db, err := store.Open(*dbPath)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		mux.Handle("/reconcile.v1.WorkbenchService/", transport.Handler(db))
		mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"hosted":false}`))
		})
	}
	mux.Handle("/", http.FileServer(http.Dir(*web)))
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
		app.ServeHTTP(w, r)
	})
	server := &http.Server{Addr: *addr, Handler: handler, Protocols: protocols, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 60 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("Reconcile ready at http://%s", *addr)
	log.Fatal(server.ListenAndServe())
}
