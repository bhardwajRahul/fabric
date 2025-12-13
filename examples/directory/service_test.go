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

package directory

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/examples/directory/directoryapi"
)

// NOTE: The in-memory emulated database is shared across all test functions when they
// run in parallel. This means tests may see data created by other tests. For production
// testing, use a real SQL database connection via svc.SetSQL(dsn) which provides proper
// isolation. The tests are designed to be tolerant of this shared state.

// must is a helper function to simplify error handling in tests
func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func TestDirectory_Create(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()
	// svc.SetSQL(dsn)

	// Initialize the testers
	tester := connector.New("directory.create.tester")
	client := directoryapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("create_valid_person", func(t *testing.T) {
		assert := testarossa.For(t)

		person := directoryapi.Person{
			FirstName: "John",
			LastName:  "Doe",
			Email:     "john.doe@example.com",
		}

		key, err := client.Create(ctx, person)
		assert.Expect(
			key > 0, true,
			err, nil,
		)
	})

	t.Run("create_person_with_birthday", func(t *testing.T) {
		assert := testarossa.For(t)

		birthday := must(time.Parse("2006-01-02", "1990-05-15"))
		person := directoryapi.Person{
			FirstName: "Jane",
			LastName:  "Smith",
			Email:     "jane.smith@example.com",
			Birthday:  birthday,
		}

		key, err := client.Create(ctx, person)
		assert.Expect(
			key > 0, true,
			err, nil,
		)
	})

	t.Run("create_missing_first_name", func(t *testing.T) {
		assert := testarossa.For(t)

		person := directoryapi.Person{
			LastName: "Doe",
			Email:    "missing.first@example.com",
		}

		_, err := client.Create(ctx, person)
		assert.Error(err)
	})

	t.Run("create_missing_last_name", func(t *testing.T) {
		assert := testarossa.For(t)

		person := directoryapi.Person{
			FirstName: "John",
			Email:     "missing.last@example.com",
		}

		_, err := client.Create(ctx, person)
		assert.Error(err)
	})

	t.Run("create_missing_email", func(t *testing.T) {
		assert := testarossa.For(t)

		person := directoryapi.Person{
			FirstName: "John",
			LastName:  "Doe",
		}

		_, err := client.Create(ctx, person)
		assert.Error(err)
	})

	t.Run("create_duplicate_email", func(t *testing.T) {
		assert := testarossa.For(t)

		person1 := directoryapi.Person{
			FirstName: "Alice",
			LastName:  "Johnson",
			Email:     "alice.j@example.com",
		}
		_, err := client.Create(ctx, person1)
		assert.NoError(err)

		// Try to create another person with the same email
		person2 := directoryapi.Person{
			FirstName: "Alice",
			LastName:  "Jones",
			Email:     "alice.j@example.com",
		}
		_, err = client.Create(ctx, person2)
		assert.Error(err)
	})
}

func TestDirectory_Load(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()
	// svc.SetSQL(dsn)

	// Initialize the testers
	tester := connector.New("directory.load.tester")
	client := directoryapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("load_existing_person", func(t *testing.T) {
		assert := testarossa.For(t)

		// First create a person
		person := directoryapi.Person{
			FirstName: "Bob",
			LastName:  "Builder",
			Email:     "bob.builder@example.com",
		}
		key, err := client.Create(ctx, person)
		assert.NoError(err)

		// Now load it
		loaded, err := client.Load(ctx, key)
		assert.Expect(
			loaded.FirstName, "Bob",
			loaded.LastName, "Builder",
			loaded.Email, "bob.builder@example.com",
			loaded.Key, key,
			err, nil,
		)
	})

	t.Run("load_with_birthday", func(t *testing.T) {
		assert := testarossa.For(t)

		birthday := must(time.Parse("2006-01-02", "1985-12-25"))
		person := directoryapi.Person{
			FirstName: "Carol",
			LastName:  "Christmas",
			Email:     "carol.xmas@example.com",
			Birthday:  birthday,
		}
		key, err := client.Create(ctx, person)
		assert.NoError(err)

		loaded, err := client.Load(ctx, key)
		assert.Expect(
			loaded.FirstName, "Carol",
			loaded.Birthday.Format("2006-01-02"), "1985-12-25",
			err, nil,
		)
	})

	t.Run("load_nonexistent_person", func(t *testing.T) {
		assert := testarossa.For(t)

		_, err := client.Load(ctx, directoryapi.PersonKey(99999))
		assert.Error(err)
	})
}

