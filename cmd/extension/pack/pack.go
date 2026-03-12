// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"text/template"

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
				Flags: []cli.Flag{
					outputFormatFlag,
					skipVerifyFlag,
				},
				Action: runPackList,
			},
			{
				Name:    "files",
				Usage:   "list files of a pack",
				Aliases: []string{"f"},
				Flags: []cli.Flag{
					outputFormatFlag,
					skipVerifyFlag,
					&cli.StringFlag{
						Name:     "name",
						Usage:    "name of the pack",
						Required: true,
						Aliases:  []string{"n"},
					},
					&cli.StringFlag{
						Name:     "version",
						Usage:    "version of the pack",
						Required: true,
						Aliases:  []string{"v"},
					},
				},
				Action: runPackListFiles,
			},
			{
				Name:    "sums",
				Usage:   "print checksums of a pack",
				Aliases: []string{"s"},
				Flags: []cli.Flag{
					skipVerifyFlag,
					&cli.StringFlag{
						Name:     "name",
						Usage:    "name of the pack",
						Required: true,
						Aliases:  []string{"n"},
					},
					&cli.StringFlag{
						Name:     "version",
						Usage:    "version of the pack",
						Required: true,
						Aliases:  []string{"v"},
					},
				},
				Action: runPackSums,
			},
			{
				Name:    "dump",
				Usage:   "dump pack resources",
				Aliases: []string{"d"},
				Flags: []cli.Flag{
					skipVerifyFlag,
					&cli.StringFlag{
						Name:     "name",
						Usage:    "name of the pack",
						Required: true,
						Aliases:  []string{"n"},
					},
					&cli.StringFlag{
						Name:     "version",
						Usage:    "version of the pack",
						Required: true,
						Aliases:  []string{"v"},
					},
					&cli.StringFlag{
						Name:     "path",
						Usage:    "dump pack to the specified path",
						Required: true,
						Aliases:  []string{"p"},
					},
				},
				Action: runPackDump,
			},
			{
				Name:    "init",
				Usage:   "init a new pack spec",
				Aliases: []string{"i"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Usage:    "name of the pack",
						Required: true,
						Aliases:  []string{"n"},
					},
					&cli.StringFlag{
						Name:     "version",
						Usage:    "version of the pack",
						Required: true,
						Aliases:  []string{"v"},
					},
					&cli.StringFlag{
						Name:     "namespace",
						Usage:    "target namespace for pack resources",
						Required: true,
						Aliases:  []string{"ns"},
					},
					&cli.StringFlag{
						Name:     "description",
						Usage:    "pack description",
						Required: true,
						Aliases:  []string{"d"},
					},
					&cli.StringFlag{
						Name:    "maintainer",
						Usage:   "pack maintainer",
						Value:   os.Getenv("USER"),
						Aliases: []string{"m"},
					},
				},
				Action: runPackInit,
			},
		},
	}

	return cmd
}

// runPackList runs the command for listing packs.
func runPackList(ctx context.Context, c *cli.Command) error {
	skipVerify := c.Bool(skipVerifyFlagName)
	collection, err := assets.New(assets.FS, assets.WithSkipVerify(skipVerify))
	if err != nil {
		return err
	}

	format := outputFormat(c.String(outputFormatFlagName))

	switch format {
	case outputFormatTable:
		headers := []string{
			"NAME",
			"VERSION",
			"NAMESPACE",
			"DESCRIPTION",
			"RESOURCES",
		}
		table := newTableWriter(os.Stdout, headers)

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
	case outputFormatJSON:
		data, err := json.MarshalIndent(collection, "", "  ")
		if err != nil {
			return err
		}

		fmt.Printf("%s\n", string(data))

		return nil
	default:
		return fmt.Errorf("%w: %s", errUnknownOutputFormat, format)
	}
}

