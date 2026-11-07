// Command admin runs one-off operations: platform migrations and creating
// the first platform admin (there is no public sign-up).
//
//	go run ./src/cmd/admin migrate
//	go run ./src/cmd/admin migrate-status
//	go run ./src/cmd/admin create-platform-admin -email you@example.com -name "Your Name"
//
// The password is read from PMS_ADMIN_PASSWORD, or prompted for without echo.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
	domain_auth "github.com/utkarshk014/procurement-system-be/src/internal/domain/auth"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository"
	"github.com/utkarshk014/procurement-system-be/src/internal/usecase"
	"github.com/utkarshk014/procurement-system-be/src/packages/database"
	"github.com/utkarshk014/procurement-system-be/src/packages/logger"
	"github.com/utkarshk014/procurement-system-be/src/utils"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: admin <command>

commands:
  migrate                 apply pending platform migrations (pms.sys_* tables)
  migrate-status          list migrations and whether they are applied
  create-platform-admin   -email <email> -name <full name>`)
}

func run(cmd string, args []string) error {
	cfg, err := utils.NewConfig()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	dbm, err := database.NewManager(ctx, cfg.ControlDatabaseURL, cfg.TenantDatabases)
	if err != nil {
		return err
	}
	defer dbm.Close()

	switch cmd {
	case "migrate":
		applied, err := database.MigrateUp(ctx, dbm.Control())
		if err != nil {
			return err
		}
		if len(applied) == 0 {
			fmt.Println("nothing to apply, platform tables are up to date")
		}
		for _, v := range applied {
			fmt.Printf("applied migration %05d\n", v)
		}
		return nil

	case "migrate-status":
		statuses, err := database.Status(ctx, dbm.Control())
		if err != nil {
			return err
		}
		for _, s := range statuses {
			state := "pending"
			if s.Applied {
				state = "applied"
			}
			fmt.Printf("%05d  %-8s  %s\n", s.Version, state, s.Name)
		}
		return nil

	case "create-platform-admin":
		fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
		email := fs.String("email", "", "admin email")
		name := fs.String("name", "", "admin full name")
		if err := fs.Parse(args); err != nil {
			return err
		}
		if *email == "" || *name == "" {
			return errors.New("-email and -name are required")
		}
		password, err := readPassword()
		if err != nil {
			return err
		}
		repos := repository.NewRepositories(dbm)
		usecases := usecase.NewUsecases(cfg, repos, dbm, logger.New(cfg.AppEnv, cfg.LogLevel))
		user, err := usecases.Auth.CreateUser(ctx, domain_auth.CreateUserInput{
			Email:    *email,
			FullName: *name,
			Password: password,
			Role:     domain.RolePlatformAdmin,
		})
		if err != nil {
			if ae, ok := domain.AsAppError(err); ok && len(ae.Fields) > 0 {
				for _, f := range ae.Fields {
					fmt.Fprintf(os.Stderr, "  %s %s\n", f.Field, f.Message)
				}
			}
			return err
		}
		fmt.Printf("created platform admin %s (id %d)\n", user.Email, user.ID)
		return nil

	default:
		usage()
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func readPassword() (string, error) {
	if p := os.Getenv("PMS_ADMIN_PASSWORD"); p != "" {
		return p, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", errors.New("no password given: set PMS_ADMIN_PASSWORD or pipe it on stdin")
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	fmt.Fprint(os.Stderr, "password (min 10 chars): ")
	first, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, "repeat password: ")
	second, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(first) != string(second) {
		return "", errors.New("passwords do not match")
	}
	return string(first), nil
}
