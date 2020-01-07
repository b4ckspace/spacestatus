package filters

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/b4ckspace/spacestatus/metrics"
)

func MqttLoadForCache(cache *sync.Map) func(string) string {
	return func(t string) string {
		value, found := cache.Load(t)
		if found {
			metrics.Count("spacestatus_mqtt_query{state=\"success\"}")
		} else {
			metrics.Count("spacestatus_mqtt_query{state=\"failed\"}")
			metrics.Count(fmt.Sprintf("spacestatus_mqtt_query_fails{state=\"%s\"}", t))
		}
		valueStr, ok := value.(string)
		if !ok {
			valueStr = fmt.Sprintf("%v", value)
		}
		return valueStr
	}
}

func CsvList(csv string) []string {
	if csv == "" {
		return []string{}
	}
	return strings.Split(csv, ", ")
}

func Jsonize(mustType string, data interface{}) string {
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
		log.Printf("invalid format for jsonize, expected %s, data is %v", mustType, oldData)
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		log.Printf("unable to jsonize %v", data)
	}
	return string(encoded)
}
