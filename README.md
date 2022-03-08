[![Build Status](https://cloud.drone.io/api/badges/b4ckspace/spacestatus/status.svg)](https://cloud.drone.io/b4ckspace/spacestatus)

# go-mqtt-spacestatus

A simple Go implementation of *spaceapi* that watches all MQTT topics and caches the last values.

### Build

Simply run `go build`.

### Usage

Edit the `status-template.json` (go-template syntax) to your needs and run the tool. Configuration options can be set via environment variables.

* `MQTT_URL`: URL of the MQTT server (default: `tcp://mqtt.core.bckspc.de:1883`)
* `MQTT_CLIENT_ID`: set MQTT client id - must be unique! (default: `go-mqtt-spacestatus-dev`)
* `DEBUG`: print MQTT topic changes, enabled when set, regardless of value

### Limitations

Currently it's not possible to limit the MQTT topics cached.
