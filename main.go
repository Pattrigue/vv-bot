package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/handler"
	"github.com/joho/godotenv"

	"github.com/Pattrigue/VVBot/internal/bugtracker"
	"github.com/Pattrigue/VVBot/internal/commands"
)

const shutdownTimeout = 10 * time.Second

var version = "unknown"

func main() {
	if err := run(); err != nil {
		slog.Error("bot stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	slog.Info("starting bot", "version", version)

	// A .env file is optional: hosts usually set real environment variables.
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("loading .env: %w", err)
	}

	env, err := requireEnv("BOT_TOKEN", "TRELLO_API_KEY", "TRELLO_TOKEN", "TRELLO_IN_PROGRESS_LIST_ID")
	if err != nil {
		return err
	}

	bugs := bugtracker.New(bugtracker.Config{
		APIKey: env["TRELLO_API_KEY"],
		Token:  env["TRELLO_TOKEN"],
		ListID: env["TRELLO_IN_PROGRESS_LIST_ID"],
	})

	client, err := disgo.New(env["BOT_TOKEN"],
		bot.WithDefaultGateway(),
		bot.WithEventManagerConfigOpts(bot.WithAsyncEventsEnabled()),
		bot.WithEventListeners(commands.NewRouter(bugs)),
	)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		client.Close(ctx)
	}()

	if err := handler.SyncCommands(client, commands.Definitions, nil); err != nil {
		return fmt.Errorf("registering commands: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := client.OpenGateway(ctx); err != nil {
		return fmt.Errorf("opening gateway: %w", err)
	}

	<-ctx.Done()

	return nil
}

// requireEnv returns the values of the named environment variables, or an
// error listing the ones that are not set.
func requireEnv(names ...string) (map[string]string, error) {
	values := make(map[string]string, len(names))

	var missing []string

	for _, name := range names {
		values[name] = os.Getenv(name)
		if values[name] == "" {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing environment variables: %s", strings.Join(missing, ", "))
	}

	return values, nil
}
