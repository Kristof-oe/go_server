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

	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/kris200036/go_server/internal/auth"
	"github.com/kris200036/go_server/internal/database"
	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	log.Printf("db_url %s", dbURL)
	platform := os.Getenv("PLATFORM")
	secret := os.Getenv("SECRET")
	apikey := os.Getenv("POLKA_KEY")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)

	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
		apiKey:         apikey,
		db:             dbQueries,
		platform:       platform,
		jwtSecret:      secret,
	}

	mux := http.NewServeMux()
	fileserver := http.FileServer(http.Dir("."))
	// mux.Handle("/app/", http.StripPrefix("/app", fileserver))
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", fileserver)))
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)
	// mux.HandleFunc("POST /api/validate_chirp", handlerChirpsValidate)
	mux.HandleFunc("POST /api/users", apiCfg.handlerUsers)
	mux.HandleFunc("POST /api/chirps", apiCfg.handleCreate)
	mux.HandleFunc("GET /api/chirps", apiCfg.handleGetAll)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.handleGetByID)
	mux.HandleFunc("POST /api/login", apiCfg.handleLogin)
	mux.HandleFunc("POST /api/refresh", apiCfg.handleRefresh)
	mux.HandleFunc("POST /api/revoke", apiCfg.handleRevoke)
	mux.HandleFunc("PUT /api/users", apiCfg.handleUpdate)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiCfg.HandleDelete)
	mux.HandleFunc("POST /api/polka/webhooks", apiCfg.handleEvent)

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
	jwtSecret      string
	apiKey         string
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
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	params := param{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Something went wrong___: %s", err)
		w.WriteHeader(500)
		return
	}
	hashed_Pwd, err := auth.HashPassword(params.Password)
	db_Param := database.CreateUserParams{
		Email:          params.Email,
		HashedPassword: hashed_Pwd,
	}
	user, err := cfg.db.CreateUser(r.Context(), db_Param)
	if err != nil {
		log.Printf("Something went wrong: %s", err)
		w.WriteHeader(500)
		return

	}

	respondWithJSON(w, http.StatusCreated, User{
		ID:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed})

}

func (cfg *apiConfig) handleCreate(w http.ResponseWriter, r *http.Request) {
	type param struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}
	decoder := json.NewDecoder(r.Body)
	params := param{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	if len(params.Body) > 140 {
		respondWithError(w, 400, "Chirp is too long")
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "wrong")
		return
	}
	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "wrong")
		return
	}

	chirp, err := cfg.db.CreateChirps(r.Context(), database.CreateChirpsParams{
		Body:   params.Body,
		UserID: userID,
	})
	if err != nil {
		log.Printf("Something went wrong: %s", err)
		w.WriteHeader(500)
		return

	}

	respondWithJSON(w, http.StatusCreated, Chirp{ID: chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID})

}

func (cfg *apiConfig) handleGetAll(w http.ResponseWriter, r *http.Request) {

	s := r.URL.Query().Get("author_id")
	sort_u := r.URL.Query().Get("sort")
	chirps := []Chirp{}
	s_u, err := uuid.Parse(s)
	if err != nil {
		chirps_db, err := cfg.db.GetChirps(r.Context())
		if err != nil {
			log.Printf("Something went wrong: %s", err)
			w.WriteHeader(500)
			return

		}

		for _, chirp := range chirps_db {

			chirps = append(chirps, Chirp{
				ID:        chirp.ID,
				CreatedAt: chirp.CreatedAt,
				UpdatedAt: chirp.UpdatedAt,
				Body:      chirp.Body,
				UserID:    chirp.UserID})
		}

	} else {
		author_by, err := cfg.db.GetByID2(r.Context(), s_u)
		if err != nil {
			respondWithError(w, 404, "not found")
			return
		}
		for _, chirp := range author_by {

			chirps = append(chirps, Chirp{
				ID:        chirp.ID,
				CreatedAt: chirp.CreatedAt,
				UpdatedAt: chirp.UpdatedAt,
				Body:      chirp.Body,
				UserID:    chirp.UserID})
		}
	}

	if sort_u == "asc" || sort_u == "" {
		sort.Slice(chirps, func(i, j int) bool { return chirps[i].CreatedAt.Before(chirps[j].CreatedAt) })

	} else {
		sort.Slice(chirps, func(i, j int) bool { return chirps[i].CreatedAt.After(chirps[j].CreatedAt) })
	}

	respondWithJSON(w, http.StatusOK, chirps)

}

func (cfg *apiConfig) handleGetByID(w http.ResponseWriter, r *http.Request) {

	data := r.PathValue("chirpID")

	data_u, err := uuid.Parse(data)
	if err != nil {

		w.WriteHeader(404)
		return

	}
	raw, err := cfg.db.GetByID(r.Context(), data_u)
	if err != nil {

		w.WriteHeader(404)
		return
	}

	respondWithJSON(w, http.StatusOK, Chirp{
		ID:        raw.ID,
		CreatedAt: raw.CreatedAt,
		UpdatedAt: raw.UpdatedAt,
		Body:      raw.Body,
		UserID:    raw.UserID})

}

