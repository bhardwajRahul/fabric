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

package login

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/microbus-io/fabric/coreservices/tokenissuer/tokenissuerapi"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/frame"

	"github.com/microbus-io/fabric/examples/login/intermediate"
	"github.com/microbus-io/fabric/examples/login/loginapi"
)

var (
	_ context.Context
	_ *http.Request
	_ time.Duration
	_ *errors.TracedError
	_ *loginapi.Client
)

const (
	authTokenCookieName = "Authorization"
)

/*
Service implements the login.example microservice.

The Login microservice demonstrates usage of authentication and authorization.
*/
type Service struct {
	*intermediate.Intermediate // DO NOT REMOVE
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

/*
Login renders a simple login screen that authenticates a user.
Known users are hardcoded as "admin", "manager" and "user".
The password is "password".
*/
func (svc *Service) Login(w http.ResponseWriter, r *http.Request) (err error) {
	ctx := r.Context()

	// Read submitted values
	err = r.ParseForm()
	if err != nil {
		return errors.Trace(err)
	}
	u := r.FormValue("u")
	p := r.FormValue("p")
	src := r.FormValue("src")
	submitted := r.FormValue("l") != ""

	// Validate credentials (hardcoded)
	userRoles := map[string][]string{
		"admin@example.com":   {"a"},      // An admin is not a standard user
		"manager@example.com": {"m", "u"}, // A manager is a standard user too
		"user@example.com":    {"u"},      // An unprivileged standard user
	}
	ok := submitted && userRoles[u] != nil && p == "password"
	if ok {
		// Use the core issuer to create a JWT
		signedJWT, err := tokenissuerapi.NewClient(svc).IssueToken(ctx, Actor{
			Subject: u,
			Roles:   userRoles[u],
		})
		if err != nil {
			return errors.Trace(err)
		}
		token, _ := jwt.Parse(signedJWT, nil)
		exp := time.Unix(int64(token.Claims.(jwt.MapClaims)["exp"].(float64)), 0)
		// Set it as a cookie
		cookie := &http.Cookie{
			Name:     authTokenCookieName,
			Value:    signedJWT,
			MaxAge:   int(time.Until(exp).Round(time.Second).Seconds()),
			HttpOnly: true,
			Secure:   r.TLS != nil,
			Path:     "/",
		}
		http.SetCookie(w, cookie)
		// Redirect
		if src != "" && !strings.Contains(src, "://") {
			// Redirect to the page where they user was denied
			http.Redirect(w, r, src, http.StatusTemporaryRedirect)
		} else {
			// Redirect to Welcome page by default
			http.Redirect(w, r, "/"+Hostname+"/welcome", http.StatusTemporaryRedirect)
		}
		return nil
	}

	// Render the form
	data := struct {
		U      string
		P      string
		Src    string
		Denied bool
	}{
		U:      u,
		P:      p,
		Src:    src,
		Denied: submitted && !ok,
	}
	rendered, err := svc.ExecuteResTemplate("login.html", data)
	if err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-Language", "en-US")
	w.Write(rendered)
	return nil
}

/*
Logout renders a page that logs out the user.
*/
func (svc *Service) Logout(w http.ResponseWriter, r *http.Request) (err error) {
	// Clear the cookie
	cookie := &http.Cookie{
		Name:     authTokenCookieName,
		Value:    "",
		MaxAge:   -1, // Expire
		HttpOnly: true,
		Secure:   r.TLS != nil,
		Path:     "/",
	}
	w.Header().Add("Set-Cookie", cookie.String())

	// Redirect to Login page
	http.Redirect(w, r, "/"+Hostname+"/login", http.StatusTemporaryRedirect)
	return nil
}

/*
Welcome renders a page that is shown to the user after a successful login.
Rendering is adjusted based on the user's roles.
*/
func (svc *Service) Welcome(w http.ResponseWriter, r *http.Request) (err error) {
	var actor Actor
	_, err = frame.Of(r).ParseActor(&actor)
	if err != nil {
		return errors.Trace(err)
	}
	data := struct {
		Actor Actor
		Raw   string
	}{
		Actor: actor,
		Raw:   r.Header.Get(frame.HeaderActor),
	}
	rendered, err := svc.ExecuteResTemplate("welcome.html", data)
	if err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-Language", "en-US")
	w.Write(rendered)
	return nil
}

/*
AdminOnly is only accessible by admins.
*/
func (svc *Service) AdminOnly(w http.ResponseWriter, r *http.Request) (err error) {
	rendered, err := svc.ExecuteResTemplate("admin-only.html", nil)
	if err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-Language", "en-US")
	w.Write(rendered)
	return nil
}

/*
ManagerOnly is only accessible by managers.
*/
func (svc *Service) ManagerOnly(w http.ResponseWriter, r *http.Request) (err error) {
	rendered, err := svc.ExecuteResTemplate("manager-only.html", nil)
	if err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-Language", "en-US")
	w.Write(rendered)
	return nil
}
