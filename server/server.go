package server

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

type Server struct {
	MqttURL *url.URL `envconfig:"MQTT_URL" default:"tcp://mqtt:1883"`
	Listen  string   `envconfig:"LISTEN" default:":8080"`
	Debug   bool     `envconfig:"DEBUG"`

	Cache *sync.Map

	mux      *http.ServeMux
	template *template.Template
}

func NewServer() (s *Server, err error) {
	s = &Server{}
	s.mux = http.NewServeMux()
	err = envconfig.Process("", s)
	if err != nil {
		return nil, err
	}
	if s.Debug {
		log.SetLevel(log.DebugLevel)
	}
	return s, nil
}

// ConnectMqtt connects to mqtt
func (s *Server) ConnectMqtt() (err error) {
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

// LoadTemplates loads the template filters and files
func (s *Server) LoadTemplates() (err error) {
	s.template, err = template.New("base").Funcs(template.FuncMap{
		"mqtt":    filters.MqttLoadForCache(s.Cache),
		"csvlist": filters.CsvList,
		"jsonize": filters.Jsonize,
	}).ParseFiles("templates/status.json")
	if err != nil {
		return err
	}
	return
}

// Serve handles http
func (s *Server) ListenAndServe() (err error) {
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		metrics.Count("spacestatus_requests")
		w.Header().Add("content-type", "application/json")
		err := s.template.ExecuteTemplate(w, "status.json", nil)
		if err != nil {
			log.WithError(err).Infof("unable to render template")
		}
	})
	s.mux.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.Dir("static"))))
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
	log.WithField("addr", s.Listen).Info("listening")
	return http.ListenAndServe(s.Listen, middleware(s.mux))
}

// GetMux returns the http.ServeMux to add additional routes
func (s *Server) GetMux() *http.ServeMux {
	return s.mux
}
