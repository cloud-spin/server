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
	"errors"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

const (
	maxRetries                = 3
	testServerEndpoint        = "http://localhost"
	customHealthcheckEndpoint = "/customhealthcheck"
)

var (
	testServerPort = 2000
)

func TestNewConfigsShouldReturnConfigsWithDefaultValuesSet(t *testing.T) {
	configs := NewConfigs()

	if configs.Port != DefaultPort {
		t.Errorf("Expected: %d; Got: %d", DefaultPort, configs.Port)
	}
	if configs.ShutdownTimeout != DefaultShutdownTimeout {
		t.Errorf("Expected: %d; Got: %d", DefaultShutdownTimeout, configs.ShutdownTimeout)
	}
	if configs.ReadTimeout != DefaultReadTimeout {
		t.Errorf("Expected: %d; Got: %d", DefaultReadTimeout, configs.ReadTimeout)
	}
	if configs.WriteTimeout != DefaultWriteTimeout {
		t.Errorf("Expected: %d; Got: %d", DefaultWriteTimeout, configs.WriteTimeout)
	}
	if configs.PingEndpoint != DefaultPingEndpoint {
		t.Errorf("Expected: %s; Got: %s", DefaultPingEndpoint, configs.PingEndpoint)
	}
	if configs.HealthcheckEndpoint != DefaultHealthcheckEndpoint {
		t.Errorf("Expected: %s; Got: %s", DefaultHealthcheckEndpoint, configs.HealthcheckEndpoint)
	}
	if configs.ShutdownEndpoint != DefaultShutdownEndpoint {
		t.Errorf("Expected: %s; Got: %s", DefaultShutdownEndpoint, configs.ShutdownEndpoint)
	}
}

func TestServerWithStartHandlerShouldStartServerSuccessfully(t *testing.T) {
	router := mux.NewRouter()
	configs := getTestConfigs()

	runTestServer(t, configs, router, true,
		func(s Server) {
			s.RegisterServerStartHandler(func(s *http.Server) error {
				return s.ListenAndServe()
			})
		},
		nil)
}

func TestServerWithShutdownHandlerShouldFireHandlersSuccessfully(t *testing.T) {
	shutdownCount := 0
	router := mux.NewRouter()
	configs := getTestConfigs()

	runTestServer(t, configs, router, true, func(s Server) {
		s.RegisterOnShutdown(func() {
			shutdownCount++
		})
		s.RegisterOnShutdown(func() {
			shutdownCount++
		})
	}, nil)

	if shutdownCount != 2 {
		t.Errorf("Expected: 2 shutdown handlers being executed; Got: %d", shutdownCount)
	}
}

func TestRegisterHealthcheckEndpointShouldRegisterAndStartEndpointsSuccessfully(t *testing.T) {
	healthcheckHit := false
	router := mux.NewRouter()
	configs := getTestConfigs()

	runTestServer(t, configs, router, true,
		func(s Server) {
			s.RegisterHealthcheckEndpoint(customHealthcheckEndpoint, func(w http.ResponseWriter, r *http.Request) {
				healthcheckHit = true
				w.WriteHeader(200)
			})
		},
		func(s Server) {
			testEndpoint(t, configs.Port, customHealthcheckEndpoint, 200)
		})

	if !healthcheckHit {
		t.Error("Expected: healthcheck handler to be executed; Got: not executed")
	}
}

func TestNewServerShouldReturnServerWithEndpointsConfigured(t *testing.T) {
	router := mux.NewRouter()
	configs := getTestConfigs()
	server := New(configs, router)

	if server == nil {
		t.Error("Expected: new instance of Server; Got: nil")
	}
	if router.GetRoute(DefaultPingEndpoint) == nil {
		t.Error("Expected: ping endpoint configured; Got: nil")
	}
	if router.GetRoute(DefaultHealthcheckEndpoint) == nil {
		t.Error("Expected: healthcheck endpoint configured; Got: nil")
	}
	if router.GetRoute(DefaultShutdownEndpoint) == nil {
		t.Error("Expected: shutdown endpoint configured; Got: nil")
	}
}

