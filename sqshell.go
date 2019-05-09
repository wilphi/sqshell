package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"sync"

	"net"
	"os"
	"strconv"
	s "strings"
	"time"

	log "github.com/sirupsen/logrus"
	protocol "github.com/wilphi/sqsrv/sqprotocol"
	"github.com/wilphi/sqsrv/sqprotocol/client"
)

const version = "v0.5.04"
const connString = "localhost:3333"

func main() {
	// setup logging
	logFile, err := os.OpenFile("sqshell.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		panic(err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	log.SetLevel(log.InfoLevel)
	log.Info("SQShell " + version)
	log.Println("Connecting to server....")

	myClient, err := NewSrvClient(connString)
	if err != nil {
		return
	}
	defer myClient.Close()

	args := os.Args[1:]
	if len(args) > 0 {
		err = readSQFromFile(myClient, args)
		if err != nil {
			log.Fatal("Error reading from file", err)
		}
	} else {
		ReadFromStream(os.Stdin, myClient)
	}

}

// NewSrvClient creates a new connection to the sqsrv
func NewSrvClient(addr string) (*client.Config, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Error("Error connecting to server ...", err.Error())
		return nil, err
	}
	return client.SetConn(conn), nil
}

// ReadFromStream -
func ReadFromStream(rd io.Reader, myClient *client.Config) {
	reader := bufio.NewReader(rd)

	for {
		fmt.Print("sqshell$ ")
		// Read the keyboad input.
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		input = s.TrimSpace(input)
		if input != "" {
			cmd := s.Fields(input)
			log.Trace(cmd)
			log.Trace(len(cmd))
			log.Trace("Is first Rune ", getFirstRune(input))
			if s.ToLower(cmd[0]) == "exit" {
				log.Println("Good bye!!")
				os.Exit(0)
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
					log.Trace("ReadSQFromFile returns error: ", err)
					break
				}
			} else {
				req := protocol.RequestToServer{Cmd: input}
				log.Trace("Sending request to server: ", req)
				err = handleRequest(myClient, req)
				if err != nil {
					log.Trace("Error returned from SendRequest", err)
					break
				}

			}
		}
	}

}

func clientPool(sqlChan chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	myClient, err := NewSrvClient(connString)
	if err != nil {
		log.Error(err)
		return
	}
	defer myClient.Close()
	for {
		line, ok := <-sqlChan
		if !ok {
			return
		}
		req := protocol.RequestToServer{Cmd: line}
		err = handleRequest(myClient, req)
		if err != nil {
			log.Error(err)
		}
	}
}

func readSQFromFile(myClient *client.Config, args []string) error {
	var numClient int
	var err error
	start := time.Now()

	filename := args[0]
	if len(args) > 1 {
		numClient, err = strconv.Atoi(args[1])
		if err != nil {
			log.Info("Invalid number of conncurrent clients")
			numClient = 1
		}
	} else {
		numClient = 1
	}
	file, err := os.Open(filename)
	if err != nil {
		log.Println(err)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	// create channel & wait group
	sqlChan := make(chan string, numClient*2)
	var wg sync.WaitGroup
	for i := 0; i < numClient; i++ {
		wg.Add(1)
		go clientPool(sqlChan, &wg)
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := s.TrimSpace(scanner.Text())
		if line != "" {
			sqlChan <- line
		}
	}
	close(sqlChan)
	wg.Wait()
	if serr := scanner.Err(); serr != nil {
		return err
	}
	elapsed := time.Since(start)
	log.Printf("Elapsed time for file: %s", elapsed.Round(10*time.Millisecond))
	return nil

}
func getFirstRune(str string) rune {
	for _, c := range str {
		return c
	}
	return ' '
}
func handleRequest(myClient *client.Config, req protocol.RequestToServer) error {

	err := myClient.SendRequest(req)
	if err != nil {
		return err
	}

	resp, err := myClient.ReceiveResponse()
	if err != nil {
		return err
	}

	if resp.IsErr {
		fmt.Println("Error received from server: ", resp.Msg)
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
				fmt.Printf("%"+strconv.Itoa(d.GetLen())+"s\t", d.ToString())
			}
			fmt.Println()

		}
		fmt.Printf("%d Rows returned\n", resp.NRows)
		return nil
	}

	log.Println(resp.Msg, resp.NRows, "Rows returned")

	return nil
}
