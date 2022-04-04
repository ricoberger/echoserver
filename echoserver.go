package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
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

		fmt.Fprintf(w, "%s", string(dump))
	})

	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK")
	})

	router.HandleFunc("/status", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("host: %s, address: %s, method: %s, requestURI: %s, proto: %s, useragent: %s", r.Host, r.RemoteAddr, r.Method, r.RequestURI, r.Proto, r.UserAgent())

		statusString := r.URL.Query().Get("status")
		if statusString == "" || statusString == "random" {
			index := rand.Intn(len(randomStatusCodes))
			w.WriteHeader(randomStatusCodes[index])
			return
		}

		status, err := strconv.Atoi(statusString)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(status)
	}))

	router.HandleFunc("/timeout", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("host: %s, address: %s, method: %s, requestURI: %s, proto: %s, useragent: %s", r.Host, r.RemoteAddr, r.Method, r.RequestURI, r.Proto, r.UserAgent())

		timeoutString := r.URL.Query().Get("timeout")
		if timeoutString == "" {
			http.Error(w, "timout parameter is missing", http.StatusBadRequest)
			return
		}

		timeout, err := time.ParseDuration(timeoutString)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		time.Sleep(timeout)
		w.WriteHeader(200)
	})

	router.HandleFunc("/headersize", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("host: %s, address: %s, method: %s, requestURI: %s, proto: %s, useragent: %s", r.Host, r.RemoteAddr, r.Method, r.RequestURI, r.Proto, r.UserAgent())

		headerSizeString := r.URL.Query().Get("size")
		if headerSizeString == "" {
			http.Error(w, "size parameter is missing", http.StatusBadRequest)
			return
		}

		size, err := strconv.Atoi(headerSizeString)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Add("X-Header-Size", strings.Repeat("0", size))
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
