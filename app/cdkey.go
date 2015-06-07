package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"text/template"

	"github.com/yxpod/cdkey"
)

var (
	port = flag.String("p", ":8080", "http port")
	dir  = flag.String("d", "/home/cdkey", "cdkey db directory")
)

type logger struct{}

func (l logger) Info(v ...interface{}) {
	log.Println("[INFO] ", fmt.Sprint(v...))
}

func (l logger) Error(v ...interface{}) {
	log.Println("[ERROR]", fmt.Sprint(v...))
}

func main() {
	flag.Parse()

	cdkey.SetLogger(&logger{})

	server := cdkey.NewServer(*dir)
	if server == nil {
		os.Exit(1)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	go func() {
		m := http.NewServeMux()
		m.HandleFunc("/packs", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "static/packs.html")
		})

		m.HandleFunc("/keys", func(w http.ResponseWriter, r *http.Request) {
			r.ParseForm()
			packName := r.FormValue("pack")

			keys, err := server.ListKeys(packName)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotAcceptable)
				return
			}

			t, err := template.ParseFiles("static/keys.tpl.html")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

			t.Execute(w, struct {
				PackName string
				Keys     []cdkey.KeyInfo
			}{
				PackName: packName,
				Keys:     keys,
			})
		})

		m.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.Dir("static"))))
		m.Handle("/", server.HTTPServeMux())

		log.Println("[APP]   HTTP server listen on", *port)
		log.Fatal(http.ListenAndServe(*port, m))
	}()

	<-c
	server.Stop()
}
