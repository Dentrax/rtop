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

package client

import (
	"bufio"
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/semgroup"
	"github.com/rapidloop/rtop/internal/ssh"
	"github.com/rapidloop/rtop/pkg/types"
)

type Client struct {
	// sshClient is the ssh client to use for executing commands on the remote host
	sshClient *ssh.Client
	workers   int
}

func New(opts ...Option) (*Client, error) {
	o := &option{}

	for _, opt := range opts {
		opt(o)
	}

	if o.workers == 0 {
		o.workers = runtime.NumCPU()
	}

	sshClient, err := ssh.NewClient(o.user, o.host, o.port, o.keypath, o.sshClient)
	if err != nil {
		return nil, err
	}

	return &Client{
		sshClient: sshClient,
		workers:   o.workers,
	}, nil
}

func (c *Client) GetStats() (types.Stats, error) {
	s := semgroup.NewGroup(context.Background(), int64(c.workers))

	var uptime time.Duration
	var hostname string
	var loads types.Loads
	var mem types.MemInfo
	var cpu types.CPUInfo
	var fsInfos []types.FSInfo
	var netIpAddrs map[string]types.NetIPAddr
	var netDevInfos map[string]types.NetDevInfo

	s.Go(func() error {
		var err error
		uptime, err = c.GetUptime()
		return err
	})
	s.Go(func() error {
		var err error
		hostname, err = c.GetHostname()
		return err
	})
	s.Go(func() error {
		var err error
		loads, err = c.GetLoad()
		return err
	})
	s.Go(func() error {
		var err error
		mem, err = c.GetMemInfo()
		return err
	})
	s.Go(func() error {
		var err error
		fsInfos, err = c.GetFSInfos()
		return err
	})
	s.Go(func() error {
		var err error
		netIpAddrs, err = c.GetNetIPAddrs()
		return err
	})
	s.Go(func() error {
		var err error
		netDevInfos, err = c.GetNetDevInfos()
		return err
	})
	s.Go(func() error {
		var err error
		cpu, err = c.GetCPU()
		return err
	})

	err := s.Wait()

	netInterface := types.MergeNetInterfaces(netIpAddrs, netDevInfos)

	return types.Stats{
		Uptime:       uptime,
		Hostname:     hostname,
		Loads:        loads,
		CPU:          cpu,
		MEM:          mem,
		FSInfos:      fsInfos,
		NetInterface: netInterface,
	}, err
}

func (c *Client) GetUptime() (time.Duration, error) {
	uptime, err := c.sshClient.Execute("/bin/cat /proc/uptime")
	if err != nil {
		return 0, fmt.Errorf("execute /bin/cat /proc/uptime: %s", err)
	}

	parts := strings.Fields(uptime)
	if len(parts) == 2 {
		var upsecs float64
		upsecs, err = strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, err
		}
		return time.Duration(upsecs * 1e9), nil
	}

	return 0, fmt.Errorf("unexpected uptime format: %s", uptime)
}

func (c *Client) GetHostname() (string, error) {
	hostname, err := c.sshClient.Execute("/bin/hostname -f")
	if err != nil {
		hostname, err = c.sshClient.Execute("/bin/hostname")
		if err != nil {
			return "", fmt.Errorf("execute /bin/hostname: %s", err)
		}
	}

	return strings.TrimSpace(hostname), nil
}

func (c *Client) GetLoad() (types.Loads, error) {
	line, err := c.sshClient.Execute("/bin/cat /proc/loadavg")
	if err != nil {
		return types.Loads{}, fmt.Errorf("execute /bin/cat /proc/loadavg: %s", err)
	}

	var res types.Loads

	parts := strings.Fields(line)
	if len(parts) == 5 {
		res.Load1 = parts[0]
		res.Load5 = parts[1]
		res.Load15 = parts[2]
		if i := strings.Index(parts[3], "/"); i != -1 {
			res.RunningProcs = parts[3][0:i]
			if i+1 < len(parts[3]) {
				res.TotalProcs = parts[3][i+1:]
			}
		}
		return res, nil
	}

	return types.Loads{}, fmt.Errorf("unexpected loadavg format: %s", line)
}

func (c *Client) GetMemInfo() (types.MemInfo, error) {
	lines, err := c.sshClient.Execute("/bin/cat /proc/meminfo")
	if err != nil {
		return types.MemInfo{}, fmt.Errorf("execute /bin/cat /proc/meminfo: %s", err)
	}

	var res types.MemInfo

	scanner := bufio.NewScanner(strings.NewReader(lines))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 3 {
			val, err := strconv.ParseUint(parts[1], 10, 64)
			if err != nil {
				continue
			}
			val *= 1024
			switch parts[0] {
			case "MemTotal:":
				res.Total = val
			case "MemFree:":
				res.Free = val
			case "Buffers:":
				res.Buffers = val
			case "Cached:":
				res.Cached = val
			case "SwapTotal:":
				res.SwapTotal = val
			case "SwapFree:":
				res.SwapFree = val
			}
		}
	}

	return res, nil
}

