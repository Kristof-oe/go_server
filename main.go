package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

func main() {

	apiCfg := &apiConfig{}
	mux := http.NewServeMux()
	fileserver := http.FileServer(http.Dir("."))
	// mux.Handle("/app/", http.StripPrefix("/app", fileserver))
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", fileserver)))
	mux.HandleFunc("/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("/reset", apiCfg.handlerReset)

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})

	mine := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	mine.ListenAndServe()
}

type apiConfig struct {
	fileserverHts atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHts.Add(1)
		next.ServeHTTP(w, r)
	})
}
func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHts.Store(0)
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Hits: %d", cfg.fileserverHts.Load())))

}
