/*
Copyright (c) 2023-2026 Microbus LLC and various contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package httpingress

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/openapi"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"

	"github.com/microbus-io/fabric/coreservices/httpingress/httpingressapi"
	"github.com/microbus-io/fabric/coreservices/httpingress/resources"
)

var (
	_ context.Context
	_ json.Encoder
	_ http.Request
	_ strconv.NumError
	_ time.Duration
	_ errors.TracedError
	_ cfg.Option
	_ httpx.BodyReader
	_ sub.Option
	_ utils.SyncMap[string, string]
	_ httpingressapi.Client
)

const (
	Hostname = httpingressapi.Hostname
	Version  = 375
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	OnChangedPorts(ctx context.Context) (err error)             // MARKER: Ports
	OnChangedAllowedOrigins(ctx context.Context) (err error)    // MARKER: AllowedOrigins
	OnChangedPortMappings(ctx context.Context) (err error)      // MARKER: PortMappings
	OnChangedReadTimeout(ctx context.Context) (err error)       // MARKER: ReadTimeout
	OnChangedWriteTimeout(ctx context.Context) (err error)      // MARKER: WriteTimeout
	OnChangedReadHeaderTimeout(ctx context.Context) (err error) // MARKER: ReadHeaderTimeout
	OnChangedBlockedPaths(ctx context.Context) (err error)      // MARKER: BlockedPaths
}

// NewService creates a new instance of the microservice.
func NewService() *Service {
	svc := &Service{}
	svc.Intermediate = NewIntermediate(svc)
	return svc
}

// Init enables a single-statement pattern for initializing the microservice.
func (svc *Service) Init(initializer func(svc *Service) (err error)) *Service {
	svc.Connector.Init(func(_ *connector.Connector) (err error) {
		return initializer(svc)
	})
	return svc
}

// Intermediate extends and customizes the generic base connector.
type Intermediate struct {
	*connector.Connector
	ToDo
}

// NewIntermediate creates a new instance of the intermediate.
func NewIntermediate(impl ToDo) *Intermediate {
	svc := &Intermediate{
		Connector: connector.New(Hostname),
		ToDo:      impl,
	}
	svc.SetVersion(Version)
	svc.SetDescription(`The HTTP ingress microservice relays incoming HTTP requests to the NATS bus.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.Subscribe("GET", ":0/openapi.json", svc.doOpenAPI)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here

	// HINT: Add web endpoints here

	// HINT: Add metrics here

	// HINT: Add tickers here

	// Configs
	svc.DefineConfig( // MARKER: TimeBudget
		"TimeBudget",
		cfg.Description(`TimeBudget specifies the timeout for handling a request, after it has been read.
A value of 0 or less indicates no time budget.`),
		cfg.DefaultValue(`20s`),
		cfg.Validation(`dur [0s,]`),
	)
	svc.DefineConfig( // MARKER: Ports
		"Ports",
		cfg.Description(`Ports is a comma-separated list of HTTP ports on which to listen for requests.`),
		cfg.DefaultValue(`8080`),
	)
	svc.DefineConfig( // MARKER: RequestMemoryLimit
		"RequestMemoryLimit",
		cfg.Description(`RequestMemoryLimit is the memory capacity used to hold pending requests, in megabytes.`),
		cfg.DefaultValue(`4096`),
		cfg.Validation(`int [1,]`),
	)
	svc.DefineConfig( // MARKER: AllowedOrigins
		"AllowedOrigins",
		cfg.Description(`AllowedOrigins is a comma-separated list of CORS origins to allow requests from.
The * origin can be used to allow CORS request from all origins.`),
		cfg.DefaultValue(`*`),
	)
	svc.DefineConfig( // MARKER: PortMappings
		"PortMappings",
		cfg.Description(`PortMappings is a comma-separated list of mappings in the form x:y->z where x is the inbound
HTTP port, y is the requested NATS port, and z is the port to serve.
An HTTP request https://ingresshost:x/servicehost:y/path is mapped to internal NATS
request https://servicehost:z/path .
Both x and y can be * to indicate all ports. Setting z to * indicates to serve the requested
port y without change. Specific rules take precedence over * rules.
The default mapping grants access to all internal ports via HTTP port 8080 but restricts
HTTP ports 443 and 80 to only internal port 443.`),
		cfg.DefaultValue(`8080:*->*, 443:*->443, 80:*->443`),
	)
	svc.DefineConfig( // MARKER: ReadTimeout
		"ReadTimeout",
		cfg.Description(`ReadTimeout specifies the timeout for fully reading a request.`),
		cfg.DefaultValue(`5m`),
		cfg.Validation(`dur [1s,]`),
	)
	svc.DefineConfig( // MARKER: WriteTimeout
		"WriteTimeout",
		cfg.Description(`WriteTimeout specifies the timeout for fully writing the response to a request.`),
		cfg.DefaultValue(`5m`),
		cfg.Validation(`dur [1s,]`),
	)
	svc.DefineConfig( // MARKER: ReadHeaderTimeout
		"ReadHeaderTimeout",
		cfg.Description(`ReadHeaderTimeout specifies the timeout for fully reading the header of a request.`),
		cfg.DefaultValue(`20s`),
		cfg.Validation(`dur [1s,]`),
	)
	svc.DefineConfig( // MARKER: BlockedPaths
		"BlockedPaths",
		cfg.Description(`A newline-separated list of paths or extensions to block with a 404.
Paths should not include any arguments and are matched exactly.
Extensions are specified with "*.ext" and are matched against the extension of the path only.`),
		cfg.DefaultValue(`/geoserver
/console/
/.env
/.amazon_aws
/solr/admin/info/system
/remote/login
/Autodiscover/Autodiscover.xml
/autodiscover/autodiscover.json
/api/v2/static/not.found
/api/sonicos/tfa
/_ignition/execute-solution
/admin.html
/auth.html
/auth1.html
/readme.txt
/__Additional
/Portal0000.htm
/docs/cplugError.html/
/CSS/Miniweb.css
/.git/*
/cgi-bin/*
/actuator/gateway/routes
/actuator/health
/Public/home/js/check.js
/mifs/.;/services/LogService
/dns-query
/ecp/Current/exporttool/microsoft.exchange.ediscovery.exporttool.application
/owa/auth/x.js
/static/admin/javascript/hetong.js
/sslvpnLogin.html
/vpn/index.html
/wsman
/geoserver/web
/remote/logincheck
/epa/scripts/win/nsepa_setup.exe
/.well-known/security.txt
/cf_scripts/scripts/ajax/ckeditor/ckeditor.js
/Temporary_Listen_Addresses/
/manager/html
/logon/LogonPoint/*
/catalog-portal/ui/oauth/verify
/error_log/.git/HEAD
/containers/json
/hello.world
*.cfm
*.asp
*.aspx
*.cgi
*.jsa
*.jsp
*.shtml
*.php
*.jhtml
*.mwsl
*.dll
*.esp
*.exe`),
	)

	// HINT: Add inbound event sinks here

	_ = marshalFunction
	return svc
}

// doOpenAPI renders the OpenAPI document of the microservice.
func (svc *Intermediate) doOpenAPI(w http.ResponseWriter, r *http.Request) (err error) {
	oapiSvc := openapi.Service{
		ServiceName: svc.Hostname(),
		Description: svc.Description(),
		Version:     svc.Version(),
		Endpoints:   []*openapi.Endpoint{},
		RemoteURI:   frame.Of(r).XForwardedFullURL(),
	}

	endpoints := []*openapi.Endpoint{
		// HINT: Register web handlers and functional endpoints by adding them here
	}

	// Filter by the port of the request
	rePort := regexp.MustCompile(`:(` + regexp.QuoteMeta(r.URL.Port()) + `|0)(/|$)`)
	reAnyPort := regexp.MustCompile(`:[0-9]+(/|$)`)
	for _, ep := range endpoints {
		if rePort.MatchString(ep.Route) || r.URL.Port() == "443" && !reAnyPort.MatchString(ep.Route) {
			oapiSvc.Endpoints = append(oapiSvc.Endpoints, ep)
		}
	}
	if len(oapiSvc.Endpoints) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if svc.Deployment() == connector.LOCAL {
		encoder.SetIndent("", "  ")
	}
	err = encoder.Encode(&oapiSvc)
	return errors.Trace(err)
}

// doOnObserveMetrics is called when metrics are produced.
func (svc *Intermediate) doOnObserveMetrics(ctx context.Context) (err error) {
	return svc.Parallel(
		// HINT: Call JIT observers to record the metric here
	)
}

// doOnConfigChanged is called when the config of the microservice changes.
func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	if changed("Ports") { // MARKER: Ports
		err = svc.OnChangedPorts(ctx)
		if err != nil {
			return err // No trace
		}
	}
	if changed("AllowedOrigins") { // MARKER: AllowedOrigins
		err = svc.OnChangedAllowedOrigins(ctx)
		if err != nil {
			return err // No trace
		}
	}
	if changed("PortMappings") { // MARKER: PortMappings
		err = svc.OnChangedPortMappings(ctx)
		if err != nil {
			return err // No trace
		}
	}
	if changed("ReadTimeout") { // MARKER: ReadTimeout
		err = svc.OnChangedReadTimeout(ctx)
		if err != nil {
			return err // No trace
		}
	}
	if changed("WriteTimeout") { // MARKER: WriteTimeout
		err = svc.OnChangedWriteTimeout(ctx)
		if err != nil {
			return err // No trace
		}
	}
	if changed("ReadHeaderTimeout") { // MARKER: ReadHeaderTimeout
		err = svc.OnChangedReadHeaderTimeout(ctx)
		if err != nil {
			return err // No trace
		}
	}
	if changed("BlockedPaths") { // MARKER: BlockedPaths
		err = svc.OnChangedBlockedPaths(ctx)
		if err != nil {
			return err // No trace
		}
	}
	return nil
}

/*
TimeBudget specifies the timeout for handling a request, after it has been read.
A value of 0 or less indicates no time budget.
*/
func (svc *Intermediate) TimeBudget() (budget time.Duration) { // MARKER: TimeBudget
	_val := svc.Config("TimeBudget")
	_dur, _ := time.ParseDuration(_val)
	return _dur
}

