package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type DeeplinkMessage struct {
	Type   string `json:"type"`
	URL    string `json:"url"`
	From   string `json:"from,omitempty"`
	At     string `json:"at,omitempty"`
	Mapped struct {
		Mode string   `json:"mode"`
		URL  string   `json:"url,omitempty"`
		Args []string `json:"args,omitempty"`
		JSON string   `json:"json,omitempty"`
	} `json:"mapped"`
}

func main() {
	// TCP address passed as argv[1], default for local testing
	addr := "127.0.0.1:59595"
	if len(os.Args) > 1 && strings.TrimSpace(os.Args[1]) != "" {
		addr = os.Args[1]
	}

	appName := "Your Desktop App"
	dataDir := resolveDataDir(appName)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "data dir: %v\n", err)
		os.Exit(2)
	}
	_ = os.WriteFile(filepath.Join(dataDir, "hello.txt"),
		[]byte("hello from "+appName+" @ "+time.Now().Format(time.RFC3339)+"\n"), 0o644)

	logPath := filepath.Join(dataDir, "messages.log")
	fmt.Printf("%s listening on %s\n", appName, addr)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(3)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleConn(conn, logPath)
	}
}

func handleConn(c net.Conn, logPath string) {
	defer c.Close()
	r := bufio.NewReader(c)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			trim := strings.TrimRight(string(line), "\r\n")
			var msg DeeplinkMessage
			_ = json.Unmarshal([]byte(trim), &msg)
			appendLine(logPath, trim+"\n")
			fmt.Printf("received deeplink: %s (mode=%s)\n", msg.URL, msg.Mapped.Mode)
		}
		if err != nil {
			return
		}
	}
}

func appendLine(path, s string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(s)
}

func resolveDataDir(appName string) string {
	home, _ := os.UserHomeDir()
	clean := func(s string) string {
		s = strings.ReplaceAll(s, "/", "-")
		s = strings.ReplaceAll(s, "\\", "-")
		return strings.TrimSpace(s)
	}
	name := clean(appName)

	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, name)
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", name)
	default:
		state := os.Getenv("XDG_STATE_HOME")
		if state == "" {
			state = filepath.Join(home, ".local", "state")
		}
		return filepath.Join(state, name)
	}
}
