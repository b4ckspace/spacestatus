package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"text/template"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"

)

type config struct {
	server *url.URL
	debug  bool
}

var (
	c      config
	cache  map[string]string
	update chan mqtt.Message
)

func main() {
	// exit handler
	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt)

	// cache
	cache = map[string]string{}
	update = make(chan mqtt.Message)
	go func() {
		for m := range update {
			cache[m.Topic()] = string(m.Payload())
		}
	}()

	// config
	err := configure()
	if err != nil {
		log.Fatal(err)
	}

	// mqtt
	err = setupMqtt()
	if err != nil {
		log.Fatal(err)
	}

	// template
	tmpl, err := loadTemplates()
	if err != nil {
		log.Fatal(err)
	}

	// metrics
	metrics.Register()

	// serve http
	serve(tmpl)
	<-s
	log.Info("exiting...")
}

func getEnv(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func setupMqtt() error {
	m := mqtt.NewClient(&mqtt.ClientOptions{
		Servers:       []*url.URL{c.server},
		ClientID:      "go-mqtt-spacestatus",
		AutoReconnect: true,
		OnConnect: func(c mqtt.Client) {
			log.Info("connected")
		},
		OnConnectionLost: func(c mqtt.Client, err error) {
			log.Errorf("connection lost: %v", err)
		},
	})
	t := m.Connect()
	for !t.WaitTimeout(1 * time.Second) {
		log.Println("waiting for mqtt")
	}
	if err := t.Error(); err != nil {
		return err
	}

	m.Subscribe("#", 0, func(c mqtt.Client, m mqtt.Message) {
		log.Debugf("%s: %s", m.Topic(), string(m.Payload()))
		update <- m
	})
	if err := t.Error(); err != nil {
		return err
	}

	log.Println("subscribed")
	return nil
}

func loadTemplates() (*template.Template, error) {
	funcMap := template.FuncMap{
		"mqtt": func(t string) string {
			value := cache[t]
			return value
		},
		"csvlen": func(csv string) string {
			l := strings.Split(csv, ", ")
			return fmt.Sprintf("%d", len(l))
		},
		"csvlist": func(csv string) string {
			l := strings.Split(csv, ", ")
			b, _ := json.Marshal(l)
			return string(b)
		},
	}
	return template.New("base").Funcs(funcMap).ParseFiles("status-template.json")
}

func serve(tmpl *template.Template) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("content-type", "application/json")
		_ = tmpl.ExecuteTemplate(w, "status-template.json", nil)
	})
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {})
	go func() {
		err := http.ListenAndServe(":8080", nil)
		if err != nil {
			log.Fatal(err)
		}
	}()
}

func configure() (err error) {
	c = config{}
	server := getEnv("MQTT_URL", "tcp://mqtt:1883")
	c.server, err = url.Parse(server)
	if err != nil {
		return err
	}
	_, c.debug = os.LookupEnv("DEBUG")
	if c.debug {
		log.SetLevel(log.DebugLevel)
	}
	return nil
}
