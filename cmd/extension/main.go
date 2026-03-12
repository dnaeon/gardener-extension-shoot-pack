// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
	ctrl "sigs.k8s.io/controller-runtime"

	controllercmd "github.com/gardener/gardener-extension-shoot-pack/cmd/extension/controller"
	packcmd "github.com/gardener/gardener-extension-shoot-pack/cmd/extension/pack"
	webhookcmd "github.com/gardener/gardener-extension-shoot-pack/cmd/extension/webhook"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/version"
)

func main() {
	app := &cli.Command{
		Name:                  "gardener-extension-shoot-pack",
		Version:               version.Version,
		EnableShellCompletion: true,
		Usage:                 "gardener shoot packs",
		Commands: []*cli.Command{
			controllercmd.New(),
			packcmd.New(),
			webhookcmd.New(),
		},
	}

	ctx := ctrl.SetupSignalHandler()
	if err := app.Run(ctx, os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
