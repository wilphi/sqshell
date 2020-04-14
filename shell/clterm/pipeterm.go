package clterm

import (
	"bufio"
	"fmt"
	"os"
)

//PipeTerm contains the state of the command line terminal
type PipeTerm struct {
	prompt        string
	consoleReader *bufio.Reader
}

//Init setup the pipeterm
func (pt *PipeTerm) Init(prompt string) {
	pt.prompt = prompt
	pt.consoleReader = bufio.NewReader(os.Stdin)
}

//Cleanup sets the terminal back to original state
func (pt *PipeTerm) Cleanup() {
	// No cleanup to be done
}

//Println prints a line to the terminal, adjusting correctly for positioning
func (pt *PipeTerm) Println(args ...interface{}) {
	fmt.Println(args...)
}

//Printf prints a formatted string to the terminal, adjusting correctly for positioning
func (pt *PipeTerm) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// ReadLine gets a line of input from the terminal
func (pt *PipeTerm) ReadLine() (line string, err error) {
	return pt.consoleReader.ReadString('\n')
}
