package metrics

import (
	"fmt"
	"net/http"
	"sync"
)

var (
	lock    = sync.RWMutex{}
	counter = map[string]int{}
)

// non-blocking call to count metrics
func Count(metric string) {
	go func() {
		lock.Lock()
		defer lock.Unlock()
		_, found := counter[metric]
		if found {
			counter[metric]++
		} else {
			counter[metric] = 1
		}
	}()
}

func Set(metric string, value int) {
	go func() {
		lock.Lock()
		defer lock.Unlock()
		counter[metric] = value
	}()
}

func Register(mux *http.ServeMux) {
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		lock.Lock()
		defer lock.Unlock()
		for k, v := range counter {
			fmt.Fprintf(w, "%s %d\n", k, v)
		}
	})
}
