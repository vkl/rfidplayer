package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"text/template"
	"time"

	"github.com/gorilla/mux"

	"github.com/vkl/rfidplayer/pkg/control"
	"github.com/vkl/rfidplayer/pkg/logging"
)

func GetCards(cardController *control.CardController) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encoder := json.NewEncoder(w)
		encoder.Encode(cardController.Cards)
	})
}

func GetCard(cardController *control.CardController) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		logging.Log.Debug("", "vars", vars)
		if _, ok := cardController.Cards[vars["id"]]; !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		encoder := json.NewEncoder(w)
		encoder.Encode(cardController.Cards[vars["id"]])
	})
}

func AddCard(cardController *control.CardController) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	})
}

func DelCard(cardController *control.CardController) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		logging.Log.Debug("", "vars", vars)
		cardController.DelCard(vars["id"])
		encoder := json.NewEncoder(w)
		encoder.Encode(cardController.Cards)
	})
}

func PlayCard(
	chromecastControl *control.ChromecastControl,
	cardController *control.CardController) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		if _, ok := cardController.Cards[vars["id"]]; !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		card := cardController.Cards[vars["id"]]
		chromecastControl.PlayCard(card)
		w.WriteHeader(http.StatusAccepted)
	})
}

func GetCasts(chromecastControl *control.ChromecastControl) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encoder := json.NewEncoder(w)
		if err := encoder.Encode(chromecastControl.GetClients()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})
}

func DiscoverCasts(chromecastControl *control.ChromecastControl) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chromecastControl.StartDiscovery(control.DISCOVERY_TIMEOUT * time.Second)
		w.WriteHeader(http.StatusAccepted)
	})
}

func CastStatus(
	chromecastControl *control.ChromecastControl,
	cardController *control.CardController) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := chromecastControl.CastStatus()
		encoder := json.NewEncoder(w)
		encoder.Encode(status)
	})
}

func ControlCasts(chromecastControl *control.ChromecastControl) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := control.ClientAction{}
		decoder := json.NewDecoder(r.Body)
		defer r.Body.Close()
		if err := decoder.Decode(&payload); err != nil {
			logging.Log.Error("control cast", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if !chromecastControl.ClientControl(payload) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
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

func Debug(w http.ResponseWriter, r *http.Request) {
	numGoroutines := runtime.NumGoroutine()
	responseText := fmt.Sprintf("Number of goroutines: %d\n", numGoroutines)
	io.WriteString(w, responseText)
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

func StartApp(
	host string,
	port int,
	cardController *control.CardController,
	chromcastController *control.ChromecastControl) {

	r := mux.NewRouter()
	r.Use(noCache)
	r.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/",
			http.FileServer(http.Dir("static"))))
	r.HandleFunc("/", SiteHandler).Methods("GET")
	apiPrefix := r.PathPrefix("/api").Subrouter()
	apiPrefix.Use(ContentJson)
	apiPrefix.HandleFunc("/cards", GetCards(cardController)).Methods("GET")
	apiPrefix.HandleFunc("/casts", GetCasts(chromcastController)).Methods("GET")
	apiPrefix.HandleFunc("/casts", DiscoverCasts(chromcastController)).Methods("POST")
	apiPrefix.HandleFunc("/casts", ControlCasts(chromcastController)).Methods("PUT")
	// apiPrefix.HandleFunc("/casts/{name}", GetCastStatus(chromcastController)).Methods("GET")
	// apiPrefix.HandleFunc("/casts/{name}", CastStatus(chromcastController)).Methods("GET")
	apiPrefix.HandleFunc("/cards", AddCard(cardController)).Methods("POST")
	apiPrefix.HandleFunc("/cards/{id}", DelCard(cardController)).Methods("DELETE")
	apiPrefix.HandleFunc("/cards/{id}", GetCard(cardController)).Methods("GET")
	apiPrefix.HandleFunc("/status", CastStatus(chromcastController, cardController)).Methods("GET")
	apiPrefix.HandleFunc("/cards/{id}", PlayCard(chromcastController, cardController)).Methods("POST")
	apiPrefix.HandleFunc("/debug", Debug).Methods("GET")

	srv := &http.Server{
		Handler:      r,
		Addr:         fmt.Sprintf("%s:%d", host, port),
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
