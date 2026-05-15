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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var version = "dev"

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

type linkReceived struct {
	at   time.Time
	url  string
	args []string
}

// ---- Bubble Tea model ----

type model struct {
	addr  string
	links []linkReceived
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case linkReceived:
		m.links = append(m.links, msg)
	}
	return m, nil
}

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	urlStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	argsStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
)

func (m model) View() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render(fmt.Sprintf("Sample  v%s", version)))
	b.WriteString(dimStyle.Render(fmt.Sprintf("   %s\n\n", m.addr)))

	if len(m.links) == 0 {
		b.WriteString(dimStyle.Render("  Waiting for deep links...\n"))
	} else {
		start := 0
		if len(m.links) > 30 {
			start = len(m.links) - 30
		}
		for _, lk := range m.links[start:] {
			ts := dimStyle.Render(lk.at.Local().Format("15:04:05"))
			b.WriteString(fmt.Sprintf("  %s  %s\n", ts, urlStyle.Render(lk.url)))
			if len(lk.args) > 0 {
				b.WriteString(fmt.Sprintf("           %s\n", argsStyle.Render("→ "+strings.Join(lk.args, " "))))
			}
		}
	}

	b.WriteString(dimStyle.Render("\n  q quit"))
	return b.String()
}

// ---- main ----

func main() {
	addr := "127.0.0.1:59595"
	if len(os.Args) > 1 && strings.TrimSpace(os.Args[1]) != "" {
		addr = os.Args[1]
	}

	dataDir := resolveDataDir("Sample")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "data dir: %v\n", err)
		os.Exit(2)
	}
	_ = os.WriteFile(filepath.Join(dataDir, "hello.txt"),
		[]byte(fmt.Sprintf("hello from Sample v%s @ %s\n", version, time.Now().Format(time.RFC3339))), 0o644)

	logPath := filepath.Join(dataDir, "messages.log")

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(3)
	}

	// Detect whether we have a terminal to render a TUI in.
	hasTTY := isTerminal(os.Stdout.Fd())

	if hasTTY {
		p := tea.NewProgram(model{addr: addr}, tea.WithAltScreen())
		go acceptLoop(ln, logPath, p)
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "tui: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Headless: just accept connections and log to file.
		acceptLoop(ln, logPath, nil)
	}
}

func acceptLoop(ln net.Listener, logPath string, p *tea.Program) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go handleConn(conn, logPath, p)
	}
}

func handleConn(c net.Conn, logPath string, p *tea.Program) {
	defer c.Close()
	r := bufio.NewReader(c)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			trim := strings.TrimRight(string(line), "\r\n")
			var msg DeeplinkMessage
			if jsonErr := json.Unmarshal([]byte(trim), &msg); jsonErr == nil {
				appendLine(logPath, trim+"\n")
				if p != nil {
					lk := linkReceived{at: time.Now(), url: msg.URL}
					if msg.Mapped.Mode == "args" {
						lk.args = msg.Mapped.Args
					}
					p.Send(lk)
				}
			}
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
