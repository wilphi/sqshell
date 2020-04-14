package shell

import (
	"bufio"
	"flag"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wilphi/sqshell/shell/clterm"
	"github.com/wilphi/sqsrv/sqprotocol"
)

var args struct {
	host string
	port string
	exit bool
	cmds []string
}

var _validConn bool

func setValidConn(b bool) {
	_validConn = b
	//term.Printf("Valid Conn = %v\r\n", _validConn)
}

// GetArgs gets the arguments passed to main
func GetArgs() {

	flag.StringVar(&args.host, "host", "localhost", "Host name of the server")
	flag.StringVar(&args.port, "port", "3333", "TCP port for server to listen on")
	flag.BoolVar(&args.exit, "e", false, "Execute command line args and exit immediately")
	flag.Parse()
	args.cmds = flag.Args()
	return
}

// Start the shell to communicate with the sqsrv
func Start(version string) {
	setValidConn(false)
	term := clterm.GetCLTerm("sqshell$ ")
	defer term.Cleanup()

	term.Println("SQShell " + version)
	term.Println("Connecting to server....")

	myClient, err := newSrvClient(term, args.host+":"+args.port)
	setValidConn(err == nil)

	defer myClient.Close()

	if len(args.cmds) > 0 {
		err = readSQFromFile(term, myClient, args.cmds)
		if err != nil {
			term.Println("Error reading from file", err)
			return
		}
	}
	if args.exit {
		return
	}
	readFromTerm(term, myClient)

}

// newSrvClient creates a new connection to the sqsrv
func newSrvClient(term clterm.CLTerm, addr string) (*sqprotocol.ClientConfig, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		term.Println("Error connecting to server ...", err.Error())
		return nil, err
	}
	return sqprotocol.SetClientConn(conn), nil
}

// readFromTerm -
func readFromTerm(term clterm.CLTerm, myClient *sqprotocol.ClientConfig) {

exit:
	for {
		// Read the keyboad input.
		input, err := term.ReadLine()

		if err != nil {
			if err == io.EOF {
				break exit
			}
			term.Println(err)
		}
		input = strings.TrimSpace(input)
		if input != "" {
			cmd := strings.Fields(input)
			switch strings.ToLower(cmd[0]) {
			case "exit":
				term.Println("Good bye!!")
				break exit
			case "disconnect":
				term.Println("Disconnecting...")
				if myClient != nil {
					myClient.Close()
				}
				setValidConn(false)
				continue
			case "connect":
				addr := args.host + ":" + args.port
				if len(cmd) > 1 {
					connstr := strings.Split(cmd[1], ":")
					if len(connstr) != 2 {
						term.Printf("Invalid connection string: %q", addr)
						continue
					}
					args.host = connstr[0]
					args.port = connstr[1]
				}
				if myClient != nil {
					myClient.Close()
				}
				term.Printf("Connecting %s...\n", addr)
				myClient, err = newSrvClient(term, addr)
				setValidConn(err == nil)

				if err != nil {
					term.Printf("Unable to connect to %q", addr)
					continue

				}
				continue
			}
			if getFirstRune(input) == '@' {
				term.Printf("Reading from... %q", input[1:])
				args := strings.Fields(input[1:])
				if len(args) < 1 {
					term.Println("No file specified.")
					continue
				}
				err = readSQFromFile(term, myClient, args)
				if err != nil {
					continue
				}
			} else {
				if _validConn {
					err = lineToServer(term, myClient, input)
					if err != nil {
						//term.Println(err.Error())
						continue
					}
				} else {
					term.Println("Invalid Connection")
					continue
				}
			}
		}
	}

}

func lineToServer(term clterm.CLTerm, myClient *sqprotocol.ClientConfig, line string) error {
	req := sqprotocol.RequestToServer{Cmd: line}
	err := handleRequest(term, myClient, req)
	if err != nil {
		if err == io.EOF {
			setValidConn(false)
			term.Println("Server disconnected...")
		} else {
			term.Println("Error returned from Server", err)
		}
	}
	return err
}
func clientPool(term clterm.CLTerm, sqlChan chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	myClient, err := newSrvClient(term, args.host+":"+args.port)
	if err != nil {
		term.Println(err)
		return
	}
	defer myClient.Close()
	for {
		line, ok := <-sqlChan
		if !ok {
			return
		}
		lineToServer(term, myClient, line)
	}
}

func readSQFromFile(term clterm.CLTerm, myClient *sqprotocol.ClientConfig, args []string) error {
	var numClient, nProtect int
	var err error
	start := time.Now()

	filename := args[0]
	if len(args) > 1 {
		numClient, err = strconv.Atoi(args[1])
		if err != nil {
			term.Println("Invalid number of conncurrent clients")
			numClient = 1
		}
	} else {
		numClient = 1
	}
	if len(args) > 2 {
		// Number of statements to protect
		nProtect, err = strconv.Atoi(args[2])
		if err != nil {
			nProtect = 1
		}
	}
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			term.Printf("File %q does not exist\n", filename)
			return nil
		}
		term.Println("Error opening file: ", err)
		return err
	}
	defer file.Close()

	// create channel & wait group
	sqlChan := make(chan string, numClient*2)
	var wg sync.WaitGroup
	if numClient > 1 {
		for i := 0; i < numClient; i++ {
			wg.Add(1)
			go clientPool(term, sqlChan, &wg)
		}
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			if nProtect > 0 || numClient == 1 {
				// do this line sequentially
				err := lineToServer(term, myClient, line)
				if err != nil {
					term.Printf("Unable to execute line %s due to error: %s", line, err.Error())
					return err
				}
				nProtect--
			} else {
				sqlChan <- line
			}
		}
	}
	close(sqlChan)
	wg.Wait()
	if serr := scanner.Err(); serr != nil {
		return err
	}
	elapsed := time.Since(start)
	term.Printf("Elapsed time for file: %s\n", elapsed.Round(10*time.Millisecond))
	return nil

}
func getFirstRune(str string) rune {
	for _, c := range str {
		return c
	}
	return ' '
}
func handleRequest(term clterm.CLTerm, myClient *sqprotocol.ClientConfig, req sqprotocol.RequestToServer) error {

	err := myClient.SendRequest(req)
	if err != nil {
		return err
	}

	resp, err := myClient.ReceiveResponse()
	if err != nil {
		return err
	}

	if resp.IsErr {
		term.Println(resp.Msg)
		return nil
	}
	if resp.CMDResponse {
		term.Println(resp.Msg)
		return nil

	}

	if resp.HasData {
		// Get Column information (Name & Width)
		colHeaders, err := myClient.ReceiveColumns(resp.NCols)
		if err != nil {
			return err
		}
		// Display column headers
		for _, colH := range colHeaders {
			term.Printf("%"+strconv.Itoa(colH.Width)+"s\t", colH.ColName)
		}
		term.Println()

		for i := 0; i < resp.NRows; i++ {
			rw, err := myClient.ReceiveRow()
			if err != nil {
				return err
			}
			if rw == nil {
				break
			}
			for _, d := range rw.Data {
				term.Printf("%"+strconv.Itoa(d.Len())+"s\t", d.ToString())
			}
			term.Println()

		}
		term.Printf("%d Rows returned\n", resp.NRows)
		return nil
	}

	term.Println(resp.Msg, resp.NRows, "Rows returned")

	return nil
}
