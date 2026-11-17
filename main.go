package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	config := flag.String("config", "", "Path to the config.json file.")
	help := flag.Bool("help", false, "Print this message.")

	flag.Parse()

	if *help || *config == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	handler, err := NewHandler(*config)
	if err != nil {
		panic(err)
	}

	fmt.Println("Loaded")

	http.HandleFunc("GET /dyndns", handler.DynDNSHandler)

	const KiB = 1 << 10

	srv := &http.Server{
		Addr:              "127.0.1.1:8080",
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      35 * time.Second, // > cloudflare calls + retries
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 * KiB,
	}

	err = srv.ListenAndServe()

	if err != nil {
		panic(err)
	}
}
