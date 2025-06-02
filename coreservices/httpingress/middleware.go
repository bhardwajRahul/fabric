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

package httpingress

import (
	"context"
	"strings"
	"time"

	"github.com/microbus-io/fabric/coreservices/httpingress/middleware"
	"github.com/microbus-io/fabric/coreservices/tokenissuer/tokenissuerapi"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/utils"
)

// Middleware names
const (
	ErrorPrinter    = "ErrorPrinter"
	BlockedPaths    = "BlockedPaths"
	Logger          = "Logger"
	Enter           = "Enter"
	SecureRedirect  = "SecureRedirect"
	CORS            = "CORS"
	XForwarded      = "XForwarded"
	InternalHeaders = "InternalHeaders"
	RootPath        = "RootPath"
	Timeout         = "Timeout"
	Authorization   = "Authorization"
	Ready           = "Ready"
	CacheControl    = "CacheControl"
	Compress        = "Compress"
	DefaultFavIcon  = "DefaultFavIcon"
)

// defaultMiddleware prepares the default middleware of the ingress proxy.
func (svc *Service) defaultMiddleware() *middleware.Chain {
	// Warning: renaming or removing middleware is a breaking change because the names are used as location markers
	m := &middleware.Chain{}
	m.Append(ErrorPrinter, middleware.ErrorPrinter())
	m.Append(BlockedPaths, middleware.BlockedPaths(func(path string) bool {
		if svc.blockedPaths[path] {
			return true
		}
		dot := strings.LastIndex(path, ".")
		if dot >= 0 && svc.blockedPaths["*"+path[dot:]] {
			return true
		}
		return false
	}))
	m.Append(Logger, middleware.Logger(svc))
	m.Append(Enter, middleware.NoOp()) // Marker
	m.Append(SecureRedirect, middleware.SecureRedirect(func() bool {
		return svc.secure443
	}))
	m.Append(CORS, middleware.Cors(func(origin string) bool {
		return svc.allowedOrigins["*"] || svc.allowedOrigins[origin]
	}))
	m.Append(XForwarded, middleware.XForwarded())
	m.Append(InternalHeaders, middleware.InternalHeaders())
	m.Append(RootPath, middleware.RootPath("/root"))
	m.Append(Timeout, middleware.Timeout(func() time.Duration {
		return svc.TimeBudget()
	}))
	m.Append(Authorization, middleware.Authorization(func(ctx context.Context, token string) (actor any, valid bool, err error) {
		if validator, ok := utils.StringClaimFromJWT(token, "validator"); ok {
			actor, valid, err = tokenissuerapi.NewClient(svc).ForHost(validator).ValidateToken(ctx, token)
		}
		return actor, valid, errors.Trace(err)
	}))
	m.Append(Ready, middleware.NoOp()) // Marker
	m.Append(CacheControl, middleware.CacheControl("no-store"))
	m.Append(Compress, middleware.Compress())
	m.Append(DefaultFavIcon, middleware.DefaultFavIcon())

	return m
}
