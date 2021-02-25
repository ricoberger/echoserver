package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"strconv"
	"time"
)

const (
	listenAddress = ":8080"
)

var (
	randomStatusCodes = []int{200, 200, 200, 200, 200, 400, 500, 502, 503}
)

func main() {
	router := http.NewServeMux()

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("host: %s, address: %s, method: %s, requestURI: %s, proto: %s, useragent: %s", r.Host, r.RemoteAddr, r.Method, r.RequestURI, r.Proto, r.UserAgent())

		dump, err := httputil.DumpRequest(r, true)
		if err != nil {
			http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "%q", dump)
	})

	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK")
	})

	router.HandleFunc("/status", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("host: %s, address: %s, method: %s, requestURI: %s, proto: %s, useragent: %s", r.Host, r.RemoteAddr, r.Method, r.RequestURI, r.Proto, r.UserAgent())

		status, ok := r.URL.Query()["status"]
		if !ok || len(status[0]) < 1 || status[0] == "random" {
			index := rand.Intn(len(randomStatusCodes))
			w.WriteHeader(randomStatusCodes[index])
			return
		}

		s, err := strconv.Atoi(status[0])
		if err == nil {
			w.WriteHeader(s)
			return
		}

		http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
	}))

	router.HandleFunc("/timeout", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("host: %s, address: %s, method: %s, requestURI: %s, proto: %s, useragent: %s", r.Host, r.RemoteAddr, r.Method, r.RequestURI, r.Proto, r.UserAgent())

		timeoutString, ok := r.URL.Query()["timeout"]
		if !ok || len(timeoutString) < 1 {
			w.WriteHeader(200)
			return
		}

		timeout, err := time.ParseDuration(timeoutString[0])
		if err != nil {
			http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
			return
		}

		time.Sleep(timeout)
		w.WriteHeader(200)
	})

	server := &http.Server{
		Addr:    listenAddress,
		Handler: router,
	}

	log.Printf("Server listen on: %s", listenAddress)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("HTTP server died unexpected: %s", err.Error())
	}
}
