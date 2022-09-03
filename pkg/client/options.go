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

import "golang.org/x/crypto/ssh"

type option struct {
	user      string
	host      string
	port      int
	keypath   string
	workers   int
	sshClient *ssh.Client
}

type Option func(o *option)

func WithUser(user string) Option {
	return func(o *option) {
		o.user = user
	}
}

func WithHost(host string) Option {
	return func(o *option) {
		o.host = host
	}
}

func WithPort(port int) Option {
	return func(o *option) {
		o.port = port
	}
}

func WithKeyPath(keypath string) Option {
	return func(o *option) {
		o.keypath = keypath
	}
}

func WithSSHClient(sshClient *ssh.Client) Option {
	return func(o *option) {
		o.sshClient = sshClient
	}
}

func WithWorkers(workers int) Option {
	return func(o *option) {
		o.workers = workers
	}
}
