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
	"net/url"
	"testing"

	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/tokenissuer"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/service"

	"github.com/microbus-io/fabric/examples/login/loginapi"
)

var (
	_ *testing.T
	_ testarossa.TestingT
	_ service.Service
	_ *loginapi.Client
)

// Initialize starts up the testing app.
func Initialize() (err error) {
	// Add microservices to the testing app
	err = App.AddAndStartup(
		tokenissuer.NewService(),

		Svc.Init(func(svc *Service) {
			// Initialize the microservice under test
		}),
	)
	if err != nil {
		return err
	}
	return nil
}

// Terminate gets called after the testing app shut down.
func Terminate() (err error) {
	return nil
}

func TestLogin_Login(t *testing.T) {
	t.Parallel()

	ctx := Context()
	Login_Get(t, ctx, "").
		BodyContains("Login").
		BodyContains("Username").
		BodyContains("Password")

	Login_Post(t, ctx, "", "", url.Values{
		"u": {"manager@example.com"},
		"p": {"password"},
		"l": {"Login"},
	}).HeaderContains("Set-Cookie", "Authorization=ey")
}

func TestLogin_Logout(t *testing.T) {
	t.Parallel()

	ctx := Context()

	frame.Of(ctx).SetActor(Actor{
		Subject: "someone@example.com",
		Roles:   []string{"m", "u"},
	})
	Logout_Get(t, ctx, "").HeaderContains("Set-Cookie", "Authorization=; Path=/; Max-Age=0;")
}

func TestLogin_Welcome(t *testing.T) {
	t.Parallel()

	ctx := Context()

	frame.Of(ctx).SetActor(Actor{
		Subject: "someone@example.com",
		Roles:   []string{"m", "u"},
	})
	Welcome_Get(t, ctx, "").BodyContains("YES, you rule")

	frame.Of(ctx).SetActor(Actor{
		Subject: "someone@example.com",
		Roles:   []string{"a"},
	})
	Welcome_Get(t, ctx, "").BodyContains("YES, you're all powerful")
}

func TestLogin_AdminOnly(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	/*
		ctx := Context()
		AdminOnly_Get(t, ctx, "").BodyContains(value)
		AdminOnly_Post(t, ctx, "", "", body).BodyContains(value)
		httpReq, _ := http.NewRequestWithContext(ctx, method, "?arg=val", body)
		AdminOnly(t, httpReq).BodyContains(value)
	*/

	ctx := Context()

	_, err := loginapi.NewClient(Svc).AdminOnly_Get(ctx, "")
	tt.Error(err)

	frame.Of(ctx).SetActor(Actor{
		Subject: "someone@example.com",
		Roles:   []string{"m", "u"},
	})
	_, err = loginapi.NewClient(Svc).AdminOnly_Get(ctx, "")
	tt.Error(err)

	frame.Of(ctx).SetActor(Actor{
		Subject: "someone@example.com",
		Roles:   []string{"a"},
	})
	_, err = loginapi.NewClient(Svc).AdminOnly_Get(ctx, "")
	tt.NoError(err)
}

func TestLogin_ManagerOnly(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := Context()

	_, err := loginapi.NewClient(Svc).ManagerOnly_Get(ctx, "")
	tt.Error(err)

	frame.Of(ctx).SetActor(Actor{
		Subject: "someone@example.com",
		Roles:   []string{"a"},
	})
	_, err = loginapi.NewClient(Svc).ManagerOnly_Get(ctx, "")
	tt.Error(err)

	frame.Of(ctx).SetActor(Actor{
		Subject: "someone@example.com",
		Roles:   []string{"m"},
	})
	_, err = loginapi.NewClient(Svc).ManagerOnly_Get(ctx, "")
	tt.NoError(err)
}
