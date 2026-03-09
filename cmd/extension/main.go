// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/urfave/cli/v3"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	controllercmd "github.com/gardener/gardener-extension-shoot-pack/cmd/extension/controller"
	webhookcmd "github.com/gardener/gardener-extension-shoot-pack/cmd/extension/webhook"
	"github.com/gardener/gardener-extension-shoot-pack/pkg/version"
)

func main() {
	app := &cli.Command{
		Name:                  "gardener-extension-shoot-pack",
		Version:               version.Version,
		EnableShellCompletion: true,
		Usage:                 "operators pack",
		Commands: []*cli.Command{
			controllercmd.New(),
			webhookcmd.New(),
		},
	}

	ctx := ctrl.SetupSignalHandler()
	if err := app.Run(ctx, os.Args); err != nil {
		ctrllog.Log.Error(err, "failed to start extension")
		os.Exit(1)
	}
}
