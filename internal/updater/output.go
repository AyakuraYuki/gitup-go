// Copyright (c) 2026 Ayakura Yuki
// Released under the terms of the MIT License. See LICENSE for details.

package updater

import "github.com/fatih/color"

const (
	indent1 = "   "
	indent2 = "       "
)

var (
	bold   = color.New(color.Bold).SprintFunc()
	blue   = color.New(color.FgBlue, color.Bold).SprintFunc()
	green  = color.New(color.FgGreen, color.Bold).SprintFunc()
	red    = color.New(color.FgRed, color.Bold).SprintFunc()
	cyan   = color.New(color.FgCyan, color.Bold).SprintFunc()
	yellow = color.New(color.FgYellow, color.Bold).SprintFunc()
)

func errorLabel() string { return red("Error:") }
