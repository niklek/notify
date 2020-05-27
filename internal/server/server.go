package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"sync/atomic"
)

var totalRequests int32

func inc() {
	atomic.AddInt32(&totalRequests, 1)
}

func main() {

	http.HandleFunc("/notify", func(w http.ResponseWriter, r *http.Request) {
		inc()

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.Println(totalRequests, string(body))

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("can not start server")
	}
}
