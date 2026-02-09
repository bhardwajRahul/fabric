# First Microservice Walk-Through

This guide walks through the creation of a microservice that implements a simplified version of a word guessing game. The player needs to reveal a secret 5-letter word by entering a limited number of guesses while observing how closely they match the secret word.

For example, if the secret word is `APPLE` and a guess is `OPERA`, the system will identify the second letter `P` as an exact match (green), the letters `A` and `E` as out-of-place matches (yellow), and the letters `O` and `R` as mismatches (grey).

<img src="first-microservice-1.png" width="213">

Microbus uses a coding agent (e.g. Claude Code) to scaffold microservices from natural-language prompts. In this walk-through we'll show the prompts needed to build the `Wordly` microservice, then manually implement the game logic.

#### Step 1: Create the Microservice

Give the agent this prompt:

> HEY CLAUDE...
>
> Create a new microservice called `wordly` in the `examples` directory. Be quick.

The agent will use a skill to create the directory structure and boilerplate files.

```
wordly
├── resources
│   └── embed.go
├── wordlyapi
│   └── client.go
├── AGENTS.md
├── CLAUDE.md
├── PROMPTS.md
├── intermediate.go
├── manifest.yaml
├── mock.go
├── service.go
└── service_test.go
```

#### Step 2: Add a Web Handler

`Wordly` is a simple game and it requires a single endpoint `/play`. We'll use a web handler because we want to create an HTML interface.

> HEY CLAUDE...
>
> Add a web handler endpoint Play to the Wordly microservice. Play renders the Wordly game page. The method should be GET and the route `/play`. Be quick.

The agent will use the `add-web` skill to wire up the endpoint in `intermediate.go`, create a client stub in `wordlyapi/client.go`, add a skeleton handler in `service.go`, and register it in the OpenAPI document.

#### Step 3: Add a Config Property

We want to control the maximum number of guesses using a config.

> HEY CLAUDE...
>
> Add a config property `MaxGuesses` to the Wordly microservice. MaxGuesses controls the maximum number of guesses the player has to guess the secret word. It's an `int` with a default value of `6` and a validation rule of `int [1,]`. No callback needed. Be quick.

The agent will use the `add-config` skill to define the config, generate a getter and setter, and bind it to the microservice.

#### Step 4: Keeping Track of Games

Open `service.go`. You'll see a type definition for the `Service` struct and 3 empty functions: the standard lifecycle callbacks `OnStartup` and `OnShutdown`, and the endpoint `Play`.

We'd like our microservice to serve many users at the same time so we're going to have to track multiple games in parallel. The structure `Game` will maintain the status of a game.

```go
type Game struct {
    secretWord string
    guesses []string
}
```

A `map[string]*Game` will keep track of all games, indexed by the game ID.

```go
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	games map[string]*Game
}
```

This map needs to be initialized in `OnStartup`.

```go
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	svc.games = map[string]*Game{}
	return nil
}
```

#### Step 5: Implementation

We'll implement the game logic in the `Play` method which handles the `GET /play` request. Add the following imports to `service.go` if they aren't already present:

```go
import (
    "bytes"
    "math/rand/v2"
    "fmt"
    "math/big"
    "regexp"
    "strings"

    "github.com/microbus-io/fabric/errors"
)
```

This endpoint accepts two optional query arguments. The first, `game`, identifies the game. If it is not present, a new game is created. Add the following code to the body of `Play`:

```go
gameID := r.URL.Query().Get("game")
game := svc.games[gameID]
if gameID == "" || game == nil {
    gameID = fmt.Sprintf("%x", rand.IntN(1<<24))
    game = &Game{
        secretWord: "APPLE",
    }
    svc.games[gameID] = game
}
```

The secret word is hard-coded to `APPLE` for the time being. We'll get to it later.

The second query argument is `guess`, which if present submits a guess to the identified game. Add the following code to `Play`.

```go
guess := r.URL.Query().Get("guess")
guess = strings.ToUpper(guess)
re := regexp.MustCompile("[A-Z]{5}")
if guess != "" && !re.MatchString(guess) {
    return errors.New("invalid guess")
}
if guess != "" && len(game.guesses) < svc.MaxGuesses() {
    game.guesses = append(game.guesses, guess)
}
```

