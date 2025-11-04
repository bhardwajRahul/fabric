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
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/coreservices/tokenissuer"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/examples/login/loginapi"
)

func TestLogin_Login(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("login.login.tester")
	client := loginapi.NewClient(tester)

	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		tokenissuer.NewService(),
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("login_form_displayed", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Login(ctx, "GET", "", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				// Check basic HTML structure
				assert.HTMLMatch(body, "HTML", "")
				assert.HTMLMatch(body, "HEAD", "")
				assert.HTMLMatch(body, "BODY", "")
				assert.HTMLMatch(body, "TITLE", "Login")

				// Check form structure
				assert.HTMLMatch(body, "FORM", "")
				assert.HTMLMatch(body, `FORM[method="POST"]`, "")

				// Check input elements
				assert.HTMLMatch(body, `INPUT[type="text"][name="u"]`, "")
				assert.HTMLMatch(body, `INPUT[type="password"][name="p"]`, "")
				assert.HTMLMatch(body, `INPUT[type="submit"][name="l"]`, "")
				assert.HTMLMatch(body, `INPUT[type="hidden"][name="src"]`, "")

				// Check labels
				assert.HTMLMatch(body, "BODY", "Username")
				assert.HTMLMatch(body, "BODY", "Password")
			}
		}
	})

	t.Run("successful_login", func(t *testing.T) {
		assert := testarossa.For(t)

		formData := url.Values{
			"u": {"manager@example.com"},
			"p": {"password"},
			"l": {"Login"},
		}
		res, err := client.Login(ctx, "POST", "", "", formData)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusTemporaryRedirect) {
			// Check that a cookie was set with the authorization token
			cookies := res.Header.Values("Set-Cookie")
			found := slices.ContainsFunc(cookies, func(cookie string) bool {
				return strings.HasPrefix(cookie, "Authorization=ey")
			})
			assert.True(found, "Expected Authorization cookie to be set with JWT token")
		}
	})
}

func TestLogin_Logout(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("login.logout.tester")
	client := loginapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		tokenissuer.NewService(),
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("logout_clears_cookie", func(t *testing.T) {
		assert := testarossa.For(t)

		// Set the actor
		actor := Actor{
			Subject: "someone@example.com",
			Roles:   []string{"m", "u"},
		}
		res, err := client.WithOptions(pub.Actor(actor)).Logout(ctx, "GET", "", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusTemporaryRedirect) {
			// Check that the Authorization cookie is cleared
			cookies := res.Header.Values("Set-Cookie")
			found := slices.ContainsFunc(cookies, func(cookie string) bool {
				return strings.Contains(cookie, "Authorization=; Path=/; Max-Age=0;")
			})
			assert.True(found, "Expected Authorization cookie to be cleared")
		}
	})
}

func TestLogin_Welcome(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("login.welcome.tester")
	client := loginapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		tokenissuer.NewService(),
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("manager_and_user_welcome", func(t *testing.T) {
		assert := testarossa.For(t)

		// Set actor with manager and user roles
		actor := Actor{
			Subject: "someone@example.com",
			Roles:   []string{"m", "u"},
		}
		res, err := client.WithOptions(pub.Actor(actor)).Welcome(ctx, "GET", "", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "YES, you rule")
			}
		}
	})

	t.Run("admin_welcome", func(t *testing.T) {
		assert := testarossa.For(t)

		// Set actor with admin role
		actor := Actor{
			Subject: "someone@example.com",
			Roles:   []string{"a"},
		}
		res, err := client.WithOptions(pub.Actor(actor)).Welcome(ctx, "GET", "", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "YES, you're all powerful")
			}
		}
	})
}

func TestLogin_AdminOnly(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("login.adminonly.tester")
	client := loginapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		tokenissuer.NewService(),
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("no_actor_denied", func(t *testing.T) {
		assert := testarossa.For(t)

		// No actor set - should be denied
		_, err := client.AdminOnly(ctx, "")
		assert.Error(err)
	})

	t.Run("manager_and_user_denied", func(t *testing.T) {
		assert := testarossa.For(t)

		// Set actor with manager and user roles - should be denied
		actor := Actor{
			Subject: "someone@example.com",
			Roles:   []string{"m", "u"},
		}
		_, err := client.WithOptions(pub.Actor(actor)).AdminOnly(ctx, "")
		assert.Error(err)
	})

	t.Run("admin_allowed", func(t *testing.T) {
		assert := testarossa.For(t)

		// Set actor with admin role - should be allowed
		actor := Actor{
			Subject: "someone@example.com",
			Roles:   []string{"a"},
		}
		_, err := client.WithOptions(pub.Actor(actor)).AdminOnly(ctx, "")
		assert.NoError(err)
	})
}

func TestLogin_ManagerOnly(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("login.manageronly.tester")
	client := loginapi.NewClient(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		tokenissuer.NewService(),
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("no_actor_denied", func(t *testing.T) {
		assert := testarossa.For(t)

		// No actor set - should be denied
		_, err := client.ManagerOnly(ctx, "")
		assert.Error(err)
	})

	t.Run("admin_denied", func(t *testing.T) {
		assert := testarossa.For(t)

		// Set actor with admin role - should be denied (admin is not a manager)
		actor := Actor{
			Subject: "someone@example.com",
			Roles:   []string{"a"},
		}
		_, err := client.WithOptions(pub.Actor(actor)).ManagerOnly(ctx, "")
		assert.Error(err)
	})

	t.Run("manager_allowed", func(t *testing.T) {
		assert := testarossa.For(t)

		// Set actor with manager role - should be allowed
		actor := Actor{
			Subject: "someone@example.com",
			Roles:   []string{"m"},
		}
		_, err := client.WithOptions(pub.Actor(actor)).ManagerOnly(ctx, "")
		assert.NoError(err)
	})
}
