package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reconciliation-workbench/internal/store"
	"reconciliation-workbench/internal/transport"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8081", "listen address")
	dbPath := flag.String("db", "data/reconcile.db", "SQLite path")
	web := flag.String("web", "web/dist", "compiled frontend directory")
	flag.Parse()
	if err := os.MkdirAll(filepath.Dir(*dbPath), 0700); err != nil {
		log.Fatal(err)
	}
	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	mux := http.NewServeMux()
	mux.Handle("/reconcile.v1.WorkbenchService/", transport.Handler(db))
	mux.Handle("/", http.FileServer(http.Dir(*web)))
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
		mux.ServeHTTP(w, r)
	})
	server := &http.Server{Addr: *addr, Handler: handler, Protocols: protocols, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("Reconcile ready at http://%s", *addr)
	log.Fatal(server.ListenAndServe())
}