func (c *Client) GetFSInfos() ([]types.FSInfo, error) {
	lines, err := c.sshClient.Execute("/bin/df -B1")
	if err != nil {
		lines, err = c.sshClient.Execute("/bin/df")
		if err != nil {
			return nil, fmt.Errorf("execute /bin/df: %s", err)
		}
	}

	var res []types.FSInfo

	scanner := bufio.NewScanner(strings.NewReader(lines))
	flag := 0
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		n := len(parts)
		dev := n > 0 && strings.Index(parts[0], "/dev/") == 0
		if n == 1 && dev {
			flag = 1
		} else {
			i := flag
			flag = 0
			total, err := strconv.ParseUint(parts[1-i], 10, 64)
			if err != nil {
				continue
			}
			used, err := strconv.ParseUint(parts[2-i], 10, 64)
			if err != nil {
				continue
			}
			free, err := strconv.ParseUint(parts[3-i], 10, 64)
			if err != nil {
				continue
			}
			res = append(res, types.FSInfo{
				MountPoint: parts[5-i],
				Total:      total,
				Used:       used,
				Free:       free,
			})
		}
	}

	return res, nil
}

func (c *Client) GetNetIPAddrs() (map[string]types.NetIPAddr, error) {
	var lines string
	lines, err := c.sshClient.Execute("/bin/ip -o addr")
	if err != nil {
		lines, err = c.sshClient.Execute("/sbin/ip -o addr")
		if err != nil {
			return nil, fmt.Errorf("execute /bin/ip -o addr: %s", err)
		}
	}

	res := make(map[string]types.NetIPAddr)

	scanner := bufio.NewScanner(strings.NewReader(lines))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 4 && (parts[2] == "inet" || parts[2] == "inet6") {
			ipv4 := parts[2] == "inet"
			intfname := parts[1]
			if info, ok := res[intfname]; ok {
				if ipv4 {
					info.IPv4 = parts[3]
				} else {
					info.IPv6 = parts[3]
				}
				res[intfname] = info
			} else {
				info := types.NetIPAddr{}
				if ipv4 {
					info.IPv4 = parts[3]
				} else {
					info.IPv6 = parts[3]
				}
				res[intfname] = info
			}
		}
	}

	return res, nil
}

func (c *Client) GetNetDevInfos() (map[string]types.NetDevInfo, error) {
	lines, err := c.sshClient.Execute("/bin/cat /proc/net/dev")
	if err != nil {
		return nil, fmt.Errorf("execute /bin/cat /proc/net/dev: %s", err)
	}

	res := make(map[string]types.NetDevInfo)

	scanner := bufio.NewScanner(strings.NewReader(lines))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 17 {
			intf := strings.TrimSpace(parts[0])
			intf = strings.TrimSuffix(intf, ":")
			info := types.NetDevInfo{}
			rx, err := strconv.ParseUint(parts[1], 10, 64)
			if err != nil {
				continue
			}
			tx, err := strconv.ParseUint(parts[9], 10, 64)
			if err != nil {
				continue
			}
			info.Rx = rx
			info.Tx = tx
			res[intf] = info
		}
	}

	return res, nil
}

func (c *Client) GetCPU() (types.CPUInfo, error) {
	lines, err := c.sshClient.Execute("/bin/cat /proc/stat")
	if err != nil {
		return types.CPUInfo{}, fmt.Errorf("execute /bin/cat /proc/stat: %s", err)
	}

	var nowCPU types.CPURaw

	scanner := bufio.NewScanner(strings.NewReader(lines))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "cpu" { // changing here if want to get every cpu-core's stats
			parseCPUFields(&nowCPU, fields)
			break
		}
	}

	total := float32(nowCPU.Total)

	return types.CPUInfo{
		User:    float32(nowCPU.User) / total * 100,
		Nice:    float32(nowCPU.Nice) / total * 100,
		System:  float32(nowCPU.System) / total * 100,
		Idle:    float32(nowCPU.Idle) / total * 100,
		IOWait:  float32(nowCPU.Iowait) / total * 100,
		IRQ:     float32(nowCPU.Irq) / total * 100,
		SoftIRQ: float32(nowCPU.SoftIrq) / total * 100,
		Steal:   float32(nowCPU.Steal) / total * 100,
		Guest:   float32(nowCPU.Guest) / total * 100,
	}, nil
}

func parseCPUFields(cpu *types.CPURaw, fields []string) {
	numFields := len(fields)
	for i := 1; i < numFields; i++ {
		val, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			continue
		}

		cpu.Total += val
		switch i {
		case 1:
			cpu.User = val
		case 2:
			cpu.Nice = val
		case 3:
			cpu.System = val
		case 4:
			cpu.Idle = val
		case 5:
			cpu.Iowait = val
		case 6:
			cpu.Irq = val
		case 7:
			cpu.SoftIrq = val
		case 8:
			cpu.Steal = val
		case 9:
			cpu.Guest = val
		}
	}
}
