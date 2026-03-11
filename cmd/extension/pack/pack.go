// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"os"
	"strconv"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/urfave/cli/v3"

	"github.com/gardener/gardener-extension-shoot-pack/pkg/assets"
)

// New creates a new [cli.Command] for running the extension controller manager.
func New() *cli.Command {
	cmd := &cli.Command{
		Name:    "pack",
		Aliases: []string{"p"},
		Usage:   "pack operations",
		Commands: []*cli.Command{
			{
				Name:    "list",
				Usage:   "list builtin packs",
				Aliases: []string{"ls"},
				Action:  runPackList,
			},
		},
	}

	return cmd
}

// runPackList runs the command for listing packs.
func runPackList(ctx context.Context, c *cli.Command) error {
	collection, err := assets.New(assets.FS)
	if err != nil {
		return err
	}

	headers := []string{
		"NAME",
		"VERSION",
		"NAMESPACE",
		"DESCRIPTION",
		"RESOURCES",
	}

	tableOpts := []tablewriter.Option{
		tablewriter.WithHeader(headers),
		tablewriter.WithRendition(
			tw.Rendition{
				Borders: tw.Border{
					Top:    tw.Off,
					Bottom: tw.Off,
					Left:   tw.Off,
					Right:  tw.Off,
				},
			},
		),
	}
	table := tablewriter.NewWriter(os.Stdout).Options(tableOpts...)
	table.Configure(func(cfg *tablewriter.Config) {
		cfg.Row.Alignment.Global = tw.AlignLeft
		cfg.Row.Formatting.AutoWrap = tw.WrapNone
		cfg.Header.Alignment.Global = tw.AlignLeft
	})

	for _, pack := range collection.Packs {
		row := []string{
			pack.Name,
			pack.Version,
			pack.Namespace,
			pack.Description,
			strconv.Itoa(len(pack.Resources)),
		}
		if err := table.Append(row); err != nil {
			return err
		}
	}

	return table.Render()
}
