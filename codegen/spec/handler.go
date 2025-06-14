/*
Copyright (c) 2023-2025 Microbus LLC and various contributors

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

package spec

import (
	"regexp"
	"strings"
	"time"

	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/utils"
)

var methodValidator = regexp.MustCompile(`^[A-Z]+$`)

// Handler is the spec of a callback handler.
// Web requests, lifecycle events, config changes, tickers, etc.
type Handler struct {
	Type   string `yaml:"-"`
	Exists bool   `yaml:"-"`

	// Shared
	Signature   *Signature `yaml:"signature"`
	Description string     `yaml:"description"`
	Method      string     `yaml:"method"`
	Path        string     `yaml:"path"`
	Queue       string     `yaml:"queue"`
	OpenAPI     bool       `yaml:"openApi"`
	Actor       string     `yaml:"actor"`
	Callback    bool       `yaml:"callback"`

	// Sink
	Event   string `yaml:"event"`
	Source  string `yaml:"source"`
	ForHost string `yaml:"forHost"`

	// Config
	Default    string `yaml:"default"`
	Validation string `yaml:"validation"`
	Secret     bool   `yaml:"secret"`

	// Metrics
	Kind    string    `json:"kind" yaml:"kind"`
	Alias   string    `json:"alias" yaml:"alias"`
	Buckets []float64 `json:"buckets" yaml:"buckets"`

	// Ticker
	Interval time.Duration `yaml:"interval"`
}

// UnmarshalYAML parses the handler.
func (h *Handler) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Unmarshal
	type different Handler
	var x different
	x.OpenAPI = true // Default
	err := unmarshal(&x)
	if err != nil {
		return errors.Trace(err)
	}
	*h = Handler(x)

	// Post processing
	if h.Path == "" {
		h.Path = "/" + utils.ToKebabCase(h.Name())
	}
	h.Path = strings.Replace(h.Path, "...", utils.ToKebabCase(h.Name()), 1)
	h.Queue = strings.ToLower(h.Queue)
	if h.Queue == "" {
		h.Queue = "default"
	}
	h.Method = strings.ToUpper(h.Method)
	h.Kind = strings.ToLower(h.Kind)
	if h.Kind == "" {
		h.Kind = "counter"
	}
	if h.Event == "" {
		h.Event = h.Name()
	}

	// Validate
	err = h.validate()
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// validate validates the data after unmarshaling.
func (h *Handler) validate() error {
	if h.Queue != "default" && h.Queue != "none" {
		return errors.New("invalid queue '%s' in '%s'", h.Queue, h.Name())
	}
	if h.Kind != "counter" && h.Kind != "gauge" && h.Kind != "histogram" {
		return errors.New("invalid metric kind '%s' in '%s'", h.Kind, h.Name())
	}
	if h.Method != "" && !methodValidator.MatchString(h.Method) {
		return errors.New("invalid method '%s'", h.Method)
	}
	if strings.Contains(h.Path, "`") {
		return errors.New("backtick not allowed in path '%s' in '%s'", h.Path, h.Name())
	}
	if strings.Contains(h.Actor, "`") {
		return errors.New("backtick not allowed in actor '%s' in '%s'", h.Actor, h.Name())
	}
	u, err := httpx.ParseURL(httpx.JoinHostAndPath("hostname", h.Path))
	if err != nil {
		return errors.Trace(err)
	}
	parts := strings.Split(u.Path, "/")
	for i := range parts {
		open := strings.Index(parts[i], "{")
		if open > 0 {
			return errors.New("path argument '%s' in '%s' must span entire section", parts[i], h.Name())
		}
		close := strings.LastIndex(parts[i], "}")
		if open == -1 && close == -1 {
			continue
		}
		if close <= open || open == -1 {
			return errors.New("malformed path argument '%s' in '%s'", parts[i], h.Name())
		}
		if close < len(parts[i])-1 {
			return errors.New("path argument '%s' in '%s' must span entire section", parts[i], h.Name())
		}
		name := parts[i]
		name = strings.TrimPrefix(name, "{")
		name = strings.TrimSuffix(name, "}")
		if strings.HasSuffix(name, "+") && i != len(parts)-1 {
			return errors.New("greedy path argument '%s' in '%s' must end path", parts[i], h.Name())
		}
		name = strings.TrimSuffix(name, "+")
		if name != "" && !utils.IsLowerCaseIdentifier(name) {
			return errors.New("name of path argument '%s' in '%s' must be an identifier", parts[i], h.Name())
		}
	}
	if h.Validation != "" {
		_, err := cfg.NewConfig(
			h.Name(),
			cfg.Validation(h.Validation),
			cfg.DefaultValue(h.Default),
		)
		if err != nil {
			return errors.Trace(err)
		}
	}

	// Type will be empty during initial parsing.
	// It will get filled by the parent service which will then call this method again.
	if h.Type == "" {
		return nil
	}

	if h.Type == "event" {
		if h.Method == "" {
			h.Method = "POST"
		}
		if h.Method == "ANY" {
			return errors.New("invalid method '%s'", h.Method)
		}
		if u.Port() == "0" {
			return errors.New("invalid port '%s'", u.Port())
		}
	} else {
		if h.Method == "" {
			h.Method = "ANY"
		}
	}

	if strings.HasPrefix(h.Path, "/") && !strings.HasPrefix(h.Path, "//") {
		if h.Type == "event" {
			h.Path = ":417" + h.Path
		} else {
			h.Path = ":443" + h.Path
		}
	}

	h.Description = conformDesc(
		h.Description,
		h.Name()+" is a "+h.Type+".",
	)

	// Validate types and number of arguments
	switch h.Type {
	case "web":
		if len(h.Signature.InputArgs) != 0 || len(h.Signature.OutputArgs) != 0 {
			return errors.New("arguments or return values not allowed in '%s'", h.Signature.OrigString)
		}
	case "config":
		if len(h.Signature.InputArgs) != 0 {
			return errors.New("arguments not allowed in '%s'", h.Signature.OrigString)
		}
		if len(h.Signature.OutputArgs) != 1 {
			return errors.New("single return value expected in '%s'", h.Signature.OrigString)
		}
		t := h.Signature.OutputArgs[0].Type
		if t != "string" && t != "int" && t != "bool" && t != "time.Duration" && t != "float64" {
			return errors.New("invalid return type '%s' in '%s'", t, h.Signature.OrigString)
		}
	case "ticker":
		if len(h.Signature.InputArgs) != 0 || len(h.Signature.OutputArgs) != 0 {
			return errors.New("arguments or return values not allowed in '%s'", h.Signature.OrigString)
		}
		if h.Interval <= 0 {
			return errors.New("non-positive interval '%v' in '%s'", h.Interval, h.Name())
		}
	case "function", "event", "sink":
		if h.Actor != "" {
			_, err = utils.EvaluateBoolExp(h.Actor, nil)
			if err != nil {
				return errors.New("invalid boolean expression '%s' in actor", h.Actor)
			}
		}
		for _, arg := range h.Signature.InputArgs {
			if !h.MethodWithBody() && arg.Name == "httpRequestBody" {
				return errors.New("cannot use '%s' in '%s' because method '%s' has no body", arg.Name, h.Signature.OrigString, h.Method)
			}
		}
	case "metric":
		if len(h.Signature.OutputArgs) != 0 {
			return errors.New("return values not allowed in '%s'", h.Signature.OrigString)
		}
		if len(h.Signature.InputArgs) == 0 {
			return errors.New("at least one argument expected in '%s'", h.Signature.OrigString)
		}
		if h.Kind == "histogram" && len(h.Buckets) < 1 {
			return errors.New("at least one bucket is required in '%s'", h.Signature.OrigString)
		}
		t := h.Signature.InputArgs[0].Type
		if t != "int" && t != "time.Duration" && t != "float64" {
			return errors.New("first argument is of a non-numeric type '%s' in '%s'", t, h.Signature.OrigString)
		}
	}

	startsWithOn := regexp.MustCompile(`^On[A-Z][a-zA-Z0-9]*$`)
	if h.Type == "sink" {
		match, _ := regexp.MatchString(`^[a-z][a-zA-Z0-9\.\-]*(/[a-z][a-zA-Z0-9\.\-]*)*$`, h.Source)
		if !match {
			return errors.New("invalid source '%s' in '%s'", h.Source, h.Name())
		}
		if h.ForHost != "" {
			err := utils.ValidateHostname(h.ForHost)
			if err != nil {
				return errors.New("invalid hostname '%s' in '%s'", h.ForHost, h.Name())
			}
		}
		if !startsWithOn.MatchString(h.Event) {
			return errors.New("event name '%s' must start with 'On' in '%s'", h.Event, h.Name())
		}
	}
	if h.Type == "sink" || h.Type == "event" {
		if !startsWithOn.MatchString(h.Name()) {
			return errors.New("function name must start with 'On' in '%s'", h.Signature.OrigString)
		}
	}

	return nil
}

// Name of the handler function.
func (h *Handler) Name() string {
	return h.Signature.Name
}

/*
In returns the input argument list as a string.
Spec options:

	, name type
	, name
	name type,
	name,
	name
	name type
*/
func (h *Handler) In(spec string) string {
	var b strings.Builder
	commaBefore := strings.HasPrefix(spec, ",")
	commaAfter := strings.HasSuffix(spec, ",")
	showType := strings.Contains(spec, "type")
	if commaBefore && len(h.Signature.InputArgs) > 0 {
		b.WriteString(", ")
	}
	for i, arg := range h.Signature.InputArgs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(arg.Name)
		if showType {
			b.WriteString(" ")
			b.WriteString(arg.Type)
		}
	}
	if commaAfter && len(h.Signature.InputArgs) > 0 {
		b.WriteString(", ")
	}
	return b.String()
}

