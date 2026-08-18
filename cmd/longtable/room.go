package main

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"longtable/internal/store"
)

// runRoomCommand handles `longtable room <subcommand>` — an
// administrative escape hatch the server host runs directly against
// the SQLite file, for cases the web UI can't cover (e.g. a GM who
// lost their password).
//
// It finds the database the same way the server does — through the
// config file — rather than taking a path of its own. Two ways of
// naming one database is how a Host ends up resetting a password in a
// file the running server has never opened, and being told the room
// doesn't exist.
func runRoomCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: longtable room <list|reset-password> [flags]")
	}

	switch args[0] {
	case "list":
		fset := flag.NewFlagSet("room list", flag.ExitOnError)
		configPath := addConfigFlag(fset)
		fset.Parse(args[1:])
		cfg, err := loadConfig(*configPath)
		if err != nil {
			return err
		}
		return roomList(cfg.Database)

	case "reset-password":
		fset := flag.NewFlagSet("room reset-password", flag.ExitOnError)
		configPath := addConfigFlag(fset)
		fset.Parse(args[1:])
		if fset.NArg() != 1 {
			// The flag must come before the room code: `-config path` is
			// parsed as a flag only if it precedes the positional argument.
			return fmt.Errorf("usage: longtable room reset-password [-config path] <room-code>")
		}
		cfg, err := loadConfig(*configPath)
		if err != nil {
			return err
		}
		return roomResetPassword(cfg.Database, fset.Arg(0))

	default:
		return fmt.Errorf("unknown room subcommand %q (want \"list\" or \"reset-password\")", args[0])
	}
}

func roomList(dbPath string) error {
	s, closeDB, err := openStore(dbPath)
	if err != nil {
		return err
	}
	defer closeDB()

	rooms, err := s.ListRooms()
	if err != nil {
		return fmt.Errorf("list rooms: %w", err)
	}
	if len(rooms) == 0 {
		fmt.Println("no rooms yet")
		return nil
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	// CODE, not SLUG: this column is the thing a Host reads back to a GM
	// over the phone, and "room code" is what it's called everywhere a
	// person can see it. `slug` survives as the column and the route
	// parameter, which nobody is ever asked to say out loud.
	fmt.Fprintln(tw, "CODE\tNAME\tCREATED")
	for _, room := range rooms {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", room.Slug, room.Name, room.CreatedAt)
	}
	return tw.Flush()
}

func roomResetPassword(dbPath, slug string) error {
	s, closeDB, err := openStore(dbPath)
	if err != nil {
		return err
	}
	defer closeDB()

	room, err := s.GetRoomBySlug(slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("no room with code %q", slug)
		}
		return fmt.Errorf("look up room: %w", err)
	}

	newPassword, err := generatePassword()
	if err != nil {
		return fmt.Errorf("generate password: %w", err)
	}

	if err := s.SetGMPassword(room.ID, newPassword); err != nil {
		return fmt.Errorf("set password: %w", err)
	}

	fmt.Printf("New GM password for room %q (%s): %s\n", room.Name, room.Slug, newPassword)
	fmt.Println("Give this to the GM — it won't be shown again.")
	return nil
}

const passwordAlphabet = "abcdefghjkmnpqrstuvwxyzACDEFGHJKLMNPQRSTUVWXYZ23456789" // no 0/O/1/l/I

// generatePassword returns a random 16-character password readable
// enough to type in by hand if needed.
func generatePassword() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i, b := range buf {
		buf[i] = passwordAlphabet[int(b)%len(passwordAlphabet)]
	}
	return string(buf), nil
}
