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

package connector

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/microbus-io/fabric/env"
)

const (
	Gray    = "\033[38;2;128;128;128m" // #808080
	Magenta = "\033[38;2;197;134;192m" // #c586c0
	Yellow  = "\033[38;2;215;186;125m" // #d7ba7d
	Orange  = "\033[38;2;206;145;120m" // #ce9178
	Green   = "\033[38;2;106;153;85m"  // #6A9955
	Cyan    = "\033[38;2;156;220;254m" // #9cdcfe
	Red     = "\033[38;2;244;71;71m"   // #f44747
	White   = "\033[38;2;212;212;212m" // #d4d4d4
	Blue    = "\033[38;2;86;159;214m"  // #569cd6
	Reset   = "\033[0m"
)

/*
LogDebug logs a message at DEBUG level.
DEBUG level messages are ignored in PROD environments or if the MICROBUS_LOG_DEBUG environment variable is not set.
The message should be static and concise. Optional arguments can be added for variable data.
Arguments conform to the standard slog pattern.

Example:

	c.LogDebug(ctx, "Tight loop", "index", i)
*/
func (c *Connector) LogDebug(ctx context.Context, msg string, args ...any) {
	logger := c.logger
	if logger == nil || !c.logDebug {
		return
	}
	span := c.Span(ctx)
	if !span.IsEmpty() {
		traceID := span.TraceID()
		if c.deployment != PROD {
			span.Log("debug", msg, args...)
		}
		args = append(args, "trace", traceID)
	}
	logger.Debug(msg, args...)
	_ = c.AddCounter(ctx, "microbus_log_messages", 1,
		"message", msg,
		"severity", "DEBUG",
	)
}

/*
LogInfo logs a message at INFO level.
The message should be static and concise. Optional arguments can be added for variable data.
Arguments conform to the standard slog pattern.

Example:

	c.LogInfo(ctx, "File uploaded", "gb", sizeGB)
*/
func (c *Connector) LogInfo(ctx context.Context, msg string, args ...any) {
	logger := c.logger
	if logger == nil {
		return
	}
	span := c.Span(ctx)
	if !span.IsEmpty() {
		traceID := span.TraceID()
		if c.deployment != PROD {
			span.Log("info", msg, args...)
		}
		args = append(args, "trace", traceID)
	}
	logger.Info(msg, args...)
	_ = c.AddCounter(ctx, "microbus_log_messages", 1,
		"message", msg,
		"severity", "INFO",
	)
}

/*
LogWarn logs a message at WARN level.
The message should be static and concise. Optional arguments can be added for variable data.
Arguments conform to the standard slog pattern.

Example:

	c.LogWarn(ctx, "Dropping job", "job", jobID)
*/
func (c *Connector) LogWarn(ctx context.Context, msg string, args ...any) {
	logger := c.logger
	if logger == nil {
		return
	}
	span := c.Span(ctx)
	if !span.IsEmpty() {
		traceID := span.TraceID()
		if c.deployment != PROD {
			span.Log("warn", msg, args...)
		}
		args = append(args, "trace", traceID)
	}
	logger.Warn(msg, args...)
	_ = c.AddCounter(ctx, "microbus_log_messages", 1,
		"message", msg,
		"severity", "WARN",
	)

	if c.deployment == LOCAL || c.deployment == TESTING {
		for _, f := range args {
			if err, ok := f.(error); ok {
				color := ""
				reset := ""
				if c.deployment == LOCAL {
					color = Red
					reset = Reset
				}
				sep := strings.Repeat("~", 120)
				fmt.Fprintf(os.Stderr, "%s%s\n%+v\n%s%s\n", color, "\u25bc"+sep+"\u25bc", err, "\u25b2"+sep+"\u25b2", reset)
				break
			}
		}
	}
}

/*
LogError logs a message at ERROR level.
The message should be static and concise. Optional arguments can be added for variable data.
Arguments conform to the standard slog pattern.
When logging an error object, name it "error".

Example:

	c.LogError(ctx, "Opening file", "error", err, "file", fileName)
*/
func (c *Connector) LogError(ctx context.Context, msg string, args ...any) {
	logger := c.logger
	if logger == nil {
		return
	}
	span := c.Span(ctx)
	if !span.IsEmpty() {
		traceID := span.TraceID()
		if c.deployment != PROD {
			span.Log("error", msg, args...)
		}
		args = append(args, "trace", traceID)
	}
	logger.Error(msg, args...)
	_ = c.AddCounter(ctx, "microbus_log_messages", 1,
		"message", msg,
		"severity", "ERROR",
	)

	if c.deployment == LOCAL || c.deployment == TESTING {
		for _, f := range args {
			if err, ok := f.(error); ok {
				color := ""
				reset := ""
				if c.deployment == LOCAL {
					color = Red
					reset = Reset
				}
				sep := strings.Repeat("~", 120)
				fmt.Fprintf(os.Stderr, "%s%s\n%+v\n%s%s\n", color, "\u25bc"+sep+"\u25bc", err, "\u25b2"+sep+"\u25b2", reset)
				break
			}
		}
	}
}

