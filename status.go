package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"text/template"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"

	"github.com/b4ckspace/spacestatus/metrics"
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
			metrics.Count("spacestatus_mqtt{state=\"connected\"}")
			log.Info("connected")
		},
		OnConnectionLost: func(c mqtt.Client, err error) {
			metrics.Count("spacestatus_mqtt{state=\"disconnected\"}")
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
		metrics.Count("spacestatus_mqtt{state=\"message\"}")
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
			value, found := cache[t]
			if found {
				metrics.Count("spacestatus_mqtt_query{state=\"success\"}")
			} else {
				metrics.Count("spacestatus_mqtt_query{state=\"failed\"}")
			}
			return value
		},
		"csvlist": func(csv string) []string {
			if csv == "" {
				return []string{}
			}
			return strings.Split(csv, ", ")
		},
		"jsonize": func(mustType string, data interface{}) string {
			var err error
			var dataString string
			oldData := data
			ok := false
			switch mustType {
			case "string":
				_, ok = data.(string)
				if !ok {
					data = ""
				}
			case "bool":
				_, ok = data.(bool)
				if !ok {
					dataString, ok = data.(string)
					data, err = strconv.ParseBool(dataString)
					if err != nil {
						ok = false
					}
				}
			case "int":
				_, ok = data.(int)
				if !ok {
					dataString, ok = data.(string)
					data, err = strconv.ParseInt(dataString, 10, 64)
					if err != nil {
						ok = false
					}
				}
			case "float":
				_, ok = data.(float64)
				if !ok {
					dataString, ok = data.(string)
					data, err = strconv.ParseFloat(dataString, 64)
					if err != nil {
						ok = false
					}
				}
			case "[]string":
				_, ok = data.([]string)
				if !ok {
					data = []string{}
				}
			case "[]bool":
				_, ok = data.([]bool)
				if !ok {
					data = []bool{}
				}
			case "[]int":
				_, ok = data.([]int)
				if !ok {
					data = []int{}
				}
			case "[]float":
				_, ok = data.([]float32)
				if !ok {
					data = []float32{}
				}
			}
			if !ok {
				log.Printf("Invalid format for jsonize, expected %s, data is %v", mustType, oldData)
			}
			encoded, err := json.Marshal(data)
			if err != nil {
				log.Printf("Unable to jsonize %v", data)
			}
			return string(encoded)
		},
	}
	return template.New("base").Funcs(funcMap).ParseFiles("status-template.json")
}

func serve(tmpl *template.Template) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		metrics.Count("spacestatus_requests")
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
