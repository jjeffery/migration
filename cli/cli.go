// Package cli provides a command line interface for
// database migrations using the popular cobra CLI package.
package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jjeffery/migration"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// NewWorkerFunc is called to creata a migration worker.
type NewWorkerFunc func() (*migration.Worker, error)

// MigrateCommand returns a cobra command that can be integrated
// into a command line program.
//
// Pass context.Background() for the  context, or alternatively
// pass a context that will cancel when the user interrupts by
// pressing Ctrl-C or similar.
func MigrateCommand(ctx context.Context, f NewWorkerFunc) *cobra.Command {
	cmd := &cobra.Command{
		Short: "database migrations",
		Use:   "migrate",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	f2 := func() (*migration.Worker, error) {
		w, err := f()
		if err != nil {
			return nil, err
		}
		if w.LogFunc == nil {
			w.LogFunc = cmd.Println
		}
		return w, nil
	}

	cmd.AddCommand(upCommand(ctx, f2))
	cmd.AddCommand(downCommand(ctx, f2))
	cmd.AddCommand(gotoCommand(ctx, f2))
	cmd.AddCommand(forceCommand(ctx, f2))
	cmd.AddCommand(lockCommand(ctx, f2))
	cmd.AddCommand(unlockCommand(ctx, f2))
	cmd.AddCommand(listCommand(ctx, f2))
	cmd.AddCommand(showCommand(ctx, f2))
	return cmd
}

func upCommand(ctx context.Context, f NewWorkerFunc) *cobra.Command {
	cmd := &cobra.Command{
		Short:   "migrate up",
		Long:    "apply all database migrations",
		Use:     "up",
		PreRunE: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := f()
			if err != nil {
				return err
			}
			return m.Up(ctx)
		},
	}
	return cmd
}

func downCommand(ctx context.Context, f NewWorkerFunc) *cobra.Command {
	cmd := &cobra.Command{
		Short:   "migrate down",
		Long:    "rollback all database migrations",
		Use:     "down",
		PreRunE: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := f()
			if err != nil {
				return err
			}
			return m.Down(ctx)
		},
	}
	return cmd
}

func gotoCommand(ctx context.Context, f NewWorkerFunc) *cobra.Command {
	cmd := &cobra.Command{
		Short:   "migrate to version",
		Long:    "migrate up or down to a specific version",
		Use:     "goto <version>",
		PreRunE: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseVersion(args[0])
			if err != nil {
				return err
			}
			m, err := f()
			if err != nil {
				return err
			}
			return m.Goto(ctx, id)
		},
	}
	return cmd
}

func forceCommand(ctx context.Context, f NewWorkerFunc) *cobra.Command {
	cmd := &cobra.Command{
		Short:   "force version",
		Long:    "force the database schema version after an error",
		Use:     "force <version>",
		PreRunE: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseVersion(args[0])
			if err != nil {
				return err
			}
			m, err := f()
			if err != nil {
				return err
			}
			return m.Force(ctx, id)
		},
	}
	return cmd
}
func lockCommand(ctx context.Context, f NewWorkerFunc) *cobra.Command {
	cmd := &cobra.Command{
		Short:   "lock version",
		Long:    "lock a database schema version: prevent down migrations",
		Use:     "lock <version>",
		PreRunE: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseVersion(args[0])
			if err != nil {
				return err
			}
			m, err := f()
			if err != nil {
				return err
			}
			return m.Lock(ctx, id)
		},
	}
	return cmd
}

func unlockCommand(ctx context.Context, f NewWorkerFunc) *cobra.Command {
	cmd := &cobra.Command{
		Short:   "unlock version",
		Long:    "unlock a database schema version: allow down migrations",
		Use:     "unlock <version>",
		PreRunE: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseVersion(args[0])
			if err != nil {
				return err
			}
			m, err := f()
			if err != nil {
				return err
			}
			return m.Unlock(ctx, id)
		},
	}
	return cmd
}

func showCommand(ctx context.Context, f NewWorkerFunc) *cobra.Command {
	cmd := &cobra.Command{
		Short:   "show version",
		Long:    "show database schema version details",
		Use:     "show <version>",
		PreRunE: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseVersion(args[0])
			if err != nil {
				return err
			}
			m, err := f()
			if err != nil {
				return err
			}
			ver, err := m.Version(ctx, id)
			if err != nil {
				return err
			}

			cmd.Printf("version %d:", id)
			if ver.Failed {
				cmd.Print(" FAILED")
			}
			if ver.Locked {
				cmd.Print(" Locked")
			}
			cmd.Println()
			cmd.Println("Up\n--")
			cmd.Println(strings.TrimSpace(ver.Up))
			cmd.Println("\nDown\n----")
			cmd.Println(strings.TrimSpace(ver.Down))

			return nil
		},
	}
	return cmd
}

func listCommand(ctx context.Context, f NewWorkerFunc) *cobra.Command {
	var flags struct {
		all bool
	}
	cmd := &cobra.Command{
		Short:   "list versions",
		Long:    "list all database versions and their status",
		Use:     "list",
		PreRunE: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := f()
			if err != nil {
				return err
			}
			versions, err := m.Versions(ctx)
			if err != nil {
				return err
			}

			if !flags.all {
				// If not instructed to list all versions, list all
				// unapplied versions, but only list applied versions
				// back to the last locked version.
				var start int
				for i := len(versions) - 1; i >= 0; i-- {
					if versions[i].Locked {
						start = i
						break
					}
				}
				var vcopy []*migration.Version
				for i, ver := range versions {
					if i >= start || ver.AppliedAt == nil {
						vcopy = append(vcopy, ver)
					}
				}
				versions = vcopy
			}

			w := tablewriter.NewWriter(cmd.OutOrStderr())
			w.SetHeader([]string{"id", "applied", "status"})
			for _, ver := range versions {
				var row []string
				row = append(row, fmt.Sprint(ver.ID))
				if ver.AppliedAt == nil {
					row = append(row, "")
				} else {
					row = append(row, (*ver.AppliedAt).Format(time.RFC3339))
				}
				if ver.Failed {
					row = append(row, "failed")
				} else if ver.Locked {
					row = append(row, "locked")
				} else if ver.AppliedAt != nil {
					row = append(row, "ok")
				} else {
					row = append(row, "")
				}
				w.Append(row)
			}
			w.Render()
			return nil
		},
	}
	cmd.Flags().BoolVarP(&flags.all, "all", "a", false, "list all versions")
	return cmd
}

func parseVersion(s string) (migration.VersionID, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid database schema version: %s", s)
	}
	if n < 0 {
		return 0, fmt.Errorf("database schema version cannot be negative: %d", n)
	}
	return migration.VersionID(n), nil
}
