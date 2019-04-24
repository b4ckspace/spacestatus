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

var (
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

	// open mqtt connection
	u, err := url.Parse("tcp://mqtt.core.bckspc.de:1883")
	if err != nil {
		log.Fatal(err)
	}
	m := mqtt.NewClient(&mqtt.ClientOptions{
		Servers:          []*url.URL{u},
		ClientID:         "go-mqtt-spacestatus",
		AutoReconnect:    true,
		OnConnect:        func(c mqtt.Client) { log.Info("connected") },
		OnConnectionLost: func(c mqtt.Client, err error) { log.Errorf("connection lost: %v", err) },
	})
	t := m.Connect()
	for !t.WaitTimeout(1 * time.Second) {
		log.Println("waiting for mqtt")
	}
	if err := t.Error(); err != nil {
		log.Fatal(err)
	}

	// subscribe to all mqtt topics
	m.Subscribe("#", 0, func(c mqtt.Client, m mqtt.Message) {
		update <- m
	})
	log.Println("subscribed")

	// template
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
	tmpl, err := template.New("base").Funcs(funcMap).ParseFiles("status-template.json")
	if err != nil {
		log.Fatal(err)
	}

	// serve http
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("content-type", "application/json")
		_ = tmpl.ExecuteTemplate(w, "status-template.json", map[string]string{"test": "test"})
	})
	go func() {
		err = http.ListenAndServe(":8080", nil)
		if err != nil {
			log.Fatal(err)
		}
	}()

	<-s
	log.Info("exiting...")
}
