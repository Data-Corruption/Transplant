package commands

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/Data-Corruption/Transplant/internal/app"
	"github.com/Data-Corruption/Transplant/internal/platform/database/config"
	"github.com/Data-Corruption/Transplant/internal/types"
	"github.com/Data-Corruption/Transplant/pkg/xlog"

	"github.com/urfave/cli/v3"
)

func configCommand(a *app.App) *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "show or change persistent configuration",
		Commands: []*cli.Command{
			{
				Name:  "show",
				Usage: "show safe user-configurable values",
				Action: func(context.Context, *cli.Command) error {
					cfg, err := config.View(a.DB)
					if err != nil {
						return fmt.Errorf("failed to read config: %w", err)
					}
					writeSafeConfig(os.Stdout, cfg)
					return nil
				},
			},
			{
				Name:  "set",
				Usage: "change persistent configuration",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "log",
						Usage: "set log level (" + xlog.ValidLevels + ")",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {

					updated := false
					cfg, err := config.Update(a.DB, func(cfg *types.Configuration) error {
						if cmd.IsSet("log") {
							cfg.LogLevel = cmd.String("log")
							updated = true
						}
						return nil
					})
					if err != nil {
						return fmt.Errorf("failed to update config: %w", err)
					}

					if !updated {
						fmt.Println("No configuration values were changed. Use --help to see available options.")
						return nil
					}
					fmt.Println("Configuration updated successfully.")
					writeSafeConfig(os.Stdout, cfg)
					return nil
				},
			},
		},
	}
}

func writeSafeConfig(w io.Writer, cfg *types.Configuration) {
	fmt.Fprintf(w, "log: %s\n", cfg.LogLevel)
}
