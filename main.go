package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/z3le/alien-buddy/content"
	"github.com/z3le/alien-buddy/display"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	phrasesFile := flag.String("phrases", "phrases.json", "path to phrases file")
	interval := flag.Duration("interval", 2*time.Hour, "rotation interval")
	fake := flag.Bool("fake", false, "use fake display (saves PNGs to ./frames/)")
	flag.Parse()

	// --- Display ---
	var disp display.Display
	if *fake {
		log.Println("using fake display (saving PNGs to ./frames/)")
		disp = display.NewFake("./frames")
	} else {
		d, err := display.NewWaveshare27()
		if err != nil {
			log.Fatalf("display init: %v", err)
		}
		defer d.Close()
		disp = d
	}

	renderer := display.NewRenderer()

	// --- Content ---
	phrases, err := content.LoadPhrases(*phrasesFile)
	if err != nil {
		log.Fatalf("load phrases: %v", err)
	}
	rotation := content.NewRotation(phrases, renderer, disp, *interval)

	// --- HTTP ---
	srv := NewServer(renderer, disp, rotation)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", srv.HandleStatus)
	mux.HandleFunc("GET /current", srv.HandleCurrent)
	mux.HandleFunc("POST /message", srv.HandleMessage)

	log.Printf("listening on %s ", *addr)
	go func() {
		if err := http.ListenAndServe(*addr, mux); err != nil {
			log.Fatal(err)
		}
	}()

	// --- Start rotation ---
	rotation.Start()
	defer rotation.Stop()

	// --- Wait for shutdown ---
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down")
	disp.Clear()
}