func TestDirectory_Delete(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()
	// svc.SetSQL(dsn)

	// Initialize the testers
	tester := connector.New("directory.delete.tester")
	client := directoryapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("delete_existing_person", func(t *testing.T) {
		assert := testarossa.For(t)

		// Create a person
		person := directoryapi.Person{
			FirstName: "Delete",
			LastName:  "Me",
			Email:     "delete.me@example.com",
		}
		key, err := client.Create(ctx, person)
		assert.NoError(err)

		// Delete the person
		err = client.Delete(ctx, key)
		assert.NoError(err)

		// Verify it's gone
		_, err = client.Load(ctx, key)
		assert.Error(err)
	})

	t.Run("delete_nonexistent_person", func(t *testing.T) {
		assert := testarossa.For(t)

		err := client.Delete(ctx, directoryapi.PersonKey(99999))
		assert.Error(err)
	})

	t.Run("delete_twice", func(t *testing.T) {
		assert := testarossa.For(t)

		// Create a person
		person := directoryapi.Person{
			FirstName: "Double",
			LastName:  "Delete",
			Email:     "double.delete@example.com",
		}
		key, err := client.Create(ctx, person)
		assert.NoError(err)

		// Delete once
		err = client.Delete(ctx, key)
		assert.NoError(err)

		// Delete again should fail
		err = client.Delete(ctx, key)
		assert.Error(err)
	})
}

func TestDirectory_Update(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()
	// svc.SetSQL(dsn)

	// Initialize the testers
	tester := connector.New("directory.update.tester")
	client := directoryapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("update_existing_person", func(t *testing.T) {
		assert := testarossa.For(t)

		// Create a person
		person := directoryapi.Person{
			FirstName: "Update",
			LastName:  "Me",
			Email:     "update.me@example.com",
		}
		key, err := client.Create(ctx, person)
		assert.NoError(err)

		// Update the person
		updated := directoryapi.Person{
			FirstName: "Updated",
			LastName:  "Person",
			Email:     "updated.person@example.com",
		}
		err = client.Update(ctx, key, updated)
		assert.NoError(err)

		// Verify the update
		loaded, err := client.Load(ctx, key)
		assert.Expect(
			loaded.FirstName, "Updated",
			loaded.LastName, "Person",
			loaded.Email, "updated.person@example.com",
			err, nil,
		)
	})

	t.Run("update_add_birthday", func(t *testing.T) {
		assert := testarossa.For(t)

		// Create a person without birthday
		person := directoryapi.Person{
			FirstName: "No",
			LastName:  "Birthday",
			Email:     "no.birthday@example.com",
		}
		key, err := client.Create(ctx, person)
		assert.NoError(err)

		// Update with birthday
		birthday := must(time.Parse("2006-01-02", "1995-03-10"))
		updated := directoryapi.Person{
			FirstName: "With",
			LastName:  "Birthday",
			Email:     "with.birthday@example.com",
			Birthday:  birthday,
		}
		err = client.Update(ctx, key, updated)
		assert.NoError(err)

		// Verify
		loaded, err := client.Load(ctx, key)
		assert.Expect(
			loaded.Birthday.Format("2006-01-02"), "1995-03-10",
			err, nil,
		)
	})

	t.Run("update_nonexistent_person", func(t *testing.T) {
		assert := testarossa.For(t)

		person := directoryapi.Person{
			FirstName: "Ghost",
			LastName:  "Person",
			Email:     "ghost@example.com",
		}
		err := client.Update(ctx, directoryapi.PersonKey(99999), person)
		assert.Error(err)
	})

	t.Run("update_invalid_data", func(t *testing.T) {
		assert := testarossa.For(t)

		// Create a valid person
		person := directoryapi.Person{
			FirstName: "Valid",
			LastName:  "Person",
			Email:     "valid.person@example.com",
		}
		key, err := client.Create(ctx, person)
		assert.NoError(err)

		// Try to update with invalid data (missing first name)
		invalid := directoryapi.Person{
			LastName: "Invalid",
			Email:    "invalid@example.com",
		}
		err = client.Update(ctx, key, invalid)
		assert.Error(err)
	})
}

