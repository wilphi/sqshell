package shell_test

import (
	"fmt"
	"strings"
)

// mock a term struct
//MockTerm contains the state of the command line terminal
type MockTerm struct {
	prompt        string
	consoleReader *strings.Reader
}

//Init setup the MockTerm
func (mt *MockTerm) Init(prompt string) {
	mt.prompt = prompt
	mt.consoleReader = strings.NewReader("")
}

//Cleanup sets the terminal back to original state
func (mt *MockTerm) Cleanup() {
	// No cleanup to be done
}

//Println prints a line to the terminal, adjusting correctly for positioning
func (mt *MockTerm) Println(args ...interface{}) {
	fmt.Println(args...)
}

//Printf prints a formatted string to the terminal, adjusting correctly for positioning
func (mt *MockTerm) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// ReadLine gets a line of input from the terminal
func (mt *MockTerm) ReadLine() (line string, err error) {
	return "nil", nil //mt.consoleReader.ReadLine('\n')
}

// Reset the Reader with string
func (mt *MockTerm) Reset(s string) {
	mt.consoleReader.Reset(s)
}