/*
Out returns the output argument list as a string.
Spec options:

	, name type
	, name
	name type,
	name,
	name
	name type
*/
func (h *Handler) Out(spec string) string {
	var b strings.Builder
	commaBefore := strings.HasPrefix(spec, ",")
	commaAfter := strings.HasSuffix(spec, ",")
	showType := strings.Contains(spec, "type")
	if commaBefore && len(h.Signature.OutputArgs) > 0 {
		b.WriteString(", ")
	}
	for i, arg := range h.Signature.OutputArgs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(arg.Name)
		if showType {
			b.WriteString(" ")
			b.WriteString(arg.Type)
		}
	}
	if commaAfter && len(h.Signature.OutputArgs) > 0 {
		b.WriteString(", ")
	}
	return b.String()
}

// SourceSuffix returns the last piece of the event source package path,
// which is expected to point to a microservice.
func (h *Handler) SourceSuffix() string {
	p := strings.LastIndex(h.Source, "/")
	if p < 0 {
		return h.Source
	}
	return h.Source[p+1:]
}

// Port returns the port number set in the path.
func (h *Handler) Port() (string, error) {
	joined := httpx.JoinHostAndPath("hostname", h.Path)
	u, err := httpx.ParseURL(joined)
	if err != nil {
		return "", err
	}
	return u.Port(), nil
}

// MethodWithBody indicates if the HTTP method of the endpoint allows sending a body.
// "GET", "DELETE", "TRACE", "OPTIONS", "HEAD" do not allow a body.
func (h *Handler) MethodWithBody() bool {
	switch strings.ToUpper(h.Method) {
	case "GET", "DELETE", "TRACE", "OPTIONS", "HEAD":
		return false
	default:
		return true
	}
}
