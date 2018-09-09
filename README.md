# Server [![Build Status](https://travis-ci.com/cloud-spin/server.svg?branch=master)](https://travis-ci.com/cloud-spin/server) [![codecov](https://codecov.io/gh/cloud-spin/server/branch/master/graph/badge.svg)](https://codecov.io/gh/cloud-spin/server) [![Go Report Card](https://goreportcard.com/badge/github.com/cloud-spin/server)](https://goreportcard.com/report/github.com/cloud-spin/server) [![GoDoc](https://godoc.org/github.com/cloud-spin/server?status.svg)](https://godoc.org/github.com/cloud-spin/server)

Server exposes a reusable HTTP server with graceful shutdown and preconfigured ping, health check and shutdown endpoints. Server uses
[gorilla/mux](https://github.com/gorilla/mux) as its main request router.

#### How to Use

Below example starts a fully working HTTP server, test it by pinging its pre-configured "/ping" endpoint and shuts it down gracefully afterwards.

```go
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/cloud-spin/server"
	"github.com/gorilla/mux"
)

func main() {
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
}
```

Output:
```
200
```

Also refer to the tests at [server_test.go](server_test.go).
