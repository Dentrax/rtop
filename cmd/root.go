/*

rtop - the remote system monitoring utility

Copyright (c) 2015 RapidLoop

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

package cmd

import (
	"fmt"
	"github.com/rapidloop/rtop/internal/tui"
	"github.com/rapidloop/rtop/pkg/types"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/rapidloop/rtop/internal/ssh"
	"github.com/rapidloop/rtop/pkg/client"
	"github.com/spf13/cobra"
)

var (
	currentUser *user.User

	flagKeyPath  string
	flagInterval time.Duration

	cmd = &cobra.Command{
		Use:   "xdsl-exporter",
		Short: "rtop monitors server statistics over an ssh connection.",
		Long: `rtop monitors server statistics over an ssh connection." +
Usage: rtop [-i private-key-file] [-t interval] [user@]host[:port]
`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(args[0])
		},
	}
)

func Execute() {
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cmd.PersistentFlags().StringVarP(&flagKeyPath, "private-key-file", "i", "~/.ssh/id_rsa", "PEM-encoded private key file to use (default: ~/.ssh/id_rsa if present)")
	cmd.PersistentFlags().DurationVarP(&flagInterval, "interval", "t", 5*time.Second, "refresh interval in seconds")
}

func run(addr string) error {
	username, host, port, err := parseAddrAsUserHostAddrPort(addr)
	if err != nil {
		return err
	}

	keyPath := flagKeyPath
	shost, sport, suser, skeyPath, err := ssh.GetSshConfig(host, flagKeyPath)
	if err != nil {
		return err
	}
	if len(shost) > 0 {
		host = shost
	}
	if sport != 0 && port == 0 {
		port = sport
	}
	if len(suser) > 0 {
		username = suser
	}
	if len(skeyPath) > 0 {
		keyPath = skeyPath
	}

	client, err := client.New(client.WithUser(username), client.WithHost(host), client.WithPort(port), client.WithKeyPath(keyPath))
	if err != nil {
		return err
	}

	stats, err := client.GetStats()
	if err != nil {
		return err
	}

	getStats := func() (types.Stats, error) {
		stats, err := client.GetStats()
		if err != nil {
			return types.Stats{}, err
		}
		return stats, nil
	}

	renderer := tui.NewRenderingState(getStats, stats, flagInterval)
	err = renderer.Start()
	if err != nil {
		return err
	}

	return nil
}

// parseAddrAsUserHostAddrPort parses the given address user@host:port into
// username, host and port, respectively.
func parseAddrAsUserHostAddrPort(flagHost string) (string, string, int, error) {
	var user, host string
	var port int

	// user, addr
	if i := strings.Index(flagHost, "@"); i != -1 {
		user = flagHost[:i]
		host = flagHost[i+1:]
	}

	// addr -> host, port
	if p := strings.Split(host, ":"); len(p) == 2 {
		host = p[0]
		var err error
		if port, err = strconv.Atoi(p[1]); err != nil {
			return "", "", 0, fmt.Errorf("bad port: %v", err)
		}
		if port <= 0 || port >= 65536 {
			return "", "", 0, fmt.Errorf("port out of range: %d", port)
		}
	}

	return user, host, port, nil
}
