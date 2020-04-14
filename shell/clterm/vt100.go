package clterm

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
)

// VT100Term implements the vt100 implementation of CLTerm
type VT100Term struct {
	prompt      string
	descriptor  int
	backupState *terminal.State
	fileDesc    *os.File
	term        *terminal.Terminal
}

//Init sets the terminal to raw
func (vt *VT100Term) Init(prompt string) {
	var err error

	vt.prompt = prompt

	vt.descriptor = syscall.Stdin
	if !terminal.IsTerminal(vt.descriptor) {
		panic("Descriptor is not a terminal")
	}

	vt.backupState, err = terminal.MakeRaw(vt.descriptor)
	if err != nil {
		panic(err)
	}

	vt.fileDesc = os.NewFile(uintptr(vt.descriptor), "/dev/tty")
	vt.term = terminal.NewTerminal(vt.fileDesc, vt.prompt)

}

//Cleanup sets the terminal back to original state
func (vt *VT100Term) Cleanup() {
	terminal.Restore(vt.descriptor, vt.backupState)
}

//Println prints a line to the terminal, adjusting correctly for positioning
func (vt *VT100Term) Println(args ...interface{}) {
	fmt.Print(strings.ReplaceAll(fmt.Sprintln(args...), "\n", "\r\n"))
}

//Printf prints a formatted string to the terminal, adjusting correctly for positioning
func (vt *VT100Term) Printf(format string, args ...interface{}) {
	fmt.Print(strings.ReplaceAll(fmt.Sprintf(format, args...), "\n", "\r\n"))
}

// ReadLine gets a line of input from the terminal
func (vt *VT100Term) ReadLine() (line string, err error) {
	return vt.term.ReadLine()
}