/*
SetTimeBudget sets the value of the configuration property.

TimeBudget specifies the timeout for handling a request, after it has been read.
A value of 0 or less indicates no time budget.
*/
func (svc *Intermediate) SetTimeBudget(budget time.Duration) (err error) { // MARKER: TimeBudget
	return svc.SetConfig("TimeBudget", budget.String())
}

/*
Ports is a comma-separated list of HTTP ports on which to listen for requests.
*/
func (svc *Intermediate) Ports() (port string) { // MARKER: Ports
	return svc.Config("Ports")
}

/*
SetPorts sets the value of the configuration property.

Ports is a comma-separated list of HTTP ports on which to listen for requests.
*/
func (svc *Intermediate) SetPorts(port string) (err error) { // MARKER: Ports
	return svc.SetConfig("Ports", port)
}

/*
RequestMemoryLimit is the memory capacity used to hold pending requests, in megabytes.
*/
func (svc *Intermediate) RequestMemoryLimit() (megaBytes int) { // MARKER: RequestMemoryLimit
	_val := svc.Config("RequestMemoryLimit")
	_i, _ := strconv.ParseInt(_val, 10, 64)
	return int(_i)
}

/*
SetRequestMemoryLimit sets the value of the configuration property.

RequestMemoryLimit is the memory capacity used to hold pending requests, in megabytes.
*/
func (svc *Intermediate) SetRequestMemoryLimit(megaBytes int) (err error) { // MARKER: RequestMemoryLimit
	return svc.SetConfig("RequestMemoryLimit", strconv.Itoa(megaBytes))
}

