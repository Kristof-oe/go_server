package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/kris200036/go_server/internal/database"
	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	log.Printf("db_url", dbURL)
	platform := os.Getenv("PLATFORM")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)

	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
		db:             dbQueries,
		platform:       platform,
	}

	mux := http.NewServeMux()
	fileserver := http.FileServer(http.Dir("."))
	// mux.Handle("/app/", http.StripPrefix("/app", fileserver))
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", fileserver)))
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)
	mux.HandleFunc("POST /api/validate_chirp", handlerChirpsValidate)
	mux.HandleFunc("POST /api/users", apiCfg.handlerUsers)

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
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
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}
func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {

	if cfg.platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	err := cfg.db.DeleteAllUsers(r.Context())
	if err != nil {
		log.Printf("Its bad: %s", err)
		w.WriteHeader(500)
		return
	}

	cfg.fileserverHits.Store(0)

	w.WriteHeader(http.StatusOK)

}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(
		`<html>
  		<body>
    		<h1>Welcome, Chirpy Admin</h1>
    		<p>Chirpy has been visited %d times!</p>
  		</body>
	</html>
	`, cfg.fileserverHits.Load())))

}

func respondWithError(w http.ResponseWriter, code int, msg string) {

	type vals struct {
		Error string `json:"error"`
	}
	resBody := vals{
		Error: msg,
	}
	respondWithJSON(w, code, resBody)

}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {

	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
}

func handlerChirpsValidate(w http.ResponseWriter, r *http.Request) {
	type param struct {
		Body string `json:"body"`
	}
	decoder := json.NewDecoder(r.Body)
	params := param{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Something went wrong %s", err)
		w.WriteHeader(500)
		return
	}
	if len(params.Body) > 140 {
		respondWithError(w, 400, "Chirp is too long")
		return
	}
	// type vals struct {
	// 	Valid bool `json:"valid"`
	// }
	// respondWithJSON(w, 200, vals{Valid: true})

	seg := strings.Split(params.Body, " ")

	for i, b := range seg {
		if strings.ToLower(b) == "kerfuffle" || strings.ToLower(b) == "sharbert" || strings.ToLower(b) == "fornax" {
			seg[i] = "****"
		}
	}
	seg2 := strings.Join(seg, " ")

	type vals_ struct {
		Cleaned_body string `json:"cleaned_body"`
	}
	respondWithJSON(w, 200, vals_{Cleaned_body: seg2})

}

func (cfg *apiConfig) handlerUsers(w http.ResponseWriter, r *http.Request) {

	type param struct {
		Email string `json:"email"`
	}
	decoder := json.NewDecoder(r.Body)
	params := param{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Something went wrong___: %s", err)
		w.WriteHeader(500)
		return
	}
	user, err := cfg.db.CreateUser(r.Context(), params.Email)
	if err != nil {
		log.Printf("Something went wrong: %s", err)
		w.WriteHeader(500)
		return

	}

	respondWithJSON(w, http.StatusCreated, User{ID: user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email})

}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}