func TestDirectory_LoadByEmail(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()
	// svc.SetSQL(dsn)

	// Initialize the testers
	tester := connector.New("directory.loadbyemail.tester")
	client := directoryapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("load_by_email_existing", func(t *testing.T) {
		assert := testarossa.For(t)

		// Create a person
		person := directoryapi.Person{
			FirstName: "Email",
			LastName:  "Search",
			Email:     "email.search@example.com",
		}
		key, err := client.Create(ctx, person)
		assert.NoError(err)

		// Load by email
		loaded, err := client.LoadByEmail(ctx, "email.search@example.com")
		assert.Expect(
			loaded.FirstName, "Email",
			loaded.LastName, "Search",
			loaded.Key, key,
			err, nil,
		)
	})

	t.Run("load_by_email_case_insensitive", func(t *testing.T) {
		assert := testarossa.For(t)

		// Create a person
		person := directoryapi.Person{
			FirstName: "Case",
			LastName:  "Sensitive",
			Email:     "case.sensitive@example.com",
		}
		_, err := client.Create(ctx, person)
		assert.NoError(err)

		// Load with different case
		loaded, err := client.LoadByEmail(ctx, "CASE.SENSITIVE@EXAMPLE.COM")
		assert.Expect(
			loaded.FirstName, "Case",
			loaded.LastName, "Sensitive",
			err, nil,
		)
	})

	t.Run("load_by_email_nonexistent", func(t *testing.T) {
		assert := testarossa.For(t)

		_, err := client.LoadByEmail(ctx, "nonexistent@example.com")
		assert.Error(err)
	})
}

func TestDirectory_List(t *testing.T) {
	// No t.Parallel: test will fail if records added concurrently by another test
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()
	// svc.SetSQL(dsn)

	// Initialize the testers
	tester := connector.New("directory.list.tester")
	client := directoryapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("list_returns_keys", func(t *testing.T) {
		assert := testarossa.For(t)

		// Get initial count
		before, err := client.List(ctx)
		assert.NoError(err)
		countBefore := len(before)

		// Create some persons
		keys := []directoryapi.PersonKey{}
		for i := 1; i <= 3; i++ {
			person := directoryapi.Person{
				FirstName: "Person",
				LastName:  must(fmt.Sprintf("Number%d", i), nil),
				Email:     must(fmt.Sprintf("person%d@list.example.com", i), nil),
			}
			key, err := client.Create(ctx, person)
			assert.NoError(err)
			keys = append(keys, key)
		}

		// List all persons
		list, err := client.List(ctx)
		assert.Expect(err, nil)
		// Should have the original count plus 3 new ones
		assert.Equal(len(list), countBefore+3)

		// Verify all newly created keys are in the list
		for _, key := range keys {
			assert.Contains(list, key)
		}
	})
}

func TestDirectory_WebUI(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()
	// svc.SetSQL(dsn)

	// Initialize the testers
	tester := connector.New("directory.webui.tester")
	client := directoryapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("get_webui_form", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.WebUI(ctx, "GET", "", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				// Check for form elements using structured HTML matching
				assert.HTMLMatch(body, "FORM", "")
				assert.HTMLMatch(body, `SELECT[name="method"]`, "")
				assert.HTMLMatch(body, `INPUT[name="path"]`, "")
				assert.HTMLMatch(body, `TEXTAREA[name="body"]`, "")
				assert.HTMLMatch(body, `INPUT[type="submit"]`, "")
			}
		}
	})

	t.Run("post_webui_list_request", func(t *testing.T) {
		assert := testarossa.For(t)

		// Simulate submitting the form to list persons
		formData := "method=GET&path=/persons"
		res, err := client.WebUI(ctx, "POST", "", "application/x-www-form-urlencoded", strings.NewReader(formData))
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				// Check that the response contains the status code 200 in the body
				assert.HTMLMatch(body, "BODY", "200")
				// Check that the form is still present after submission
				assert.HTMLMatch(body, "FORM", "")
				// Check that the path input element exists (value is in attribute, not inner text)
				assert.HTMLMatch(body, `INPUT[name="path"]`, "")
				// Check that the response is displayed in a PRE element
				assert.HTMLMatch(body, "PRE", "")
			}
		}
	})
}
