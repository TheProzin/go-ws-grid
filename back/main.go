package main

import (
	"crypto/tls"
	"go-ws-grid/controller"
	"go-ws-grid/model"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetPrefix("go-ws-grid: ")
	log.SetFlags(log.Ldate | log.Ltime)
	var err error

	go model.Retention(60 * time.Second)
	go model.VerificaClientesWs()

	r := mux.NewRouter()

	r.HandleFunc("/wsGrid", controller.WsEndpointHandler)
	r.HandleFunc("/getTokenWsGrid", controller.GetTokenWsHandler)

	port := ":" + model.GoDotEnvVariable("SERVER_PORT")
	srv := &http.Server{
		Addr:         port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		Handler:      r, // Use the router
		TLSConfig:    &tls.Config{},
	}

	err = srv.ListenAndServe()
	if err != nil {
		log.Println("Erro no server: ", err)
	}

	defer func() {
		if err := recover(); err != nil {
			log.Println("Recovered from panic na main:", err)
		}
	}()
}
