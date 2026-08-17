package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/database"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/gogetsomefoodferris/backend/internal/service"
	"golang.org/x/term"
)

var nonInteractivePasswordReader = bufio.NewReader(os.Stdin)

// The admin CLI deliberately provisions identities outside the HTTP server.
// This keeps the first credential out of environment variables and prevents a
// deployment restart from silently creating or replacing an admin account.
func main() {
	if len(os.Args) < 2 || os.Args[1] != "create-user" {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/admin create-user --email admin@example.com")
		os.Exit(2)
	}
	if err := createUser(os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func createUser(args []string) error {
	flags := flag.NewFlagSet("create-user", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	email := flags.String("email", "", "admin email")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parse create-user options: %w", err)
	}
	normalizedEmail, err := service.NormalizeAdminEmail(*email)
	if err != nil {
		return fmt.Errorf("invalid admin email")
	}

	password, err := readHiddenPassword("Password: ")
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	// Validate before asking for confirmation so an operator immediately sees
	// the password policy instead of receiving a vague hash/provisioning error.
	if err := service.ValidateAdminPassword(password); err != nil {
		return err
	}
	confirmation, err := readHiddenPassword("Confirm password: ")
	if err != nil {
		return fmt.Errorf("read password confirmation: %w", err)
	}
	if password != confirmation {
		return errors.New("passwords do not match")
	}
	passwordHash, err := service.HashAdminPassword(password)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	cfg, err := config.LoadLocal()
	if err != nil {
		return fmt.Errorf("load backend configuration: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	defer pool.Close()

	repo := repository.NewPostgresAdminRepository(pool)
	user, err := repo.CreateAdminUser(ctx, normalizedEmail, passwordHash)
	if err != nil {
		if errors.Is(err, repository.ErrAdminEmailExists) {
			return errors.New("admin email already exists; refusing to overwrite it")
		}
		return fmt.Errorf("create admin user: %w", err)
	}
	fmt.Printf("Admin created: %s (%s)\n", user.Email, user.ID)
	return nil
}

func readHiddenPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		password, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		return string(password), err
	}
	// Non-interactive fallback is useful for an operator piping input in a
	// controlled shell, but still never echoes or logs the secret itself.
	// Reuse one buffered reader for both prompts. Creating a new reader for the
	// confirmation would lose bytes already buffered from a piped stdin stream.
	value, err := nonInteractivePasswordReader.ReadString('\n')
	return strings.TrimRight(value, "\r\n"), err
}
