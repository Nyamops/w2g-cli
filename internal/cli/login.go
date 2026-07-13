package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"
	"golang.org/x/term"
)

func loginCmd() *cli.Command {
	return &cli.Command{
		Name:  "login",
		Usage: "Sign in with your W2G email and password (stores the session)",
		Description: `Authenticate against the W2G API and save the credentials it returns.

The usual way is your account email and password:

    w2g login --email you@example.com --password "secret"
    w2g login                    # prompts for both (password is not echoed)

Advanced: if you already have the cookie a logged-in browser holds, you can
store it directly instead of signing in:

    w2g login --token "eyJfcmFpbHMi...--c832700b..."

Credentials are written to the config file — see
` + "`w2g config path`" + `.`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "email", Usage: "your W2G account email"},
			&cli.StringFlag{Name: "password", Usage: "your W2G account password (prompted if omitted)"},
			&cli.StringFlag{Name: "token", Usage: "store a remember_user_token cookie directly (skips sign-in)"},
			&cli.StringFlag{Name: "session", Usage: "value of the w2g_session_id cookie (with --token)"},
			&cli.StringFlag{Name: "lang", Usage: "interface language cookie w2glang (default: en)"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			app := appFrom(ctx)
			if l := cmd.String("lang"); l != "" {
				app.Cfg.Lang = strings.TrimSpace(l)
			}

			if token := strings.TrimSpace(cmd.String("token")); token != "" {
				app.Cfg.RememberToken = token
				if s := cmd.String("session"); s != "" {
					app.Cfg.SessionID = strings.TrimSpace(s)
				}
				return finishLogin(app, "")
			}

			in := bufio.NewReader(app.in)
			email := strings.TrimSpace(cmd.String("email"))
			if email == "" {
				v, err := prompt(in, app, "email", true)
				if err != nil {
					return err
				}
				email = strings.TrimSpace(v)
			}
			password := cmd.String("password")
			if password == "" {
				v, err := promptPassword(in, app, "password")
				if err != nil {
					return err
				}
				password = v
			}

			res, err := app.Client().SignIn(ctx, email, password)
			if err != nil {
				return err
			}
			app.Cfg.RememberToken = res.RememberToken
			if res.SessionID != "" {
				app.Cfg.SessionID = res.SessionID
			}
			return finishLogin(app, email)
		},
	}
}

func finishLogin(app *App, who string) error {
	if err := app.Cfg.Save(); err != nil {
		return err
	}
	if who != "" {
		fmt.Fprintf(app.Out(), "Signed in as %s. ", who)
	}
	fmt.Fprintf(app.Out(), "Saved credentials to %s\n", app.Cfg.Path())
	fmt.Fprintln(app.Out(), "Next: `w2g rooms` to list rooms, then `w2g join <name|id>`.")
	return nil
}

func prompt(in *bufio.Reader, app *App, label string, required bool) (string, error) {
	fmt.Fprintf(app.Errw(), "%s: ", label)
	line, err := in.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("read %s: %w", label, err)
	}
	line = strings.TrimRight(line, "\r\n")
	if required && strings.TrimSpace(line) == "" {
		return "", fmt.Errorf("%s cannot be empty", label)
	}
	return line, nil
}

func promptPassword(in *bufio.Reader, app *App, label string) (string, error) {
	if f, ok := app.in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		fmt.Fprintf(app.Errw(), "%s: ", label)
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(app.Errw())
		if err != nil {
			return "", fmt.Errorf("read %s: %w", label, err)
		}
		if len(b) == 0 {
			return "", fmt.Errorf("%s cannot be empty", label)
		}
		return string(b), nil
	}
	return prompt(in, app, label, true)
}