// initLogger initializes a logger to match the deployment environment.
func (c *Connector) initLogger() (err error) {
	if c.logger != nil {
		return nil
	}

	if debug := env.Get("MICROBUS_LOG_DEBUG"); debug != "" {
		c.logDebug = true
	}

	env := c.Deployment()

	var handler slog.Handler
	switch env {
	case LOCAL:
		handler = &localLogHandler{}
	case TESTING:
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			AddSource: false,
			Level:     slog.LevelDebug,
		})
	case LAB:
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			AddSource: false,
			Level:     slog.LevelDebug,
		})
	default:
		// Default PROD config
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			AddSource: false,
			Level:     slog.LevelInfo,
		})
	}

	c.logger = slog.New(handler).With(
		"plane", c.Plane(),
		"service", c.Hostname(),
		"ver", c.Version(),
		"id", c.ID(),
		"deployment", c.Deployment(),
	)
	return nil
}

// localLogHandler applies custom logging in the LOCAL deployment.
type localLogHandler struct {
	attrs   []slog.Attr
	bufPool sync.Pool
}

func (h *localLogHandler) Enabled(ctx context.Context, _ slog.Level) bool {
	return true
}

func (h *localLogHandler) Handle(ctx context.Context, rec slog.Record) error {
	var ptrBuf *[]byte
	pooled := h.bufPool.Get()
	if pooled == nil {
		buf := make([]byte, 0, 384)
		ptrBuf = &buf
	} else {
		ptrBuf = pooled.(*[]byte)
	}
	w := bytes.NewBuffer(*ptrBuf)

	w.WriteString(Gray)

	// Timestamp
	if !rec.Time.IsZero() {
		w.WriteString(rec.Time.Format("15:04:05.000 "))
	}

	// Level
	switch rec.Level {
	case slog.LevelDebug:
		w.WriteString(White)
		w.WriteString("DBUG")
	case slog.LevelInfo:
		w.WriteString(White)
		w.WriteString("INFO")
	case slog.LevelWarn:
		w.WriteString(Yellow)
		w.WriteString("WARN")
	case slog.LevelError:
		w.WriteString(Red)
		w.WriteString("ERR!")
	default:
		w.WriteString(White)
		lvl := rec.Level.String()
		if len(lvl) > 4 {
			lvl = lvl[:4]
		}
		fmt.Fprintf(w, "%-4s", lvl)
	}

	// Service
	service := ""
	for _, a := range h.attrs {
		if a.Key == "service" {
			service = a.Value.String()
			break
		}
	}
	const maxlen = 20
	if len([]rune(service)) > maxlen {
		service = string([]rune(service)[:maxlen-1]) + "\u2026"
	}
	w.WriteString(Magenta)
	fmt.Fprintf(w, " %-*s", maxlen, service)

	// Message
	w.WriteString(Cyan)
	fmt.Fprintf(w, " %-30s ", rec.Message)

	// Attributes
	allZeros := func(s string) bool {
		for _, r := range s {
			if r != '0' {
				return false
			}
		}
		return true
	}
	rec.Attrs(func(a slog.Attr) bool {
		if a.Key == "trace" && allZeros(a.Value.String()) {
			return true
		}
		w.WriteString(" ")
		w.WriteString(Blue)
		w.WriteString(a.Key)
		w.WriteString(Gray)
		w.WriteString("=")
		orig := a.Value.String()
		quoted := strconv.Quote(orig)
		quoted = quoted[1 : len(quoted)-1]
		if quoted != orig {
			w.WriteString(Gray)
			w.WriteString(`"`)
			w.WriteString(White)
			w.WriteString(quoted)
			w.WriteString(Gray)
			w.WriteString(`"`)
		} else {
			w.WriteString(White)
			w.WriteString(orig)
		}
		return true
	})

	w.WriteString(Reset)
	w.WriteString("\n")

	os.Stderr.Write(w.Bytes())
	h.bufPool.Put(ptrBuf)
	return nil
}

func (h *localLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &localLogHandler{
		attrs: append(h.attrs, attrs...),
	}
}

func (h *localLogHandler) WithGroup(name string) slog.Handler {
	return h
}

var _ slog.Handler = &localLogHandler{}
