package clterm

import (
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
)

// CLTerm is the Command Line terminal interface. Options are for
//  a vt100 terminal or redirected file/pipe
type CLTerm interface {
	Init(prompt string)
	Cleanup()
	Println(args ...interface{})
	Printf(format string, args ...interface{})
	ReadLine() (line string, err error)
}

// GetCLTerm returns the proper command line terminal interface
func GetCLTerm(prompt string) CLTerm {
	if terminal.IsTerminal(syscall.Stdin) {
		// use a vt100 terminal
		vt100 := new(VT100Term)
		vt100.Init(prompt)
		return vt100
	}

	//Otherwise use a pipeterm
	pterm := new(PipeTerm)
	pterm.Init(prompt)
	return pterm
}
