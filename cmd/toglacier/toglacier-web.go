package main

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/rafaeljusto/toglacier/internal/config"
	"github.com/rafaeljusto/toglacier/internal/storage"
)

func startWEB() *http.Server {
	server := &http.Server{
		Addr: config.Current().WEB.Address,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t := template.New("homepage")
		t, err := t.Parse(webHomepageTemplate)

		if err != nil {
			logger.Warningf("error parsing homepage template. details: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err = t.Execute(w, nil); err != nil {
			logger.Warningf("error executing homepage template. details: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/backups", func(w http.ResponseWriter, r *http.Request) {
		backups, err := toGlacier.ListBackups(false)
		if err != nil {
			logger.Warningf("error retrieving backups. details: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if backups == nil {
			// initialize slice to avoid showing null in json
			backups = make(storage.Backups, 0)
		}

		encoder := json.NewEncoder(w)
		if err = encoder.Encode(backups); err != nil {
			logger.Warningf("error marshalling backups. details: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		encoder := json.NewEncoder(w)
		if err := encoder.Encode(config.Current()); err != nil {
			logger.Warningf("error marshalling backups. details: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Warningf("error listening web server. details: %s", err)
		}
	}()

	return server
}
