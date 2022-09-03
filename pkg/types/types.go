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

package types

import "time"

type Stats struct {
	Uptime       time.Duration
	Hostname     string
	Loads        Loads
	CPU          CPUInfo // or []CPUInfo to get all the cpu-core's stats?
	MEM          MemInfo
	FSInfos      []FSInfo
	NetInterface map[string]NetInterface
}

type FSInfo struct {
	MountPoint string
	Total      uint64
	Used       uint64
	Free       uint64
}

type NetInterface struct {
	NetIPAddr
	NetDevInfo
}

// MergeNetInterfaces merges the given map of interfaces into single map.
func MergeNetInterfaces(addr map[string]NetIPAddr, dev map[string]NetDevInfo) map[string]NetInterface {
	keys := make([]string, 0, len(addr))

	// traverse only the keys from the addr and map the corresponding dev info
	for k := range addr {
		keys = append(keys, k)
	}

	merged := make(map[string]NetInterface, len(keys))

	for _, k := range keys {
		if d, ok := dev[k]; ok {
			merged[k] = NetInterface{
				NetIPAddr:  addr[k],
				NetDevInfo: d,
			}
		}
	}

	return merged
}

type NetIPAddr struct {
	IPv4 string
	IPv6 string
}

type NetDevInfo struct {
	Rx uint64
	Tx uint64
}

type CPURaw struct {
	User    uint64 // time spent in user mode
	Nice    uint64 // time spent in user mode with low priority (nice)
	System  uint64 // time spent in system mode
	Idle    uint64 // time spent in the idle task
	Iowait  uint64 // time spent waiting for I/O to complete (since Linux 2.5.41)
	Irq     uint64 // time spent servicing  interrupts  (since  2.6.0-test4)
	SoftIrq uint64 // time spent servicing softirqs (since 2.6.0-test4)
	Steal   uint64 // time spent in other OSes when running in a virtualized environment
	Guest   uint64 // time spent running a virtual CPU for guest operating systems under the control of the Linux kernel.
	Total   uint64 // total of all time fields
}

type CPUInfo struct {
	User    float32
	Nice    float32
	System  float32
	Idle    float32
	IOWait  float32
	IRQ     float32
	SoftIRQ float32
	Steal   float32
	Guest   float32
}

type Loads struct {
	Load1        string
	Load5        string
	Load15       string
	RunningProcs string
	TotalProcs   string
}

type MemInfo struct {
	Total     uint64
	Free      uint64
	Buffers   uint64
	Cached    uint64
	SwapTotal uint64
	SwapFree  uint64
}

func (m MemInfo) Used() uint64 {
	return m.Total - m.Free - m.Buffers - m.Cached
}
