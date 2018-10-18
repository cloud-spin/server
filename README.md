# Server [![Build Status](https://travis-ci.com/cloud-spin/server.svg?branch=master)](https://travis-ci.com/cloud-spin/server) [![codecov](https://codecov.io/gh/cloud-spin/server/branch/master/graph/badge.svg)](https://codecov.io/gh/cloud-spin/server) [![Go Report Card](https://goreportcard.com/badge/github.com/cloud-spin/server)](https://goreportcard.com/report/github.com/cloud-spin/server) [![GoDoc](https://godoc.org/github.com/cloud-spin/server?status.svg)](https://godoc.org/github.com/cloud-spin/server)

Server exposes a reusable HTTP server with graceful shutdown and preconfigured ping, health check and shutdown endpoints. Server uses
[gorilla/mux](https://github.com/gorilla/mux) as its main request router.
 
#### Install

From a configured [Go environment](https://golang.org/doc/install#testing):
```sh
go get -u github.com/cloud-spin/server
```

If you are using dep:
```sh
dep ensure -add github.com/cloud-spin/server
```

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
	s := server.New(configs, mux.NewRouter())

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


## Handlers

Package server uses [http.ListenAndSeve](https://golang.org/pkg/net/http/#ListenAndServe) by default to start the HTTP server. However, this can be overriden to be used any other http methods to start the server such as [http.ListenAndServeTLS](https://golang.org/pkg/net/http/#ListenAndServeTLS) or any other custom function. Below code register a custom handler and start the server with http.ListenAndServeTLS instead of the default http.ListenAndServe.

```go
...
s := server.New(configs, router)
s.RegisterServerStartHandler(func(s *http.Server) error {
	return s.ListenAndServeTLS(...)
})

go func() {
	if err := s.Start(); err != nil {
		log.Println("Error serving requests")
		log.Fatal(err)
	}
}()
```

Package server also provides a shutdown hook that can be used to release the system resources at shutdown time. Below code register a custom shutdown handler that gets executed when the http server is shutting down.

```go
...
s := server.New(configs, router)
s.RegisterOnShutdown(func() {
	fmt.Println("shutting down server")
})
		
go func() {
	if err := s.Start(); err != nil {
		log.Println("Error serving requests")
		log.Fatal(err)
	}
}()
```


## License
MIT, see [LICENSE](LICENSE).

"Use, abuse, have fun and contribute back!"


## Contributions
See [contributing.md](https://github.com/cloud-spin/docs/blob/master/contributing.md).

