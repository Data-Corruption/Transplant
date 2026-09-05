package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Data-Corruption/Transplant/internal/app"
	"github.com/Data-Corruption/Transplant/internal/app/commands"
	"github.com/Data-Corruption/Transplant/internal/build"
	"github.com/Data-Corruption/Transplant/internal/transplant"
	"github.com/Data-Corruption/Transplant/pkg/xlog"

	"github.com/urfave/cli/v3"
)

func main() {
	os.Exit(runMain())
}

func notifyProcessContext(parent context.Context) (context.Context, context.CancelFunc) {
	// On Windows, the Go runtime installs SetConsoleCtrlHandler. It maps delivered
	// CTRL_C_EVENT and CTRL_BREAK_EVENT to os.Interrupt, and maps
	// CTRL_CLOSE_EVENT, CTRL_LOGOFF_EVENT, and CTRL_SHUTDOWN_EVENT to SIGTERM.
	ctx, stopSignals := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	// Pretty sure all of these signals escalate to a hard kill after a timeout.
	go func() {
		<-ctx.Done()
		stopSignals()
	}()
	return ctx, stopSignals
}

func runMain() int {
	ctx, stopSignals := notifyProcessContext(context.Background())
	defer stopSignals()

	application := app.New(build.Info())

	rootCommand := &cli.Command{
		Name:    application.BuildInfo().Name,
		Version: application.BuildInfo().Version,
		Usage:   "plant a Sprout",
		Flags: append(transplant.Flags(), []cli.Flag{
			&cli.StringFlag{
				Name:    "log",
				Aliases: []string{"l"},
				Value:   application.BuildInfo().DefaultLogLevel,
				Usage:   "override log level (" + xlog.ValidLevels + ")",
			},
			&cli.BoolFlag{
				Name:    "migrate",
				Aliases: []string{"m"},
				Hidden:  true,
				Usage:   "apply database migrations (installer use only)",
			},
			&cli.BoolFlag{
				Name:   "build-vars",
				Hidden: true,
				Usage:  "print build variables and exit",
			},
		}...),
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			if cmd.Bool("build-vars") {
				fmt.Println(application.BuildInfo().PrintJSON())
				os.Exit(0)
			}
			if cmd.Args().Len() == 0 && !cmd.Bool("migrate") {
				if err := transplant.CheckPlatform(); err != nil {
					return ctx, err
				}
			}
			ctx, err := application.Init(ctx, cmd)
			if err != nil || cmd.Bool("migrate") {
				return ctx, err
			}
			// Before receives the root command, whose remaining args name the
			// selected subcommand. Explicit updates own their own check.
			if cmd.Args().First() != "update" {
				if err := application.StartUpdateCheckIfDue(ctx); err != nil {
					// Update discovery is optional and must never prevent recovery or
					// an unrelated command from running. Manual checks still return
					// release-source errors directly.
					application.Log.Errorf("start update check: %v", err)
				}
			}
			return ctx, nil
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.Bool("migrate") {
				return nil
			}
			if cmd.Args().Len() != 0 {
				return fmt.Errorf("don't know %q; run transplant --help to see the options", cmd.Args().First())
			}
			wizard := &transplant.Wizard{}
			return wizard.Run(ctx, transplant.FromCommand(cmd))
		},
		Commands: commands.All(application),
	}

	commandErr := rootCommand.Run(ctx, os.Args)
	if commandErr != nil && application.Log != nil {
		application.Log.Errorf("command failed: %v", commandErr)
	}
	closeErr := application.Close()
	if err := errors.Join(commandErr, closeErr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
