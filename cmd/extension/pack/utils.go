// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pack

import (
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/urfave/cli/v3"
)

// outputFormat specifies the format in which commands will output data.
type outputFormat string

var (
	// outputFormatFlagName is the name of the flag for specifying command output
	// format.
	outputFormatFlagName = "format"

	// outputFormatTable is the default output format used by the commands. It
	// prints data in tabular format.
	outputFormatTable outputFormat = "table"

	// outputFormatJSON produces command output in JSON format.
	outputFormatJSON outputFormat = "json"
)

// errUnknownOutputFormat is an error, which is returned when a user requested
// an unknown output format.
var errUnknownOutputFormat = errors.New("unknown output format")

// outputFormatFlag is a [cli.Flag] which specifies the output format of various
// commands.
var outputFormatFlag = &cli.StringFlag{
	Name:    outputFormatFlagName,
	Usage:   "output format to use - table or json",
	Value:   string(outputFormatTable),
	Aliases: []string{"f"},
	Validator: func(format string) error {
		supportedFormats := []outputFormat{
			outputFormatTable,
			outputFormatJSON,
		}

		if !slices.Contains(supportedFormats, outputFormat(format)) {
			return fmt.Errorf("%w: %s", errUnknownOutputFormat, format)
		}

		return nil
	},
}

// newTableWriter creates a new [tablewriter.Table] with the given [io.Writer]
// and headers.
func newTableWriter(w io.Writer, headers []string) *tablewriter.Table {
	opts := []tablewriter.Option{
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
	table := tablewriter.NewWriter(w).Options(opts...)

	table.Configure(func(cfg *tablewriter.Config) {
		cfg.Row.Alignment.Global = tw.AlignLeft
		cfg.Row.Formatting.AutoWrap = tw.WrapNone
		cfg.Header.Alignment.Global = tw.AlignLeft
	})

	return table
}