func TestServerShouldStartAllPreConfiguredEndpointsSuccessfully(t *testing.T) {
	router := mux.NewRouter()
	configs := getTestConfigs()

	runTestServer(t, configs, router, false, nil, func(s Server) {
		testEndpoint(t, configs.Port, DefaultPingEndpoint, 200)
		testEndpoint(t, configs.Port, DefaultHealthcheckEndpoint, 200)
		testEndpoint(t, configs.Port, DefaultShutdownEndpoint, 200)
	})
}

func TestServerWithStartErrorShouldReturnOriginalStartError(t *testing.T) {
	configs := &Configs{
		Port: -1,
	}
	router := mux.NewRouter()
	server := New(configs, router)

	err := server.Start()
	if err == nil {
		t.Error("Expected: error as the server port is not valid; Got: success")
	}
	if operr, ok := err.(*net.OpError); ok {
		if _, ok2 := operr.Err.(*net.AddrError); !ok2 {
			t.Errorf("Expected: error as the server port is not valid; Got: %s", reflect.TypeOf(operr.Err))
		}
	} else {
		t.Errorf("Expected: error as the server port is not valid; Got: %s", reflect.TypeOf(err))
	}

	testEndpoint(t, configs.Port, DefaultPingEndpoint, 404)
}

func TestServerWithStopErrorShouldReturnOriginalStopError(t *testing.T) {
	router := mux.NewRouter()
	configs := getTestConfigs()
	testError := errors.New("Simulate shutdown error")

	runTestServer(t, configs, router, false,
		func(s Server) {
			s.RegisterServerShutdownHandler(func(s *http.Server, ctx context.Context) error {
				return testError
			})
		},
		func(s Server) {
			if err := s.Stop(); err != testError {
				if err == nil {
					t.Error("Expected: shutdown test error; Got: nil")
				} else {
					t.Errorf("Expected: shutdown test error; Got: %s", err.Error())
				}
			}
		})
}

func TestStopWithoutCallingStartShouldReturnNil(t *testing.T) {
	router := mux.NewRouter()
	configs := getTestConfigs()
	server := New(configs, router)

	if err := server.Stop(); err != nil {
		t.Errorf("Expected: nil; Got: %s", err.Error())
	}
}

func TestGetHTTPServerShouldReturnInitializedServer(t *testing.T) {
	router := mux.NewRouter()
	configs := getTestConfigs()
	server := New(configs, router)

	if s := server.GetHTTPServer(); s == nil {
		t.Error("Expected: initialized server; Got: nil")
	}
}

func runTestServer(t *testing.T, configs *Configs, router *mux.Router, stopServer bool, serverCreatedHook func(Server), serverRunningHook func(Server)) {
	server := New(configs, router)
	if server == nil {
		t.Error("Expected: new instance of Server; Got: nil")
	}

	if serverCreatedHook != nil {
		serverCreatedHook(server)
	}

	go func() {
		if err := server.Start(); err != nil {
			t.Errorf("Expected: success; Got: %s", err.Error())
		}
	}()

	testEndpoint(t, configs.Port, DefaultPingEndpoint, 200)

	if serverRunningHook != nil {
		serverRunningHook(server)
	}

	if stopServer {
		if err := server.Stop(); err != nil {
			t.Errorf("Expected: success; Got: %s", err.Error())
		}

		testEndpoint(t, configs.Port, DefaultPingEndpoint, 404)
	}
}

// Test the specified endpoint with retries as some server operations are async with no external way to find out when they fully finalized.
func testEndpoint(t *testing.T, port int, path string, expectedStatusCode int) {
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err := http.Get(fmt.Sprintf("%s:%d%s", testServerEndpoint, port, path))
		if err != nil {
			if expectedStatusCode != 404 {
				if attempt == maxRetries {
					t.Fatal(err)
				} else {
					time.Sleep(time.Millisecond)
				}
			} else {
				break
			}
		} else if resp.StatusCode != expectedStatusCode {
			if attempt == maxRetries {
				t.Fatalf("Expected: %d; Got: %d", expectedStatusCode, resp.StatusCode)
			} else {
				time.Sleep(time.Millisecond)
			}
		} else {
			break
		}
	}
}

// Increment and return a new port for each test, avoiding port collisions on parallel tests.
func getTestConfigs() *Configs {
	testServerPort++

	return &Configs{
		Port: testServerPort,
	}
}
