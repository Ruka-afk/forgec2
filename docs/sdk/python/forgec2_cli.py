#!/usr/bin/env python3
"""
ForgeC2 Command Line Interface
==============================

A powerful CLI tool for interacting with ForgeC2 C2 framework.

Installation:
    pip install forgec2-cli

Usage:
    forgec2-cli login
    forgec2-cli agents
    forgec2-cli execute <agent_id> <command>
"""

import argparse
import sys
import os
import json
from datetime import datetime, timedelta
from typing import Optional, List
from getpass import getpass

# Import SDK
try:
    from forgec2 import ForgeC2Client, ForgeC2Error
except ImportError:
    print("Error: forgec2 SDK not installed. Run: pip install forgec2")
    sys.exit(1)


class Colors:
    """Terminal colors."""
    RED = '\033[91m'
    GREEN = '\033[92m'
    YELLOW = '\033[93m'
    BLUE = '\033[94m'
    MAGENTA = '\033[95m'
    CYAN = '\033[96m'
    WHITE = '\033[97m'
    RESET = '\033[0m'
    BOLD = '\033[1m'


def print_success(msg: str):
    """Print success message."""
    print(f"{Colors.GREEN}✓ {msg}{Colors.RESET}")


def print_error(msg: str):
    """Print error message."""
    print(f"{Colors.RED}✗ {msg}{Colors.RESET}")


def print_info(msg: str):
    """Print info message."""
    print(f"{Colors.CYAN}ℹ {msg}{Colors.RESET}")


def print_warning(msg: str):
    """Print warning message."""
    print(f"{Colors.YELLOW}⚠ {msg}{Colors.RESET}")


def print_table(headers: List[str], rows: List[List[str]], 
                col_widths: Optional[List[int]] = None):
    """Print formatted table."""
    if not col_widths:
        col_widths = [max(len(str(row[i])) for row in [headers] + rows) + 2 
                      for i in range(len(headers))]
    
    # Header
    header_line = ''.join(f"{h:<{col_widths[i]}}" for i, h in enumerate(headers))
    print(f"{Colors.BOLD}{header_line}{Colors.RESET}")
    print('-' * sum(col_widths))
    
    # Rows
    for row in rows:
        row_line = ''.join(f"{str(cell):<{col_widths[i]}}" 
                          for i, cell in enumerate(row))
        print(row_line)