// runPackListFiles runs the command for listing pack files.
func runPackListFiles(ctx context.Context, c *cli.Command) error {
	skipVerify := c.Bool(skipVerifyFlagName)
	collection, err := assets.New(assets.FS, assets.WithSkipVerify(skipVerify))
	if err != nil {
		return err
	}

	name := c.String("name")
	version := c.String("version")
	pack, err := collection.GetPack(name, version)
	if err != nil {
		return err
	}

	format := outputFormat(c.String(outputFormatFlagName))
	switch format {
	case outputFormatTable:
		headers := []string{
			"PACK",
			"PATH",
			"SHA256",
		}
		table := newTableWriter(os.Stdout, headers)

		for _, resource := range pack.Resources {
			row := []string{
				fmt.Sprintf("%s@%s", pack.Name, pack.Version),
				resource.Path,
				resource.SHA256,
			}
			if err := table.Append(row); err != nil {
				return err
			}
		}

		return table.Render()
	case outputFormatJSON:
		data, err := json.MarshalIndent(pack, "", "  ")
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", string(data))

		return nil
	default:
		return fmt.Errorf("%w: %s", errUnknownOutputFormat, format)
	}
}

// runPackSums runs the command for printing checksums of pack resources.
func runPackSums(ctx context.Context, c *cli.Command) error {
	skipVerify := c.Bool(skipVerifyFlagName)
	collection, err := assets.New(assets.FS, assets.WithSkipVerify(skipVerify))
	if err != nil {
		return err
	}

	name := c.String("name")
	version := c.String("version")
	pack, err := collection.GetPack(name, version)
	if err != nil {
		return err
	}

	data, err := pack.ReadFile(assets.MetaFileSums)
	if err != nil {
		return err
	}

	fmt.Println(string(data))

	return nil
}

// runPackDump dumps the pack resources on the filesystem.
func runPackDump(ctx context.Context, c *cli.Command) error {
	skipVerify := c.Bool(skipVerifyFlagName)
	collection, err := assets.New(assets.FS, assets.WithSkipVerify(skipVerify))
	if err != nil {
		return err
	}

	name := c.String("name")
	version := c.String("version")
	pack, err := collection.GetPack(name, version)
	if err != nil {
		return err
	}

	path := c.String("path")
	packBaseDir := filepath.Join(path, pack.BaseDir)
	if err := os.MkdirAll(packBaseDir, os.FileMode(0755)); err != nil {
		return err
	}

	for _, resource := range pack.Resources {
		data, err := resource.Read()
		if err != nil {
			return err
		}

		filePath := filepath.Join(path, resource.Path)
		if err := os.WriteFile(filePath, data, os.FileMode(0644)); err != nil {
			return err
		}
	}

	return nil
}

// runPackInit creates a new pack spec.
func runPackInit(ctx context.Context, c *cli.Command) error {
	tmplRaw := `# -*- mode: bash-ts-mode; sh-basic-offset 2; -*-

# Maintainer: {{ .Maintainer }}

NAME={{ .Name }}
VERSION={{ .Version }}
NAMESPACE={{ .Namespace }}
DESCRIPTION={{ .Description }}

package() {
  # Use Helm
  ${HELM} repo add ...
  ${HELM} template <repo>/<name> > ${PACK_DIR}/bundle.yaml

  # Or kustomize
  # ${KUSTOMIZE} build ...

  # ... or simply install your resources from the pack source dir
  # install -m0644 ${SRC_DIR}/configmap.yaml ${PACK_DIR}/
  # install -m0644 ${SRC_DIR}/pvc.yaml ${PACK_DIR}/
  # install -m0644 ${SRC_DIR}/role.yaml ${PACK_DIR}/
}

# Uncomment the following function, if your pack provides tests.
# package_test() {
#   echo "Testing pack ..."
# }
`
	data := struct {
		Name        string
		Version     string
		Namespace   string
		Description string
		Maintainer  string
	}{
		Name:        c.String("name"),
		Version:     c.String("version"),
		Description: c.String("description"),
		Namespace:   c.String("namespace"),
		Maintainer:  c.String("maintainer"),
	}

	tmpl, err := template.New("pack").Parse(tmplRaw)
	if err != nil {
		return err
	}

	return tmpl.Execute(os.Stdout, data)
}
