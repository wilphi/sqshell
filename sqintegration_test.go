package main_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

var ShellDir, SrvDir string

const ShellCmd = "./sqshell"
const SrvCmd = "/sqsrv"

func TestMain(m *testing.M) {
	// setup logging
	logFile, err := os.OpenFile("sqintegration_test.log", os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		panic(err)
	}
	log.SetOutput(logFile)

	log.Println("Building Client")
	// build client
	cmd := exec.Command("go", "build")
	err = cmd.Run()
	if err != nil {
		log.Println("Unable to build sqshell: ", err)
		os.Exit(-1)
	}

	// change directory
	ShellDir, err = os.Getwd()
	if err != nil {
		log.Println("Unable to get client directory: ", err)
		os.Exit(-1)
	}

	err = os.Chdir("../sqsrv")
	if err != nil {
		log.Println("Unable to change directory to sqsrv: ", err)
		os.Exit(-1)
	}
	SrvDir, err = os.Getwd()
	if err != nil {
		log.Println("Unable to get Server directory: ", err)
		os.Exit(-1)
	}

	log.Println("Building Server")
	srv := exec.Command("go", "build")
	err = srv.Run()
	if err != nil {
		log.Println("Unable to build sqsrv: ", err)
		os.Exit(-1)
	}

	err = os.Chdir(ShellDir)
	if err != nil {
		log.Println("Unable to change directory to sqshell: ", err)
		os.Exit(-1)
	}
	os.Exit(m.Run())

}

func pipeToCmd(Sourcefile, cmdStr string, cmdArgs ...string) (string, error) {
	source, err := ioutil.ReadFile(Sourcefile)
	if err != nil {
		return "", err
	}

	cmd := exec.Command(cmdStr, cmdArgs...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, string(source))
	}()

	result, err := cmd.CombinedOutput()
	return string(result), err
}

type CliData struct {
	TestName     string
	Args         []string
	CmdFile      string
	ResultFile   string
	NeedsServer  bool
	Timeout      time.Duration
	ExpectSrvErr bool
	StartDelay   time.Duration
}

func testCliFunc(d CliData) func(t *testing.T) {
	return func(t *testing.T) {
		defer func() {
			r := recover()
			if r != nil {
				t.Errorf(t.Name() + " panicked unexpectedly")
			}
		}()

		if d.NeedsServer {
			log.Println("Starting Server...")
			ctx, cancel := context.WithTimeout(context.Background(), d.Timeout*2)
			defer cancel()

			tmpDir := os.TempDir()
			testDir, err := ioutil.TempDir(tmpDir, "sqshelltest")
			if err != nil {
				t.Error(t.Name() + " Unable to create temp dir")
				return
			}
			defer os.RemoveAll(testDir)
			args := []string{
				"-dbfile=" + testDir,
				"-tlog=" + testDir + "/trans.tlog",
			}

			cmd := exec.CommandContext(ctx, SrvDir+SrvCmd, args...)

			err = cmd.Start()
			if err != nil {
				t.Errorf("%s: Error starting server: %s", t.Name(), err)
				return
			}

			time.Sleep(d.StartDelay)

			defer func() {
				err := cmd.Wait()
				if err != nil && !d.ExpectSrvErr {
					t.Errorf("%s: Error from server: %s", t.Name(), err)
				}
			}()
		}
		actual, err := pipeToCmd(d.CmdFile, "./sqshell", d.Args...)
		if err != nil {
			t.Error(err)
			return
		}
		//	log.Println(">>>", d.TestName, strings.Repeat("-", 77-len(d.TestName)))
		//	log.Println(actual)
		//	log.Println(">>>", strings.Repeat("-", 78))

		// load file with Expected data
		txtFile, err := ioutil.ReadFile(d.ResultFile)
		if err != nil {
			t.Error(err)
			return
		}
		expected := string(txtFile)
		res, msg, err := fileDiff(strings.NewReader(actual), strings.NewReader(expected))
		if !res {
			t.Errorf(msg)
			return
		}
	}
}

