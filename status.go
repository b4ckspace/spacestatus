package main

import (
	"net/http"
	"net/url"
	"sync"
	"text/template"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"

	"github.com/b4ckspace/spacestatus/filters"
	"github.com/b4ckspace/spacestatus/metrics"
)

type server struct {
	MqttURL *url.URL `envconfig:"MQTT_URL"`
	Debug   bool     `envconfig:"DEBUG"`

	Cache *sync.Map

	mux      *http.ServeMux
	template *template.Template
}

func main() {
	s := &server{}
	s.mux = http.NewServeMux()
	err := envconfig.Process("", s)
	if err != nil {
		log.WithError(err).Fatalf("unable to process env")
	}
	if s.Debug {
		log.SetLevel(log.DebugLevel)
	}

	// mqtt
	err = s.connectMqtt()
	if err != nil {
		log.WithError(err).Fatalf("unable to connect mqtt")
	}

	// template
	err = s.loadTemplates()
	if err != nil {
		log.WithError(err).Fatalf("unable to load templates")
	}

	// metrics
	metrics.Register(s.mux)

	// serve http
	s.serve()
}

func (s *server) connectMqtt() (err error) {
	m := mqtt.NewClient(&mqtt.ClientOptions{
		Servers:       []*url.URL{s.MqttURL},
		ClientID:      "go-mqtt-spacestatus-dev",
		AutoReconnect: true,
		OnConnect: func(c mqtt.Client) {
			metrics.Count("spacestatus_mqtt{state=\"connected\"}")
			log.Infof("connected")
		},
		OnConnectionLost: func(c mqtt.Client, err error) {
			metrics.Count("spacestatus_mqtt{state=\"disconnected\"}")
			log.WithError(err).Errorf("connection lost")
		},
	})
	t := m.Connect()
	_ = t.Wait()
	if err := t.Error(); err != nil {
		return err
	}
	t = m.Subscribe("#", 0, func(c mqtt.Client, m mqtt.Message) {
		metrics.Count("spacestatus_mqtt{state=\"message\"}")
		log.Debugf("%s: %s", m.Topic(), string(m.Payload()))
		s.Cache.Store(m.Topic(), string(m.Payload()))
	})
	t.Wait()
	if err := t.Error(); err != nil {
		return err
	}
	log.Println("subscribed")
	return
}

func (s *server) loadTemplates() (err error) {
	s.template, err = template.New("base").Funcs(template.FuncMap{
		"mqtt":    filters.MqttLoadForCache(s.Cache),
		"csvlist": filters.CsvList,
		"jsonize": filters.Jsonize,
	}).ParseFiles("status-template.json")
	return
}

func (s *server) serve() (err error) {
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		metrics.Count("spacestatus_requests")
		w.Header().Add("content-type", "application/json")
		err := s.template.ExecuteTemplate(w, "status-template.json", nil)
		if err != nil {
			log.WithError(err).Infof("unable to render template")
		}
	})
	s.mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {})
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			log.WithFields(log.Fields{
				"duration": time.Since(start).String(),
				"method":   r.Method,
			}).Info(r.RequestURI)
		})
	}
	return http.ListenAndServe(":8080", middleware(s.mux))
}
