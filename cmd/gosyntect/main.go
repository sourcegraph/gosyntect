package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sourcegraph/gosyntect"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("gosyntect: ")
	if len(os.Args) != 4 {
		fmt.Println("usage: gosyntect <server> <theme> <file.go>")
		fmt.Println("")
		fmt.Println("example:")
		fmt.Println("	gosyntect http://localhost:9238 'InspiredGitHub' gosyntect.go")
		fmt.Println("")
		os.Exit(2)
	}

	// Validate server argument.
	server := os.Args[1]
	if !strings.HasPrefix(server, "http://") && !strings.HasPrefix(server, "https://") {
		log.Fatal("expected server to have http:// or https:// prefix")
	}

	// Validate theme argument.
	theme := os.Args[2]
	if theme == "" {
		log.Fatal("theme argument is required (e.x. 'InspiredGitHub')")
	}

	// Validate file argument.
	file := os.Args[3]
	data, err := os.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}

	var stabilizeTimeout time.Duration
	if v := os.Getenv("STABILIZE_TIMEOUT"); v != "" {
		stabilizeTimeout, err = time.ParseDuration(v)
		if err != nil {
			log.Fatal(err)
		}
	}

	cl := gosyntect.New(server)
	resp, err := cl.Highlight(context.Background(), &gosyntect.Query{
		Filepath:         filepath.Base(file),
		Theme:            theme,
		Code:             string(data),
		StabilizeTimeout: stabilizeTimeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp.Data)
}