func fileDiff(actual, expected io.Reader) (bool, string, error) {
	var err error
	line := 0

	ExpScan := bufio.NewScanner(expected)
	ActScan := bufio.NewScanner(actual)
	for ExpScan.Scan() {
		line++
		if !ActScan.Scan() {
			err = ActScan.Err()
			if err != nil {
				return false, fmt.Sprintf("Error in Actual File @: line#%d - %q", line, ExpScan.Text()), err
			}
			return false, fmt.Sprintf("Actual file is shorter that Expected @: line#%d - %q", line, ExpScan.Text()), nil
		}
		if ExpScan.Text() != ActScan.Text() {
			return false, fmt.Sprintf("Mismatch in line %d\nExpected: %q\n  Actual: %q", line, ExpScan.Text(), ActScan.Text()), nil
		}
	}
	err = ExpScan.Err()
	if err != nil {
		return false, fmt.Sprintf("Error scanning Expected after line %d", line), err
	}
	if ActScan.Scan() {
		return false, fmt.Sprintf("Actual file has more text after end of Expected: %q", ActScan.Text()), nil
	}
	err = ActScan.Err()
	if err != nil {
		return false, fmt.Sprintf("Error scanning Actual after line %d", line), err
	}
	return true, "Perfect Match", nil

}
func TestCli(t *testing.T) {
	data := []CliData{
		{
			TestName:    "NoServerNoArgs",
			Args:        []string{},
			CmdFile:     "./testdata/cmds/exit.txt",
			ResultFile:  "./testdata/results/noservernoargs.txt",
			NeedsServer: false,
			Timeout:     100 * time.Millisecond,
		},
		{
			TestName:    "NoServerPortArg",
			Args:        []string{"-port=2222"},
			CmdFile:     "./testdata/cmds/exit.txt",
			ResultFile:  "./testdata/results/noserverportarg.txt",
			NeedsServer: false,
			Timeout:     100 * time.Millisecond,
		},
		{
			TestName:    "NoServerHostArg",
			Args:        []string{"-host=test"},
			CmdFile:     "./testdata/cmds/exit.txt",
			ResultFile:  "./testdata/results/noserverhostarg.txt",
			NeedsServer: false,
			Timeout:     100 * time.Millisecond,
		},
		{
			TestName:    "NoServerShowTables",
			Args:        []string{},
			CmdFile:     "./testdata/cmds/showtables.txt",
			ResultFile:  "./testdata/results/noservershowtables.txt",
			NeedsServer: false,
			Timeout:     100 * time.Millisecond,
		},
		{
			TestName:     "ServerNoArgs",
			Args:         []string{},
			CmdFile:      "./testdata/cmds/exit.txt",
			ResultFile:   "./testdata/results/servernoargs.txt",
			NeedsServer:  true,
			Timeout:      100 * time.Millisecond,
			StartDelay:   50 * time.Millisecond,
			ExpectSrvErr: true,
		},
		{
			TestName:     "CreateDropTables",
			Args:         []string{},
			CmdFile:      "./testdata/cmds/createdroptables.txt",
			ResultFile:   "./testdata/results/createdroptables.txt",
			NeedsServer:  true,
			Timeout:      100 * time.Millisecond,
			StartDelay:   50 * time.Millisecond,
			ExpectSrvErr: true,
		},
		{
			TestName:     "SmallMultiSQ",
			Args:         []string{"-port=3333"},
			CmdFile:      "./testdata/cmds/smallmulti.sq",
			ResultFile:   "./testdata/results/smallmulti.txt",
			NeedsServer:  true,
			Timeout:      500 * time.Millisecond,
			StartDelay:   50 * time.Millisecond,
			ExpectSrvErr: false,
		},
	}

	for i, row := range data {
		t.Run(fmt.Sprintf("%d: %s", i, row.TestName),
			testCliFunc(row))

	}
}