Notice the use of `svc.MaxGuesses()` to pull the value of the config property.

Next, we'll render the UI. First, a simple form to submit a guess,

```go
var page bytes.Buffer
page.WriteString(`<html><body>`)

page.WriteString(`
    <form method="GET">
        <input type="text" name="guess" maxlength="5">
        <input type="hidden" name="game" value="` + gameID + `">
        <input type="submit" value="Guess">
    </form><p></p>
`)
```

then all prior guesses with indication of how they match the secret word.

```go
for _, g := range game.guesses {
    secret := []rune(game.secretWord)
    word := []rune(g)
    colors := make([]string, 5)
    for i := range 5 {
        colors[i] = "grey"
    }
    for i := range 5 {
        if word[i] == secret[i] {
            colors[i] = "green"
            word[i] = 0
            secret[i] = 0
        }
    }
    for i := range 5 {
        if word[i] == 0 {
            continue
        }
        for j := range 5 {
            if secret[j] == 0 {
                continue
            }
            if word[i] == secret[j] {
                colors[i] = "#DBA800"
                word[i] = 0
                secret[j] = 0
                break
            }
        }
    }
    word = []rune(g)
    for i := range 5 {
        page.WriteString(`<div style="background:` + colors[i] + `;color:white;`)
        page.WriteString(`display:inline-block;font-size:24pt;width:32pt;text-align:center;">`)
        page.WriteRune(word[i])
        page.WriteString(`</div>`)
    }
    page.WriteString(`<p>`)
}

page.WriteString(`</body></html>`)

w.Header().Set("Content-Type", "text/html")
w.Write(page.Bytes())
return nil
```

#### Step 6: Randomized Secret Word

In the prior step we hard-coded the word `APPLE` as the secret word. We need to randomize it but we can't just generate it from random letters because that will result in unguessable gibberish. Instead, we'll choose a word randomly from a list of real dictionary words.

The `resources` folder allows us to embed resource files with the microservice. Ask the coding agent to create a word list resource.

> HEY CLAUDE...
>
> Create a new file `resources/words.txt` with a space-separated list of 5-letter dictionary words. Remove any trailing spaces or newlines at the end of the file.

With `svc.ReadResFile()`, it's a simple matter of choosing a random word from the resource.

```go
func (svc *Service) randomWord() string {
	binary, _ := svc.ReadResFile("words.txt")
	words := strings.Split(string(binary), " ")
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(words))))
	return strings.ToUpper(words[n.Int64()])
}
```

Lastly, adjust the new game generation to make use of `randomWord` instead of the hard-coded `"APPLE"`.

```go
game = &Game{
    secretWord: svc.randomWord(),
}
```

#### Step 7: Time to Play!

Make sure the microservice has been added to the app in `main/main.go`. The agent should have included it in Step 1, but if not, add it manually:

```go
app.Add(
    // ...
    wordly.NewService(),
)
```

Run the app using `go run` or your IDE.

```shell
cd main
go run main.go
```

Go to http://localhost:8080/wordly.example/play and enjoy!

Want to make the game harder by decreasing the number of guesses? Change the value of the config property `MaxGuesses` by editing `config.yaml` in the root of the project.

```yaml
wordly.example:
  MaxGuesses: 5
```

#### Step 8: Extra Credit

This microservice is far from being polished. Try the following on your own, either by hand or by prompting the coding agent:

- Add a title to the top of the page
- Add instructions that mention how many guesses the player has left
- Add a UI element (link or button) to enable the player to start a new game
- Give a cleaner error message to the user on an invalid guess
- Do not print the guessing form if the player exhausted all of their guesses
- Do not accept guesses after the player identified the secret word
- Accept guesses only if they are themselves valid words
- Print the secret word when the player fails to guess it
- Support 4, 6 and 7 letter words
- The microservice stores state in local memory which will break when a second replica is added to the app and requests are load-balanced. Use the [distributed cache](../structure/dlru.md) to work around that
