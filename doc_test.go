package server_test

import (
	"fmt"
	"log"
	"net/http"

	"github.com/cloud-spin/server"
	"github.com/gorilla/mux"
)

func Example() {
	configs := server.NewConfigs()
	s := server.NewServer(configs, mux.NewRouter())

	go func() {
		if err := s.Start(); err != nil {
			log.Println("Error serving requests")
			log.Fatal(err)
		}
	}()

	// Test if the server was initialized successfully by pinging its "/ping" endpoint.
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/%s", configs.Port, configs.PingEndpoint))
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println(resp.StatusCode)
	}

	s.Stop()

	// Output: 200
}
