// Copyright (c) 2018 cloud-spin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

const (
	// DefaultPort holds the default port the server will listen on.
	DefaultPort = 9090

	// DefaultShutdownTimeout holds the timeout to shutdown the server.
	DefaultShutdownTimeout = 10 * time.Second

	// DefaultReadTimeout holds the default read timeout.
	DefaultReadTimeout = 15 * time.Second

	// DefaultWriteTimeout holds the default write timeout.
	DefaultWriteTimeout = 15 * time.Second

	// DefaultPingEndpoint holds the default ping endpoint.
	DefaultPingEndpoint = "/ping"

	// DefaultHealthcheckEndpoint holds the default healtcheck endpoint.
	DefaultHealthcheckEndpoint = "/healthcheck"

	// DefaultShutdownEndpoint holds the default shutdown endpoint.
	DefaultShutdownEndpoint = "/shutdown"

	// stopSignal signals the Stop method was called and the server should stop.
	stopSignal = syscall.Signal(0x99)
)

// ShutdownHandler is fired when the server should be shutdown.
type ShutdownHandler = func(s *http.Server, ctx context.Context) error

// Configs holds server specific configs.
// Port holds the server port.
// ShutdownTimeout holds the timeout to shutdown the server.
// ReadTimeout holds the read timeout.
// WriteTimeout holds the write timeout.
// PingEndpoint holds the ping endpoint.
// HealthcheckEndpoint holds the healthcheck endpoint.
// ShutdownEndpoint holds the shutdown endpoint.
type Configs struct {
	Port                int
	ShutdownTimeout     time.Duration
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	PingEndpoint        string
	HealthcheckEndpoint string
	ShutdownEndpoint    string
}

// Server represents a HTTP server.
type Server interface {
	Start() error
	Stop() error
	GetHTTPServer() *http.Server
	RegisterOnShutdown(f func())
	RegisterServerStartHandler(f func(s *http.Server) error)
	RegisterHealthcheckEndpoint(path string, handler func(w http.ResponseWriter, r *http.Request))
	RegisterServerShutdownHandler(f ShutdownHandler)
}

// ServerImpl implements a HTTP Server.
type ServerImpl struct {
	Configs               *Configs
	Router                *mux.Router
	HTTPServer            *http.Server
	healthcheckHandler    func(w http.ResponseWriter, r *http.Request)
	serverStartHandler    func(s *http.Server) error
	serverShutdownHandler ShutdownHandler
	stop                  chan os.Signal
	stopError             chan error
	pingEndpoint          string
	healthcheckEndpoint   string
	shutdownEndpoint      string
}

// NewConfigs initializes a new instance of Configs with default values.
func NewConfigs() *Configs {
	return &Configs{
		Port:                DefaultPort,
		ShutdownTimeout:     DefaultShutdownTimeout,
		ReadTimeout:         DefaultReadTimeout,
		WriteTimeout:        DefaultWriteTimeout,
		PingEndpoint:        DefaultPingEndpoint,
		HealthcheckEndpoint: DefaultHealthcheckEndpoint,
		ShutdownEndpoint:    DefaultShutdownEndpoint,
	}
}

// New initializes a new instance of Server.
func New(configs *Configs, router *mux.Router) Server {
	server := &ServerImpl{
		Configs:             configs,
		Router:              router,
		HTTPServer:          newHTTPServer(configs, router),
		pingEndpoint:        configs.PingEndpoint,
		healthcheckEndpoint: configs.HealthcheckEndpoint,
		shutdownEndpoint:    configs.ShutdownEndpoint,
	}
	if server.pingEndpoint == "" {
		server.pingEndpoint = DefaultPingEndpoint
	}
	if server.healthcheckEndpoint == "" {
		server.healthcheckEndpoint = DefaultHealthcheckEndpoint
	}
	if server.shutdownEndpoint == "" {
		server.shutdownEndpoint = DefaultShutdownEndpoint
	}

	router.Path(server.pingEndpoint).Name(server.pingEndpoint).Methods("GET").HandlerFunc(server.handleFuncPing)
	router.Path(server.healthcheckEndpoint).Name(server.healthcheckEndpoint).Methods("GET").HandlerFunc(server.handleFuncHealthcheck)
	router.Path(server.shutdownEndpoint).Name(server.shutdownEndpoint).Methods("GET").HandlerFunc(server.handleFuncShutdown)

	return server
}

