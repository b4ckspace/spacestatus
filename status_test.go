package main

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/b4ckspace/spacestatus/server"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/go-cmp/cmp"
	log "github.com/sirupsen/logrus"
)

func TestMqtt(t *testing.T) {
	s, err := server.NewServer()
	if err != nil {
		log.Fatal(err)
	}

	m := mqtt.NewClient(&mqtt.ClientOptions{
		Servers:          []*url.URL{s.MqttURL},
		ClientID:         "go-mqtt-spacestatus-test",
		AutoReconnect:    true,
		OnConnect:        func(c mqtt.Client) { log.Info("connected") },
		OnConnectionLost: func(c mqtt.Client, err error) { log.Errorf("connection lost: %v", err) },
	})
	token := m.Connect()
	log.Infof("%+v", token)

	go main()
	<-time.After(1 * time.Second)

	_ = m.Publish("sensor/space/member/names", 0, false, "a, b, c, d")
	_ = m.Publish("sensor/space/member/present", 0, false, "4")
	_ = m.Publish("sensor/space/member/count", 0, false, "30")
	_ = m.Publish("sensor/temperature/lounge/podest", 0, false, "23.3")
	_ = m.Publish("sensor/temperature/lounge/ceiling", 0, false, "24.3")
	_ = m.Publish("sensor/temperature/hackcenter/shelf", 0, false, "21.3")
	_ = m.Publish("sensor/power/main/L1", 0, false, "123")
	_ = m.Publish("sensor/power/main/L2", 0, false, "234")
	_ = m.Publish("sensor/power/main/L3", 0, false, "345")
	_ = m.Publish("sensor/power/main/total", 0, false, "1234")
	_ = m.Publish("sensor/space/status", 0, false, "closed")
	_ = m.Publish("sensor/space/member/deviceCount", 0, false, "77")

	resp, err := http.Get("http://localhost:8080/")
	if err != nil {
		t.Errorf("Unable to query API: %v", err)
	}

	have, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("Unable to load body: %v", err)
	}

	want, err := ioutil.ReadFile("status_test.json")
	if err != nil {
		t.Errorf("Unable to load want: %v", err)
	}

	wants := strings.Split(string(want), "\n")
	haves := strings.Split(string(have), "\n")
	if diff := cmp.Diff(wants, haves); diff != "" {
		t.Errorf("Invalid output. \n%s", diff)
	}
}
