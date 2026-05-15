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
	addr   string
	links  []linkReceived
	height int
	width  int
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
	case linkReceived:
		m.links = append(m.links, msg)
	}
	return m, nil
}

var (
	badgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("86")).
			Padding(0, 1)
	versionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86"))
	addrStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
	dimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	urlStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	argsStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("118"))
	tsStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("239"))
	countStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)
)

func (m model) View() string {
	var b strings.Builder

	// Header
	n := len(m.links)
	noun := "links"
	if n == 1 {
		noun = "link"
	}
	b.WriteString(badgeStyle.Render(" Sample "))
	b.WriteString("  ")
	b.WriteString(versionStyle.Render("v" + version))
	b.WriteString("  ")
	b.WriteString(addrStyle.Render(m.addr))
	b.WriteString("  ")
	b.WriteString(countStyle.Render(fmt.Sprintf("%d %s", n, noun)))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ────────────────────────────────────────\n\n"))

	if n == 0 {
		b.WriteString(dimStyle.Render("  waiting for deep links...\n"))
	} else {
		// header=3 lines, footer=2 lines; each link is 1 + len(args) lines.
		// Walk backwards through links, collecting rows until we fill the budget.
		budget := m.height - 5
		if budget < 1 {
			budget = 10 // fallback before first WindowSizeMsg
		}
		type row struct{ s string }
		var rows []row
		for i := n - 1; i >= 0 && budget > 0; i-- {
			lk := m.links[i]
			needed := 1 + len(lk.args)
			if needed > budget {
				break
			}
			budget -= needed
			// prepend so oldest-visible is at top
			var entry []row
			entry = append(entry, row{fmt.Sprintf("  %s  %s\n",
				tsStyle.Render(lk.at.Local().Format("15:04:05")),
				urlStyle.Render(lk.url))})
			for _, arg := range lk.args {
				entry = append(entry, row{fmt.Sprintf("             %s\n",
					argsStyle.Render("▸ "+arg))})
			}
			rows = append(entry, rows...)
		}
		for _, r := range rows {
			b.WriteString(r.s)
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
	hasTTY := isInteractiveTTY(os.Stdout.Fd())

	if hasTTY {
		p := tea.NewProgram(model{addr: addr}, tea.WithAltScreen())
		go acceptLoop(ln, logPath, p)
		if _, err := p.Run(); err != nil {
			// TUI failed to initialise (e.g. not the foreground process group).
			// The acceptLoop goroutine is still running and will log deeplinks
			// to file. Block so the process stays alive and the TCP port stays open.
			fmt.Fprintf(os.Stderr, "tui: %v; continuing headless\n", err)
			<-make(chan struct{})
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
