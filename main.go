package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	fmt.Println("Sitemap-Go Content Monitor")
	fmt.Println("Usage:")
	fmt.Println("  go run cmd/server/main.go [flags]")
	fmt.Println("")
	fmt.Println("Available commands:")
	fmt.Println("  server  - Start the monitoring server")
	fmt.Println("")
	fmt.Println("For detailed usage, run:")
	fmt.Println("  go run cmd/server/main.go -h")
	
	if len(os.Args) > 1 && os.Args[1] == "server" {
		cmd := exec.Command("go", "run", "cmd/server/main.go")
		cmd.Args = append(cmd.Args, os.Args[2:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		
		if err := cmd.Run(); err != nil {
			fmt.Printf("Error running server: %v\n", err)
			os.Exit(1)
		}
	}
}