/*
AllowedOrigins is a comma-separated list of CORS origins to allow requests from.
The * origin can be used to allow CORS request from all origins.
*/
func (svc *Intermediate) AllowedOrigins() (origins string) { // MARKER: AllowedOrigins
	return svc.Config("AllowedOrigins")
}

/*
SetAllowedOrigins sets the value of the configuration property.

AllowedOrigins is a comma-separated list of CORS origins to allow requests from.
The * origin can be used to allow CORS request from all origins.
*/
func (svc *Intermediate) SetAllowedOrigins(origins string) (err error) { // MARKER: AllowedOrigins
	return svc.SetConfig("AllowedOrigins", origins)
}

/*
PortMappings is a comma-separated list of mappings in the form x:y->z where x is the inbound
HTTP port, y is the requested NATS port, and z is the port to serve.
An HTTP request https://ingresshost:x/servicehost:y/path is mapped to internal NATS
request https://servicehost:z/path .
Both x and y can be * to indicate all ports. Setting z to * indicates to serve the requested
port y without change. Specific rules take precedence over * rules.
The default mapping grants access to all internal ports via HTTP port 8080 but restricts
HTTP ports 443 and 80 to only internal port 443.
*/
func (svc *Intermediate) PortMappings() (mappings string) { // MARKER: PortMappings
	return svc.Config("PortMappings")
}

