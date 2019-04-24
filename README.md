# go-mqtt-spacestatus

A simple go implementation of *spaceapi* that whatches all mqtt topics and caches there last value.

### Usage

Edit the `status-template.json` (go-template syntax) to your needs and run the tool. One can configuraiton options using environment variables.

* `MQTT_URL`: url of the mqtt server (default: `tcp://mqtt.core.bckspc.de:1883`)
* `DEBUG`: print mqtt topic changes, enabled when set, regadless of value

### Limitations

Currently its not possible to limit the mqtt topics cached.
