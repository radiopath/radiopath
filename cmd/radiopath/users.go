package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/radiopath/radiopath/internal/store"
	"github.com/radiopath/radiopath/internal/web"
)

const usersUsage = `usage:
  radiopath useradd NAME   create a user or set a new password (asks for it, or reads it from stdin)
  radiopath userdel NAME   delete a user with their sessions and all their data
  radiopath users          list users
Needs DATABASE_URL.`

func userCommand(args []string) int {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		return 2
	}
	ctx := context.Background()
	st, err := store.New(ctx, url)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer st.Close()
	if err := st.Migrate(ctx, slog.New(slog.NewJSONHandler(os.Stderr, nil))); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	switch {
	case args[0] == "useradd" && len(args) == 2:
		name := strings.ToLower(args[1])
		pw, err := readPassword()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if errs := web.ValidateCredentials(name, pw); len(errs) > 0 {
			fmt.Fprintln(os.Stderr, strings.Join(errs, "\n"))
			return 1
		}
		hash, err := web.HashPassword(pw)
		if err == nil {
			err = st.UpsertUser(ctx, name, hash)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("user", name, "saved")
	case args[0] == "userdel" && len(args) == 2:
		name := strings.ToLower(args[1])
		if err := st.DeleteUser(ctx, name); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("user", name, "deleted")
	case args[0] == "users" && len(args) == 1:
		users, err := st.ListUsers(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		for _, u := range users {
			state := ""
			if !u.Verified {
				state = " (unconfirmed)"
			}
			fmt.Printf("%-24s created %s  %s%s\n", u.Name, u.CreatedAt.Format("2006-01-02"), u.Email, state)
		}
	default:
		fmt.Fprintln(os.Stderr, usersUsage)
		return 2
	}
	return 0
}

func readPassword() (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", fmt.Errorf("reading password from stdin: %w", err)
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	fmt.Fprint(os.Stderr, "Password: ")
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, "Repeat: ")
	pw2, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(pw) != string(pw2) {
		return "", fmt.Errorf("passwords do not match")
	}
	return string(pw), nil
}
