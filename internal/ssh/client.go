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

package ssh

import (
	"bufio"
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/terminal"
)

type Client struct {
	conn   net.Conn
	client *ssh.Client
}

func NewClient(user, host string, port int, keypath string, client *ssh.Client) (*Client, error) {
	// if an ssh client is provided, use it. otherwise, try to initialize one.
	if client != nil {
		return &Client{client: client}, nil
	}

	if port == 0 {
		port = 22
	}

	addr := fmt.Sprintf("%s:%d", host, port)

	// try connecting via agent first
	sshClient := tryAgentConnect(user, addr)
	if sshClient != nil {
		return nil, nil
	}

	// if that failed try with the key and password methods
	auths := make([]ssh.AuthMethod, 0, 2)
	auths = addKeyAuth(auths, keypath)
	auths = addPasswordAuth(user, addr, auths)

	config := &ssh.ClientConfig{
		User: user,
		Auth: auths,
		HostKeyCallback: func(string, net.Addr, ssh.PublicKey) error {
			return nil
		},
	}
	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, err
	}

	return &Client{
		client: sshClient,
	}, nil
}

func (c *Client) Execute(command string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	err = session.Run(command)

	if err != nil {
		return "", err
	}

	return string(buf.Bytes()), nil
}

func tryAgentConnect(user, addr string) (client *ssh.Client) {
	if auth, ok := getAgentAuth(); ok {
		config := &ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{auth},
		}
		client, _ = ssh.Dial("tcp", addr, config)
	}

	return
}

func getAgentAuth() (auth ssh.AuthMethod, ok bool) {
	if sock := os.Getenv("SSH_AUTH_SOCK"); len(sock) > 0 {
		if agconn, err := net.Dial("unix", sock); err == nil {
			ag := agent.NewClient(agconn)
			auth = ssh.PublicKeysCallback(ag.Signers)
			ok = true
		}
	}
	return
}

func addKeyAuth(auths []ssh.AuthMethod, keypath string) []ssh.AuthMethod {
	if len(keypath) == 0 {
		return auths
	}

	keypath, err := homedir.Expand(keypath)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}

	// read the file
	pemBytes, err := os.ReadFile(keypath)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}

	// get first pem block
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		log.Printf("no key found in %s", keypath)
		return auths
	}

	// handle plain and encrypted keyfiles
	if x509.IsEncryptedPEMBlock(block) {
		prompt := fmt.Sprintf("Enter passphrase for key '%s': ", keypath)
		pass, err := readPass(prompt)
		if err != nil {
			return auths
		}
		block.Bytes, err = x509.DecryptPEMBlock(block, []byte(pass))
		if err != nil {
			log.Print(err)
			return auths
		}
		key, err := ParsePemBlock(block)
		if err != nil {
			log.Print(err)
			return auths
		}
		signer, err := ssh.NewSignerFromKey(key)
		if err != nil {
			log.Print(err)
			return auths
		}
		return append(auths, ssh.PublicKeys(signer))
	} else {
		signer, err := ssh.ParsePrivateKey(pemBytes)
		if err != nil {
			log.Print(err)
			return auths
		}
		return append(auths, ssh.PublicKeys(signer))
	}
}

func addPasswordAuth(user, addr string, auths []ssh.AuthMethod) []ssh.AuthMethod {
	if terminal.IsTerminal(0) == false {
		return auths
	}
	host := addr
	if i := strings.LastIndex(host, ":"); i != -1 {
		host = host[:i]
	}
	prompt := fmt.Sprintf("%s@%s's password: ", user, host)
	passwordCallback := func() (string, error) {
		return readPass(prompt)
	}
	return append(auths, ssh.PasswordCallback(passwordCallback))
}

func readPass(prompt string) (string, error) {
	tstate, err := terminal.GetState(0)
	if err != nil {
		return "", err
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		quit := false
		for range sig {
			quit = true
			break
		}
		terminal.Restore(0, tstate)
		if quit {
			fmt.Println()
			os.Exit(2)
		}
	}()
	defer func() {
		signal.Stop(sig)
		close(sig)
	}()

	f := bufio.NewWriter(os.Stdout)
	f.Write([]byte(prompt))
	f.Flush()

	defer func() {
		f.Write([]byte("\n"))
		f.Flush()
	}()

	passbytes, err := terminal.ReadPassword(0)
	if err != nil {
		return "", err
	}

	return string(passbytes), nil
}
