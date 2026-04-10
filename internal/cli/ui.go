package cli

import "io"

type UI struct {
	Stdout io.Writer
	Stderr io.Writer
}

func NewUI(stdout io.Writer, stderr io.Writer) *UI {
	return &UI{
		Stdout: stdout,
		Stderr: stderr,
	}
}