func (cfg *apiConfig) handleLogin(w http.ResponseWriter, r *http.Request) {

	type param struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		// ExpiresIn int    `json:"expires_in_seconds"`
	}
	decoder := json.NewDecoder(r.Body)
	params := param{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Something went wrong___: %s", err)
		respondWithError(w, http.StatusBadRequest, "Invalid")
		return
	}
	// expirationTime := time.Hour

	// if params.ExpiresIn > 0 && params.ExpiresIn < 3600 {
	// 	expirationTime = time.Duration(params.ExpiresIn) * time.Second
	// }

	user, err := cfg.db.GetByLogin(r.Context(), params.Email)
	if err != nil {
		log.Printf("Something went wrong: %s", err)
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return

	}
	seg, err := auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}
	if !seg {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}

	accees_token, err := auth.MakeJWT(user.ID, cfg.jwtSecret, time.Hour)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create token")
		return
	}
	refreshed_token := auth.MakeRefreshToken()
	sixty_days := time.Now().UTC().Add(time.Hour * 24 * 60)

	_, err = cfg.db.CreateRefreshToken(r.Context(),
		database.CreateRefreshTokenParams{
			Token:     refreshed_token,
			UserID:    user.ID,
			ExpiresAt: sixty_days,
		})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "wrong")
		return
	}

	respondWithJSON(w, http.StatusOK, User{
		ID:            user.ID,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
		Email:         user.Email,
		Token:         accees_token,
		Refresh_token: refreshed_token,
		IsChirpyRed:   user.IsChirpyRed})
}

func (cfg *apiConfig) handleRefresh(w http.ResponseWriter, r *http.Request) {

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "wrong")
		return
	}
	user, err := cfg.db.GetUserFromRefreshToken(r.Context(), token)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "unauthorized getuser")
		return
	}
	accessToken, err := auth.MakeJWT(
		user.ID,
		cfg.jwtSecret,
		time.Hour,
	)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "wrong makeJwt")
		return
	}
	type response struct {
		Token string `json:"token"`
	}

	respondWithJSON(w, http.StatusOK, response{
		Token: accessToken,
	})

}

func (cfg *apiConfig) handleRevoke(w http.ResponseWriter, r *http.Request) {

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "wrong header")
		return
	}

	_, err = cfg.db.RefreshToken(r.Context(), token)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "unauthorized refresh")
		return
	}

	w.WriteHeader(http.StatusNoContent)

}

func (cfg *apiConfig) handleUpdate(w http.ResponseWriter, r *http.Request) {

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "no token")
		return
	}
	type param struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	params := param{}
	err = decoder.Decode(&params)
	if err != nil {
		log.Printf("Something went wrong___: %s", err)
		respondWithError(w, http.StatusBadRequest, "Invalid")
		return
	}
	seg, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "no id")
		return
	}
	hashed, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "this is it")
		return
	}

	user, err := cfg.db.UpdateUser(r.Context(), database.UpdateUserParams{
		Email:          params.Email,
		HashedPassword: hashed,
		ID:             seg})

	if err != nil {
		log.Printf("Something went wrong: %s", err)
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return

	}

	respondWithJSON(w, http.StatusOK, User{
		ID:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed})
}

func (cfg *apiConfig) HandleDelete(w http.ResponseWriter, r *http.Request) {

	data := r.PathValue("chirpID")

	data_u, err := uuid.Parse(data)
	if err != nil {

		w.WriteHeader(404)
		return

	}
	chirp, err := cfg.db.GetByID(r.Context(), data_u)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "its not a real chirp")
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "no token")
		return
	}

	seg, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "no id")
		return
	}

	if chirp.UserID != seg {
		respondWithError(w, http.StatusForbidden, "error forbidden")
		return
	}
	err = cfg.db.DeleteChripByID(r.Context(), data_u)
	if err != nil {

		w.WriteHeader(404)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) handleEvent(w http.ResponseWriter, r *http.Request) {

	type param struct {
		Event string `json:"event"`
		Data  struct {
			Userid uuid.UUID `json:"user_id"`
		}
	}
	decoder := json.NewDecoder(r.Body)
	params := param{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Something went wrong___: %s", err)
		respondWithError(w, http.StatusBadRequest, "Invalid")
		return
	}
	r_header_authorization, err := auth.GetAPIKey(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "not good")
		return
	}
	if cfg.apiKey != r_header_authorization {
		respondWithError(w, http.StatusUnauthorized, "not matchy")
		return
	}

	if params.Event != "user.upgraded" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	_, err = cfg.db.UpdateChripy(r.Context(), params.Data.Userid)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "its not a real user")
		return
	}

	w.WriteHeader(http.StatusNoContent)

}

type User struct {
	ID            uuid.UUID `json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Email         string    `json:"email"`
	Password      string    `json:"password"`
	Token         string    `json:"token"`
	Refresh_token string    `json:"refresh_token"`
	IsChirpyRed   bool      `json:"is_chirpy_red"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}
