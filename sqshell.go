package main

import (
	"bufio"
	"fmt"
	"io"

	"net"
	"os"
	"strconv"
	s "strings"
	"time"

	log "github.com/sirupsen/logrus"
	protocol "github.com/wilphi/sqsrv/sqprotocol"
	"github.com/wilphi/sqsrv/sqprotocol/client"
)

const version = "v0.2.02"

func main() {
	// setup logging
	logFile, err := os.OpenFile("sqshell.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		panic(err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	log.SetLevel(log.DebugLevel)
	log.Info("SQShell " + version)
	log.Println("Connecting to server....")
	conn, err := net.Dial("tcp", "localhost:3333")
	if err != nil {
		log.Fatal("Error connecting to server ...", err.Error())
	}
	defer conn.Close()

	myClient := client.SetConn(conn)

	args := os.Args[1:]
	if len(args) > 0 {
		err = readSQFromFile(myClient, args[0])
		if err != nil {
			log.Fatal("Error reading from file", err)
		}
	} else {
		ReadFromStream(os.Stdin, myClient)
	}

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
		if s.TrimSpace(input) != "" {
			cmd := s.Fields(input)
			log.Trace(cmd)
			log.Trace(len(cmd))
			log.Trace("Is first Rune ", getFirstRune(input))
			if s.ToLower(cmd[0]) == "exit" {
				log.Println("Good bye!!")
				os.Exit(0)
			}
			if getFirstRune(input) == '@' {
				fmt.Printf("Reading from... %q\n", input[1:len(input)-1])
				err = readSQFromFile(myClient, input[1:len(input)-1])
				if err != nil {
					log.Trace("ReadSQFromFile returns error: ", err)
					break
				}
			} else {
				req := protocol.RequestToServer{Cmd: input[:len(input)-1]}
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
func readSQFromFile(myClient *client.Config, filename string) error {
	start := time.Now()

	file, err := os.Open(filename)
	if err != nil {
		log.Println(err)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := s.TrimSpace(scanner.Text())
		if line != "" {
			req := protocol.RequestToServer{Cmd: scanner.Text()}
			err = handleRequest(myClient, req)
			if err != nil {
				return err
			}
		}
	}

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