class ForgeC2CLI:
    """ForgeC2 Command Line Interface."""
    
    def __init__(self):
        """Initialize CLI."""
        self.config_file = os.path.expanduser('~/.forgec2/config.json')
        self.client: Optional[ForgeC2Client] = None
        self.load_config()
    
    def load_config(self):
        """Load configuration from file."""
        if os.path.exists(self.config_file):
            try:
                with open(self.config_file, 'r') as f:
                    config = json.load(f)
                    
                    server = config.get('server', 'http://localhost:8080')
                    self.client = ForgeC2Client(server)
                    self.client.token = config.get('token')
            except:
                self.client = ForgeC2Client('http://localhost:8080')
        else:
            self.client = ForgeC2Client('http://localhost:8080')
    
    def save_config(self):
        """Save configuration to file."""
        os.makedirs(os.path.dirname(self.config_file), exist_ok=True)
        
        config = {
            'server': self.client.base_url,
            'token': self.client.token,
        }
        
        with open(self.config_file, 'w') as f:
            json.dump(config, f)
    
    def cmd_login(self, args):
        """Login to ForgeC2 server."""
        server = args.server or input("Server URL [http://localhost:8080]: ") or 'http://localhost:8080'
        username = args.username or input("Username: ")
        password = args.password or getpass("Password: ")
        
        try:
            self.client = ForgeC2Client(server)
            user = self.client.login(username, password)
            
            print_success(f"Logged in as {user.username} ({user.role})")
            self.save_config()
        except ForgeC2Error as e:
            print_error(f"Login failed: {e}")
            sys.exit(1)
    
    def cmd_agents(self, args):
        """List all agents."""
        if not self.client.token:
            print_error("Not logged in. Run: forgec2-cli login")
            sys.exit(1)
        
        try:
            agents = self.client.get_agents()
            
            if not agents:
                print_warning("No agents found")
                return
            
            headers = ['ID', 'Hostname', 'IP', 'OS', 'User', 'Status', 'Last Seen']
            rows = []
            
            for agent in agents:
                status = f"{Colors.GREEN}{agent.status}{Colors.RESET}" if agent.status == 'online' \
                    else f"{Colors.RED}{agent.status}{Colors.RESET}"
                
                last_seen = agent.last_seen.strftime('%Y-%m-%d %H:%M') if agent.last_seen else 'N/A'
                
                rows.append([
                    agent.id[:8],
                    agent.hostname,
                    agent.ip,
                    agent.os,
                    agent.username,
                    status,
                    last_seen,
                ])
            
            print_table(headers, rows)
        except ForgeC2Error as e:
            print_error(f"Failed to get agents: {e}")
    
    def cmd_execute(self, args):
        """Execute a shell command on an agent."""
        if not self.client.token:
            print_error("Not logged in. Run: forgec2-cli login")
            sys.exit(1)
        
        try:
            print_info(f"Executing command on {args.agent_id}...")
            
            task = self.client.execute_shell(args.agent_id, args.command)
            print_info(f"Task {task.id} created, waiting for result...")
            
            # Wait for completion
            result_task = self.client.wait_for_task(task.id, timeout=args.timeout)
            
            if result_task.status == 'completed':
                print_success("Command executed successfully")
                print(f"\n{Colors.BOLD}Result:{Colors.RESET}\n")
                print(result_task.result or "No output")
            elif result_task.status == 'failed':
                print_error(f"Task failed: {result_task.result}")
            else:
                print_warning(f"Task status: {result_task.status}")
                
        except TimeoutError:
            print_error(f"Timeout waiting for task completion")
        except ForgeC2Error as e:
            print_error(f"Failed to execute command: {e}")
    
    def cmd_screenshot(self, args):
        """Take a screenshot."""
        if not self.client.token:
            print_error("Not logged in. Run: forgec2-cli login")
            sys.exit(1)
        
        try:
            print_info(f"Taking screenshot on {args.agent_id}...")
            
            task = self.client.take_screenshot(args.agent_id)
            result_task = self.client.wait_for_task(task.id)
            
            if result_task.status == 'completed':
                print_success(f"Screenshot saved: {result_task.result}")
            else:
                print_error(f"Screenshot failed: {result_task.result}")
                
        except ForgeC2Error as e:
            print_error(f"Failed to take screenshot: {e}")
    
    def cmd_download(self, args):
        """Download a file from agent."""
        if not self.client.token:
            print_error("Not logged in. Run: forgec2-cli login")
            sys.exit(1)
        
        try:
            print_info(f"Downloading {args.path} from {args.agent_id}...")
            
            task = self.client.download_file(args.agent_id, args.path)
            result_task = self.client.wait_for_task(task.id)
            
            if result_task.status == 'completed':
                print_success(f"File downloaded: {result_task.result}")
            else:
                print_error(f"Download failed: {result_task.result}")
                
        except ForgeC2Error as e:
            print_error(f"Failed to download file: {e}")
    
    def cmd_upload(self, args):
        """Upload a file to agent."""
        if not self.client.token:
            print_error("Not logged in. Run: forgec2-cli login")
            sys.exit(1)
        
        try:
            if not os.path.exists(args.local_path):
                print_error(f"Local file not found: {args.local_path}")
                sys.exit(1)
            
            print_info(f"Uploading {args.local_path} to {args.agent_id}:{args.remote_path}...")
            
            task = self.client.upload_file(args.agent_id, args.remote_path, args.local_path)
            result_task = self.client.wait_for_task(task.id)
            
            if result_task.status == 'completed':
                print_success(f"File uploaded successfully")
            else:
                print_error(f"Upload failed: {result_task.result}")
                
        except ForgeC2Error as e:
            print_error(f"Failed to upload file: {e}")
    
    def cmd_listeners(self, args):
        """List all listeners."""
        if not self.client.token:
            print_error("Not logged in. Run: forgec2-cli login")
            sys.exit(1)
        
        try:
            listeners = self.client.get_listeners()
            
            if not listeners:
                print_warning("No listeners found")
                return
            
            headers = ['ID', 'Name', 'Protocol', 'Host', 'Port', 'Status']
            rows = []
            
            for listener in listeners:
                status = f"{Colors.GREEN}{listener.status}{Colors.RESET}" if listener.status == 'active' \
                    else f"{Colors.RED}{listener.status}{Colors.RESET}"
                
                rows.append([
                    listener.id,
                    listener.name,
                    listener.protocol,
                    listener.host,
                    listener.port,
                    status,
                ])
            
            print_table(headers, rows)
        except ForgeC2Error as e:
            print_error(f"Failed to get listeners: {e}")
    
    def cmd_credentials(self, args):
        """List all credentials."""
        if not self.client.token:
            print_error("Not logged in. Run: forgec2-cli login")
            sys.exit(1)
        
        try:
            credentials = self.client.get_credentials()
            
            if not credentials:
                print_warning("No credentials found")
                return
            
            headers = ['ID', 'Agent', 'Source', 'Username', 'Password/Hash']
            rows = []
            
            for cred in credentials:
                secret = cred.password or cred.hash or 'N/A'
                if len(secret) > 20:
                    secret = secret[:20] + '...'
                
                rows.append([
                    cred.id,
                    cred.agent_id[:8],
                    cred.source,
                    cred.username,
                    secret,
                ])
            
            print_table(headers, rows)
        except ForgeC2Error as e:
            print_error(f"Failed to get credentials: {e}")


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description='ForgeC2 Command Line Interface',
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    
    subparsers = parser.add_subparsers(dest='command', help='Commands')
    
    # Login command
    login_parser = subparsers.add_parser('login', help='Login to ForgeC2 server')
    login_parser.add_argument('--server', '-s', help='Server URL')
    login_parser.add_argument('--username', '-u', help='Username')
    login_parser.add_argument('--password', '-p', help='Password')
    
    # Agents command
    agents_parser = subparsers.add_parser('agents', help='List all agents')
    
    # Execute command
    exec_parser = subparsers.add_parser('execute', help='Execute shell command')
    exec_parser.add_argument('agent_id', help='Agent ID')
    exec_parser.add_argument('command', help='Command to execute')
    exec_parser.add_argument('--timeout', '-t', type=int, default=60, 
                            help='Timeout in seconds')
    
    # Screenshot command
    screen_parser = subparsers.add_parser('screenshot', help='Take screenshot')
    screen_parser.add_argument('agent_id', help='Agent ID')
    
    # Download command
    download_parser = subparsers.add_parser('download', help='Download file')
    download_parser.add_argument('agent_id', help='Agent ID')
    download_parser.add_argument('path', help='Remote file path')
    
    # Upload command
    upload_parser = subparsers.add_parser('upload', help='Upload file')
    upload_parser.add_argument('agent_id', help='Agent ID')
    upload_parser.add_argument('remote_path', help='Remote destination path')
    upload_parser.add_argument('local_path', help='Local file path')
    
    # Listeners command
    listeners_parser = subparsers.add_parser('listeners', help='List listeners')
    
    # Credentials command
    creds_parser = subparsers.add_parser('credentials', help='List credentials')
    
    args = parser.parse_args()
    
    if not args.command:
        parser.print_help()
        sys.exit(1)
    
    cli = ForgeC2CLI()
    
    commands = {
        'login': cli.cmd_login,
        'agents': cli.cmd_agents,
        'execute': cli.cmd_execute,
        'screenshot': cli.cmd_screenshot,
        'download': cli.cmd_download,
        'upload': cli.cmd_upload,
        'listeners': cli.cmd_listeners,
        'credentials': cli.cmd_credentials,
    }
    
    if args.command in commands:
        commands[args.command](args)
    else:
        parser.print_help()


if __name__ == '__main__':
    main()
