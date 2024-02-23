package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"text/template"
	"time"

	"github.com/gorilla/mux"

	"github.com/vkl/rfidplayer/pkg/control"
	"github.com/vkl/rfidplayer/pkg/logging"
)

var (
	cardController    *control.CardController
	chromecastControl *control.ChromecastControl
)

func GetCards(w http.ResponseWriter, r *http.Request) {
	encoder := json.NewEncoder(w)
	encoder.Encode(cardController.Cards)
}

func GetCard(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	logging.Log.Debug("", "vars", vars)
	if _, ok := cardController.Cards[vars["id"]]; !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(cardController.Cards[vars["id"]])
}

func AddCard(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	card := control.Card{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&card); err != nil {
		logging.Log.Error("add card", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	cardController.AddCard(card)
	encoder := json.NewEncoder(w)
	encoder.Encode(cardController.Cards)
}

func DelCard(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	logging.Log.Debug("", "vars", vars)
	cardController.DelCard(vars["id"])
	encoder := json.NewEncoder(w)
	encoder.Encode(cardController.Cards)
}

func PlayCard(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	if _, ok := cardController.Cards[vars["id"]]; !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	card := cardController.Cards[vars["id"]]
	chromecastControl.PlayCard(card)
	w.WriteHeader(http.StatusAccepted)
}

func GetCasts(w http.ResponseWriter, r *http.Request) {
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(chromecastControl.GetClients()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

func DiscoverCasts(w http.ResponseWriter, r *http.Request) {
	chromecastControl.StartDiscovery(control.DISCOVERY_TIMEOUT * time.Second)
	w.WriteHeader(http.StatusAccepted)
}

func ControlCasts(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	payload := control.ClientAction{}
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	if err := decoder.Decode(&payload); err != nil {
		logging.Log.Error("control cast", "error", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if !chromecastControl.ClientControl(vars["name"], payload) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func SiteHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		logging.Log.Error("parse template", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var data interface{}
	tmpl.Execute(w, data)
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}

func ContentJson(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func StartApp() {
	r := mux.NewRouter()
	r.Use(noCache)
	r.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/",
			http.FileServer(http.Dir("static"))))
	r.HandleFunc("/", SiteHandler).Methods("GET")
	apiPrefix := r.PathPrefix("/api").Subrouter()
	apiPrefix.Use(ContentJson)
	apiPrefix.HandleFunc("/cards", GetCards).Methods("GET")
	apiPrefix.HandleFunc("/casts", GetCasts).Methods("GET")
	apiPrefix.HandleFunc("/casts", DiscoverCasts).Methods("POST")
	apiPrefix.HandleFunc("/casts/{name}", ControlCasts).Methods("POST")
	apiPrefix.HandleFunc("/cards", AddCard).Methods("POST")
	apiPrefix.HandleFunc("/cards/{id}", DelCard).Methods("DELETE")
	apiPrefix.HandleFunc("/cards/{id}", GetCard).Methods("GET")
	apiPrefix.HandleFunc("/cards/{id}", PlayCard).Methods("POST")

	var err error
	cardController, err = control.NewCardController("cards.json")
	if err != nil {
		log.Fatal(err)
	}
	chromecastControl = control.NewChromeCastControl()
	chromecastControl.StartDiscovery(control.DISCOVERY_TIMEOUT * time.Second)

	srv := &http.Server{
		Handler:      r,
		Addr:         "127.0.0.1:8080",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		logging.Log.Info("HTTP server Shutdown")

		// We received an interrupt signal, shut down.
		if err := srv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			logging.Log.Info("HTTP server Shutdown", "error", err)
		}

		close(idleConnsClosed)
	}()

	logging.Log.Info("Start app")
	log.Fatal(srv.ListenAndServe())
	// log.Fatal(srv.ListenAndServeTLS(cert, key))

	<-idleConnsClosed
}
