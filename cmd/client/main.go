package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcClient "github.com/anon-d/gophKeeper/internal/client/grpc"
	"github.com/anon-d/gophKeeper/internal/client/tui"
)

var (
	version   = "dev"
	buildDate = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("gophkeeper-client %s (built %s)\n", version, buildDate)
		return
	}

	addr := "localhost:44044"
	if v := os.Getenv("SERVER_ADDR"); v != "" {
		addr = v
	}

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := grpcClient.New(conn)
	app := tui.NewApp(client)

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
