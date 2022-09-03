/*

rtop-bot - remote system monitoring bot

Copyright (c) 2015 RapidLoop
Copyright (c) 2022 Furkan Türkal

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package tui

import (
	"bytes"
	"fmt"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rapidloop/rtop/pkg/types"
	"sort"
	"time"
)

type (
	tickMsg    time.Time
	getStatsFn func() (types.Stats, error)
)

type Rendering struct {
	getStatsFn getStatsFn
	stats      types.Stats
	tick       tea.Cmd
	w, h       int
	ready      bool
	viewport   viewport.Model
}

func NewRenderingState(getStatsFn getStatsFn, stats types.Stats, interval time.Duration) *tea.Program {
	rendering := &Rendering{
		getStatsFn: getStatsFn,
		stats:      stats,
		tick: tea.Tick(interval, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
	}

	return tea.NewProgram(rendering, tea.WithAltScreen(), tea.WithMouseCellMotion())
}

func (r Rendering) Init() tea.Cmd {
	return r.tick
}

func (r Rendering) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return r, tea.Quit
		}
	case tickMsg:
		if r.ready {
			b := r.render()
			r.viewport.SetContent(b.String())
		}
		return r, nil

	case tea.WindowSizeMsg:
		if !r.ready {
			r.viewport = viewport.New(msg.Width, msg.Height)
			r.viewport.HighPerformanceRendering = false
			b := r.render()
			r.viewport.SetContent(b.String())
			r.ready = true
		} else {
			r.viewport.Width = msg.Width
			r.viewport.Height = msg.Height
		}
		return r, nil
	}

	r.viewport, cmd = r.viewport.Update(msg)
	cmds = append(cmds, cmd)
	//cmds = append(cmds, r.tick)

	return r, tea.Batch(cmds...)
}

func (r Rendering) View() string {
	return r.viewport.View()
}

func (r Rendering) render() bytes.Buffer {
	TEMPLATE := `%s up %s

Load:
    %s %s %s

CPU:
    %s user, %s sys, %s nice, %s idle, %s iowait, %s hardirq, %s softirq, %s steal, %s guest

Processes:
    %s running of %s total

Memory:
    total   = %s
    free    = %s
    used    = %s
    buffers = %s
    cached  = %s
    swap    = %s free of %s

`

	w := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)

	var b bytes.Buffer

	fmt.Fprintf(&b,
		TEMPLATE,
		w.Render(r.stats.Hostname),
		w.Render(fmtUptime(r.stats.Uptime)),
		w.Render(r.stats.Loads.Load1),
		w.Render(r.stats.Loads.Load5),
		w.Render(r.stats.Loads.Load15),
		w.Render(fmt.Sprintf("%.2f", r.stats.CPU.User)),
		w.Render(fmt.Sprintf("%.2f", r.stats.CPU.System)),
		w.Render(fmt.Sprintf("%.2f", r.stats.CPU.Nice)),
		w.Render(fmt.Sprintf("%.2f", r.stats.CPU.Idle)),
		w.Render(fmt.Sprintf("%.2f", r.stats.CPU.IOWait)),
		w.Render(fmt.Sprintf("%.2f", r.stats.CPU.IRQ)),
		w.Render(fmt.Sprintf("%.2f", r.stats.CPU.SoftIRQ)),
		w.Render(fmt.Sprintf("%.2f", r.stats.CPU.Steal)),
		w.Render(fmt.Sprintf("%.2f", r.stats.CPU.Guest)),
		w.Render(r.stats.Loads.RunningProcs),
		w.Render(r.stats.Loads.TotalProcs),
		w.Render(fmtBytes(r.stats.MEM.Total)),
		w.Render(fmtBytes(r.stats.MEM.Free)),
		w.Render(fmtBytes(r.stats.MEM.Used())),
		w.Render(fmtBytes(r.stats.MEM.Buffers)),
		w.Render(fmtBytes(r.stats.MEM.Cached)),
		w.Render(fmtBytes(r.stats.MEM.SwapFree)),
		w.Render(fmtBytes(r.stats.MEM.SwapTotal)),
	)

	if len(r.stats.FSInfos) > 0 {
		b.WriteString("Filesystems:\n")
		for _, fs := range r.stats.FSInfos {
			b.WriteString(fmt.Sprintf("    %8s: %s free of %s\n",
				w.Render(fs.MountPoint),
				w.Render(fmtBytes(fs.Free)),
				w.Render(fmtBytes(fs.Total)),
			))
		}
		b.WriteString("\n")
	}

	if len(r.stats.NetInterface) > 0 {
		b.WriteString("Network Interfaces:\n")

		keys := make([]string, 0, len(r.stats.NetInterface))
		for k := range r.stats.NetInterface {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			info := r.stats.NetInterface[key]

			b.WriteString(fmt.Sprintf("    %s - %s",
				w.Render(key),
				w.Render(info.IPv4),
			))
			if len(info.IPv6) > 0 {
				b.WriteString(fmt.Sprintf(", %s\n",
					w.Render(info.IPv6),
				))
			} else {
				b.WriteString("\n")
			}
			b.WriteString(fmt.Sprintf("      rx = %s, tx = %s\n",
				w.Render(fmtBytes(info.Rx)),
				w.Render(fmtBytes(info.Tx)),
			))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b
}

func fmtUptime(uptime time.Duration) string {
	dur := uptime
	dur = dur - (dur % time.Second)
	var days int
	for dur.Hours() > 24.0 {
		days++
		dur -= 24 * time.Hour
	}
	s1 := dur.String()
	s2 := ""
	if days > 0 {
		s2 = fmt.Sprintf("%dd ", days)
	}
	for _, ch := range s1 {
		s2 += string(ch)
		if ch == 'h' || ch == 'm' {
			s2 += " "
		}
	}
	return s2
}

func fmtBytes(val uint64) string {
	if val < 1024 {
		return fmt.Sprintf("%d bytes", val)
	} else if val < 1024*1024 {
		return fmt.Sprintf("%6.2f KiB", float64(val)/1024.0)
	} else if val < 1024*1024*1024 {
		return fmt.Sprintf("%6.2f MiB", float64(val)/1024.0/1024.0)
	} else {
		return fmt.Sprintf("%6.2f GiB", float64(val)/1024.0/1024.0/1024.0)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
