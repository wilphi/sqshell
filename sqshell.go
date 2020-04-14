package main

import "github.com/wilphi/sqshell/shell"

const version = "v0.12.00"

func main() {
	shell.GetArgs()
	shell.Start(version)

}
