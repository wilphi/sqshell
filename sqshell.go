package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"strings"
	"sync"

	"net"
	"os"
	"strconv"
	"time"

	"github.com/wilphi/sqsrv/sqprotocol"
)

const version = "v0.7.00"

var host, port *string

func main() {
	host = flag.String("host", "localhost", "Host name of the server")
	port = flag.String("port", "3333", "TCP port for server to listen on")
	flag.Parse()

	fmt.Println("SQShell " + version)
	fmt.Println("Connecting to server....")

	myClient, err := NewSrvClient(*host + ":" + *port)
	if err != nil {
		// dont return
	}
	defer myClient.Close()

	args := flag.Args()
	if len(args) > 0 {
		err = readSQFromFile(myClient, args)
		if err != nil {
			fmt.Println("Error reading from file", err)
			os.Exit(0)
		}
	}

	ReadFromStream(os.Stdin, myClient)

}

// NewSrvClient creates a new connection to the sqsrv
func NewSrvClient(addr string) (*sqprotocol.ClientConfig, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Println("Error connecting to server ...", err.Error())
		return nil, err
	}
	return sqprotocol.SetClientConn(conn), nil
}

// ReadFromStream -
func ReadFromStream(rd io.Reader, myClient *sqprotocol.ClientConfig) {
	reader := bufio.NewReader(rd)

	for {
		fmt.Print("sqshell$ ")
		// Read the keyboad input.
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		input = strings.TrimSpace(input)
		if input != "" {
			cmd := strings.Fields(input)
			switch strings.ToLower(cmd[0]) {
			case "exit":
				fmt.Println("Good bye!!")
				os.Exit(0)
			case "disconnect":
				fmt.Println("Disconnecting...")
				if myClient != nil {
					myClient.Close()
				}
				continue
			case "connect":
				fmt.Println("Connecting...")
				connstr := strings.Split(cmd[1], ":")
				if len(connstr) != 2 {
					fmt.Printf("Invalid connection string: %q\n", cmd[1])
					continue
				}
				host = &connstr[0]
				port = &connstr[1]
				if myClient != nil {
					myClient.Close()
				}
				myClient, err = NewSrvClient(cmd[1])
				if err != nil {
					fmt.Printf("Unable to connect to %q\n", cmd[1])
					continue

				}
				continue
			}
			if getFirstRune(input) == '@' {
				fmt.Printf("Reading from... %q\n", input[1:len(input)])
				args := strings.Fields(input[1:len(input)])
				if len(args) < 1 {
					fmt.Println("No file specified.")
					continue
				}
				err = readSQFromFile(myClient, args)
				if err != nil {
					continue
				}
			} else {
				err = lineToServer(myClient, input)
				if err != nil {
					fmt.Println(err)
					//fmt.Println(err)
					continue
				}
			}
		}
	}

}

func lineToServer(myClient *sqprotocol.ClientConfig, line string) error {
	req := sqprotocol.RequestToServer{Cmd: line}
	err := handleRequest(myClient, req)
	if err != nil {
		fmt.Println("Error returned from Server", err)
	}
	return err
}
func clientPool(sqlChan chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	myClient, err := NewSrvClient(*host + ":" + *port)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer myClient.Close()
	for {
		line, ok := <-sqlChan
		if !ok {
			return
		}
		lineToServer(myClient, line)
	}
}

func readSQFromFile(myClient *sqprotocol.ClientConfig, args []string) error {
	var numClient, nProtect int
	var err error
	start := time.Now()

	filename := args[0]
	if len(args) > 1 {
		numClient, err = strconv.Atoi(args[1])
		if err != nil {
			fmt.Println("Invalid number of conncurrent clients")
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
			fmt.Printf("File %q does not exist\n", filename)
			return nil
		}
		fmt.Println("Error opening file: ", err)
		return err
	}
	defer file.Close()

	// create channel & wait group
	sqlChan := make(chan string, numClient*2)
	var wg sync.WaitGroup
	if numClient > 1 {
		for i := 0; i < numClient; i++ {
			wg.Add(1)
			go clientPool(sqlChan, &wg)
		}
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			if nProtect > 0 || numClient == 1 {
				// do this line sequentially
				err := lineToServer(myClient, line)
				if err != nil {
					fmt.Printf("Unable to execute line %s due to error: %s", line, err.Error())
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
	fmt.Printf("Elapsed time for file: %s", elapsed.Round(10*time.Millisecond))
	return nil

}
func getFirstRune(str string) rune {
	for _, c := range str {
		return c
	}
	return ' '
}
func handleRequest(myClient *sqprotocol.ClientConfig, req sqprotocol.RequestToServer) error {

	err := myClient.SendRequest(req)
	if err != nil {
		return err
	}

	resp, err := myClient.ReceiveResponse()
	if err != nil {
		return err
	}

	if resp.IsErr {
		fmt.Println(resp.Msg)
		return nil
	}
	if resp.CMDResponse {
		fmt.Println(resp.Msg)
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
			fmt.Printf("%"+strconv.Itoa(colH.Width)+"s\t", colH.ColName)
		}
		fmt.Println()

		for i := 0; i < resp.NRows; i++ {
			rw, err := myClient.ReceiveRow()
			if err != nil {
				return err
			}
			if rw == nil {
				break
			}
			for _, d := range rw.Data {
				fmt.Printf("%"+strconv.Itoa(d.Len())+"s\t", d.ToString())
			}
			fmt.Println()

		}
		fmt.Printf("%d Rows returned\n", resp.NRows)
		return nil
	}

	fmt.Println(resp.Msg, resp.NRows, "Rows returned")

	return nil
}
