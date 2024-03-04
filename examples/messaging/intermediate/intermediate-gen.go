/*
Copyright (c) 2023 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
*/

// Code generated by Microbus. DO NOT EDIT.

/*
Package intermediate serves as the foundation of the messaging.example microservice.

The Messaging microservice demonstrates service-to-service communication patterns.
*/
package intermediate

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/microbus-io/fabric/cb"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/log"
	"github.com/microbus-io/fabric/shardedsql"
	"github.com/microbus-io/fabric/sub"

	"github.com/microbus-io/fabric/examples/messaging/resources"
	"github.com/microbus-io/fabric/examples/messaging/messagingapi"
)

var (
	_ context.Context
	_ *embed.FS
	_ *json.Decoder
	_ fmt.Stringer
	_ *http.Request
	_ filepath.WalkFunc
	_ strconv.NumError
	_ strings.Reader
	_ time.Duration
	_ cb.Option
	_ cfg.Option
	_ *errors.TracedError
	_ *httpx.ResponseRecorder
	_ *log.Field
	_ *shardedsql.DB
	_ sub.Option
	_ messagingapi.Client
)

// ToDo defines the interface that the microservice must implement.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Home(w http.ResponseWriter, r *http.Request) (err error)
	NoQueue(w http.ResponseWriter, r *http.Request) (err error)
	DefaultQueue(w http.ResponseWriter, r *http.Request) (err error)
	CacheLoad(w http.ResponseWriter, r *http.Request) (err error)
	CacheStore(w http.ResponseWriter, r *http.Request) (err error)
}

// Intermediate extends and customizes the generic base connector.
// Code generated microservices then extend the intermediate.
type Intermediate struct {
	*connector.Connector
	impl ToDo
}

// NewService creates a new intermediate service.
func NewService(impl ToDo, version int) *Intermediate {
	svc := &Intermediate{
		Connector: connector.New("messaging.example"),
		impl: impl,
	}
	svc.SetVersion(version)
	svc.SetDescription(`The Messaging microservice demonstrates service-to-service communication patterns.`)

	// Lifecycle
	svc.SetOnStartup(svc.impl.OnStartup)
	svc.SetOnShutdown(svc.impl.OnShutdown)
	
	// Webs
	svc.Subscribe(`:443/home`, svc.impl.Home)
	svc.Subscribe(`:443/no-queue`, svc.impl.NoQueue, sub.NoQueue())
	svc.Subscribe(`:443/default-queue`, svc.impl.DefaultQueue)
	svc.Subscribe(`:443/cache-load`, svc.impl.CacheLoad)
	svc.Subscribe(`:443/cache-store`, svc.impl.CacheStore)

	// Resources file system
	svc.SetResFS(resources.FS)

	return svc
}

// doOnConfigChanged is called when the config of the microservice changes.
func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	return nil
}
