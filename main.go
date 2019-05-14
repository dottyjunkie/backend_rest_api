package main

import (
	"database/sql"
	"fmt"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

var (
	dbUser = "wudajun"
	dbPassword = "godisagirl"
	dbName = "track_db"
	dbHost = "localhost"
	dbPort = "5432"
	dbSSLMode = "disable"
)
const (
	getSongQuery    = "SELECT Song FROM Tracks WHERE id=$1"
	getSongsQuery   = "SELECT id, Song FROM Tracks LIMIT $1 OFFSET $2"
	updateSongQuery = "UPDATE Songs SET Song=$1 WHERE id=$2"
	deleteSongQuery = "DELETE FROM Tracks WHERE id=$1"
	createSongQuery = "INSERT INTO Tracks(Song) VALUES($1) RETURNING id"
)
func main() {
	a := App{}
	a.Initialize(dbUser, dbPassword, dbName, dbHost, dbPort, dbSSLMode)
	a.Run(":8080")
}
type App struct {
	Router *mux.Router
	DB     *sql.DB
}
type track struct {
	ID    int     `json:"id"`
	Song  string  `json:"song"`
}

func (t *track) getSong(db *sql.DB) error {
	return db.QueryRow(getSongQuery, t.ID).Scan(&t.Song)
}
func (t *track) updateSong(db *sql.DB) error {
	_, err := db.Exec(updateSongQuery, t.Song, t.ID)
	return err
}
func (t *track) deleteSong(db *sql.DB) error {
	_, err := db.Exec(deleteSongQuery, t.ID)
	return err
}
func (t *track) createSong(db *sql.DB) error {
	err := db.QueryRow(createSongQuery, t.Song).Scan(&t.ID)
	if err != nil {
		return err
	}

	return nil
}
func getSongs(db *sql.DB, start, count int) ([]track, error) {
	rows, err := db.Query(getSongsQuery, count, start)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tracks := []track{}

	for rows.Next() {
		var t track
		if err := rows.Scan(&t.ID, &t.Song); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}

	return tracks, nil
}

func (a *App) Initialize(user, password, dbname, host, port, sslmode string) {
	dsn := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%s sslmode=%s", user, password, dbname, host, port, sslmode)

	var err error
	a.DB, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}

	err = a.DB.Ping()
	if err != nil {
		log.Fatal(err)
	}

	a.Router = mux.NewRouter()
	a.initializeRoutes()
}
func (a *App) Run(addr string) {
	log.Fatal(http.ListenAndServe(addr, a.Router))

	defer a.DB.Close()
}
func (a *App) initializeRoutes() {
	a.Router.HandleFunc("/tracks", a.getSongs).Methods("GET")
	a.Router.HandleFunc("/track/{id:[0-9]+}", a.getSong).Methods("GET")
	a.Router.HandleFunc("/track", a.createSong).Methods("POST")
	a.Router.HandleFunc("/track/{id:[0-9]+}", a.updateSong).Methods("PUT")
	a.Router.HandleFunc("/track/{id:[0-9]+}", a.deleteSong).Methods("DELETE")
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func (a *App) getSong(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		msg := fmt.Sprintf("Invalid track ID. Error: %s", err.Error())
		respondWithError(w, http.StatusBadRequest, msg)
		return
	}

	t := track{ID: id}
	if err := t.getSong(a.DB); err != nil {
		switch err {
		case sql.ErrNoRows:
			msg := fmt.Sprintf("track not found. Error: %s", err.Error())
			respondWithError(w, http.StatusNotFound, msg)
		default:
			respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	respondWithJSON(w, http.StatusOK, t)
}
func (a *App) getSongs(w http.ResponseWriter, r *http.Request) {
	count, _ := strconv.Atoi(r.FormValue("count"))
	start, _ := strconv.Atoi(r.FormValue("start"))

	if count > 10 || count < 1 {
		count = 10
	}
	if start < 0 {
		start = 0
	}

	Songs, err := getSongs(a.DB, start, count)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, Songs)
}
func (a *App) createSong(w http.ResponseWriter, r *http.Request) {
	var t track
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&t); err != nil {
		msg := fmt.Sprintf("Invalid request payload. Error: %s", err.Error())
		respondWithError(w, http.StatusBadRequest, msg)
		return
	}
	defer r.Body.Close()

	if err := t.createSong(a.DB); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, t)
}
func (a *App) updateSong(w http.ResponseWriter, r *http.Request) {
	var t track

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		msg := fmt.Sprintf("Invalid track ID. Error: %s", err.Error())
		respondWithError(w, http.StatusBadRequest, msg)
		return
	}
	t.ID = id

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&t); err != nil {
		msg := fmt.Sprintf("Invalid request payload. Error: %s", err.Error())
		respondWithError(w, http.StatusBadRequest, msg)
		return
	}
	defer r.Body.Close()

	if err := t.updateSong(a.DB); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, t)
}
func (a *App) deleteSong(w http.ResponseWriter, r *http.Request) {
	var t track

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		msg := fmt.Sprintf("Invalid track ID. Error: %s", err.Error())
		respondWithError(w, http.StatusBadRequest, msg)
		return
	}
	t.ID = id

	if err := t.deleteSong(a.DB); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
	}
}