/*
SetPortMappings sets the value of the configuration property.

PortMappings is a comma-separated list of mappings in the form x:y->z where x is the inbound
HTTP port, y is the requested NATS port, and z is the port to serve.
An HTTP request https://ingresshost:x/servicehost:y/path is mapped to internal NATS
request https://servicehost:z/path .
Both x and y can be * to indicate all ports. Setting z to * indicates to serve the requested
port y without change. Specific rules take precedence over * rules.
The default mapping grants access to all internal ports via HTTP port 8080 but restricts
HTTP ports 443 and 80 to only internal port 443.
*/
func (svc *Intermediate) SetPortMappings(mappings string) (err error) { // MARKER: PortMappings
	return svc.SetConfig("PortMappings", mappings)
}

/*
ReadTimeout specifies the timeout for fully reading a request.
*/
func (svc *Intermediate) ReadTimeout() (timeout time.Duration) { // MARKER: ReadTimeout
	_val := svc.Config("ReadTimeout")
	_dur, _ := time.ParseDuration(_val)
	return _dur
}

/*
SetReadTimeout sets the value of the configuration property.

ReadTimeout specifies the timeout for fully reading a request.
*/
func (svc *Intermediate) SetReadTimeout(timeout time.Duration) (err error) { // MARKER: ReadTimeout
	return svc.SetConfig("ReadTimeout", timeout.String())
}

/*
WriteTimeout specifies the timeout for fully writing the response to a request.
*/
func (svc *Intermediate) WriteTimeout() (timeout time.Duration) { // MARKER: WriteTimeout
	_val := svc.Config("WriteTimeout")
	_dur, _ := time.ParseDuration(_val)
	return _dur
}

/*
SetWriteTimeout sets the value of the configuration property.

WriteTimeout specifies the timeout for fully writing the response to a request.
*/
func (svc *Intermediate) SetWriteTimeout(timeout time.Duration) (err error) { // MARKER: WriteTimeout
	return svc.SetConfig("WriteTimeout", timeout.String())
}

/*
ReadHeaderTimeout specifies the timeout for fully reading the header of a request.
*/
func (svc *Intermediate) ReadHeaderTimeout() (timeout time.Duration) { // MARKER: ReadHeaderTimeout
	_val := svc.Config("ReadHeaderTimeout")
	_dur, _ := time.ParseDuration(_val)
	return _dur
}

/*
SetReadHeaderTimeout sets the value of the configuration property.

ReadHeaderTimeout specifies the timeout for fully reading the header of a request.
*/
func (svc *Intermediate) SetReadHeaderTimeout(timeout time.Duration) (err error) { // MARKER: ReadHeaderTimeout
	return svc.SetConfig("ReadHeaderTimeout", timeout.String())
}

/*
A newline-separated list of paths or extensions to block with a 404.
Paths should not include any arguments and are matched exactly.
Extensions are specified with "*.ext" and are matched against the extension of the path only.
*/
func (svc *Intermediate) BlockedPaths() (blockedPaths string) { // MARKER: BlockedPaths
	return svc.Config("BlockedPaths")
}

/*
SetBlockedPaths sets the value of the configuration property.

A newline-separated list of paths or extensions to block with a 404.
Paths should not include any arguments and are matched exactly.
Extensions are specified with "*.ext" and are matched against the extension of the path only.
*/
func (svc *Intermediate) SetBlockedPaths(blockedPaths string) (err error) { // MARKER: BlockedPaths
	return svc.SetConfig("BlockedPaths", blockedPaths)
}

// marshalFunction handled marshaling for functional endpoints.
func marshalFunction(w http.ResponseWriter, r *http.Request, route string, in any, out any, execute func(in any, out any) error) error {
	err := httpx.ReadInputPayload(r, route, in)
	if err != nil {
		return errors.Trace(err)
	}
	err = execute(in, out)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, out)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}