// RegisterHealthcheckEndpoint register the handler to handle healthcheck responses.
func (s *ServerImpl) RegisterHealthcheckEndpoint(path string, handler func(w http.ResponseWriter, r *http.Request)) {
	s.healthcheckEndpoint = path
	s.healthcheckHandler = handler
	s.Router.Path(path).Name(path).Methods("GET").HandlerFunc(s.handleFuncHealthcheck)
}

// RegisterOnShutdown registers a function to call on Shutdown. It delegates the calls to the standard http.Server package.
func (s *ServerImpl) RegisterOnShutdown(f func()) {
	s.HTTPServer.RegisterOnShutdown(f)
}

// RegisterServerStartHandler registers a function that should start the HTTP server.
func (s *ServerImpl) RegisterServerStartHandler(f func(s *http.Server) error) {
	s.serverStartHandler = f
}

// RegisterServerShutdownHandler registers a function that should shutdown the HTTP server.
func (s *ServerImpl) RegisterServerShutdownHandler(f ShutdownHandler) {
	s.serverShutdownHandler = f
}

// Start starts the server and blocks, listening for requests.
func (s *ServerImpl) Start() error {
	s.stop = make(chan os.Signal)
	s.stopError = make(chan error)
	signal.Notify(s.stop, os.Interrupt, stopSignal)
	var serveError error

	go func() {
		if err := s.startHTTPServer(); err != nil {
			if err != http.ErrServerClosed {
				// Force shutdown so this method can return with the serve (original) error.
				serveError = err
				s.stop <- os.Interrupt
			}
		}
	}()

	signal := <-s.stop

	timeoutContext, cancel := context.WithTimeout(context.Background(), s.Configs.ShutdownTimeout)
	defer cancel()

	err := s.shutdownHTTPServer(timeoutContext)

	// If Stop() was called, doesn't return any error here. Any errors after Stop() was called will be returned only in the Stop() method.
	var origErr error
	if signal == stopSignal {
		s.stopError <- err
	} else {
		origErr = serveError
	}

	return origErr
}

// Stop stops the server gracefully and synchronously, returning any error detected during shutdown.
// The ShutdownTimeout is respected for all in-flight requests. When the server is no longer processing any requests,
// Stop() will return and the server won't listen for requests anymore.
func (s *ServerImpl) Stop() error {
	if s.stop != nil {
		s.stop <- stopSignal
		return <-s.stopError
	}
	return nil
}

// GetHTTPServer returns the HTTP server instance,
func (s *ServerImpl) GetHTTPServer() *http.Server {
	return s.HTTPServer
}

func (s *ServerImpl) startHTTPServer() error {
	if s.serverStartHandler != nil {
		return s.serverStartHandler(s.HTTPServer)
	}
	return s.HTTPServer.ListenAndServe()
}

func (s *ServerImpl) shutdownHTTPServer(ctx context.Context) error {
	if s.serverShutdownHandler != nil {
		return s.serverShutdownHandler(s.HTTPServer, ctx)
	}
	return s.HTTPServer.Shutdown(ctx)
}

func newHTTPServer(configs *Configs, router *mux.Router) *http.Server {
	port := DefaultPort
	if configs.Port != 0 {
		port = configs.Port
	}
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      router,
		WriteTimeout: configs.WriteTimeout,
		ReadTimeout:  configs.ReadTimeout,
	}
	return server
}

func (s *ServerImpl) handleFuncPing(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}

func (s *ServerImpl) handleFuncHealthcheck(w http.ResponseWriter, r *http.Request) {
	if s.healthcheckHandler != nil {
		s.healthcheckHandler(w, r)
	} else {
		w.WriteHeader(200)
	}
}

func (s *ServerImpl) handleFuncShutdown(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	go s.Stop()
}
