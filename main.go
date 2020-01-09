package main

import (
	log "github.com/sirupsen/logrus"

	"github.com/b4ckspace/spacestatus/metrics"
	"github.com/b4ckspace/spacestatus/server"
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})

	// server
	s, err := server.NewServer()
	if err != nil {
		log.WithError(err).Fatalf("unable to process env")
	}

	// mqtt
	err = s.ConnectMqtt()
	if err != nil {
		log.WithError(err).Fatalf("unable to connect mqtt")
	}

	// template
	err = s.LoadTemplates()
	if err != nil {
		log.WithError(err).Fatalf("unable to load templates")
	}

	// metrics
	metrics.Register(s.GetMux())

	// serve http
	err = s.ListenAndServe()
	if err != nil {
		log.WithError(err).Fatalf("unable to listen")
	}
}
