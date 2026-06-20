"""
ForgeC2 SDK - Client
====================

Main client class for interacting with the ForgeC2 API.
"""

import requests
import time
from typing import List, Optional, Dict, Any, Union
from datetime import datetime
from urllib.parse import urljoin

from .models import Agent, Task, Listener, Credential, AuditLog, User, Report


class ForgeC2Error(Exception):
    """Base exception for ForgeC2 SDK errors."""
    
    def __init__(self, message: str, status_code: Optional[int] = None, 
                 response: Optional[requests.Response] = None):
        super().__init__(message)
        self.status_code = status_code
        self.response = response
    
    @classmethod
    def from_response(cls, response: requests.Response) -> 'ForgeC2Error':
        """Create error from HTTP response."""
        try:
            data = response.json()
            message = data.get('error', 'Unknown error')
        except:
            message = response.text or f"HTTP {response.status_code}"
        
        return cls(message, response.status_code, response)


class ForgeC2Client:
    """
    ForgeC2 API Client
    
    A comprehensive client for interacting with the ForgeC2 Command & Control API.
    
    Example:
        >>> client = ForgeC2Client('http://localhost:8080')
        >>> client.login('admin', 'admin')
        >>> agents = client.get_agents()
        >>> for agent in agents:
        ...     print(f"{agent.hostname}: {agent.status}")
    """
    
    def __init__(self, base_url: str, timeout: int = 30, verify_ssl: bool = False):
        """
        Initialize ForgeC2 client.
        
        Args:
            base_url: Base URL of the ForgeC2 server (e.g., 'http://localhost:8080')
            timeout: Request timeout in seconds
            verify_ssl: Whether to verify SSL certificates
        """
        self.base_url = base_url.rstrip('/')
        self.timeout = timeout
        self.verify_ssl = verify_ssl
        self.token: Optional[str] = None
        self.session = requests.Session()
        
        if not verify_ssl:
            import urllib3
            urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)
    
    def __enter__(self):
        """Context manager entry."""
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        """Context manager exit."""
        self.session.close()
    
    def _request(self, method: str, endpoint: str, **kwargs) -> requests.Response:
        """Make an authenticated HTTP request."""
        url = f"{self.base_url}{endpoint}"
        
        headers = kwargs.pop('headers', {})
        if self.token:
            headers['Authorization'] = f'Bearer {self.token}'
        headers['Content-Type'] = 'application/json'
        
        kwargs.setdefault('timeout', self.timeout)
        kwargs.setdefault('verify', self.verify_ssl)
        
        response = self.session.request(method, url, headers=headers, **kwargs)
        
        if not response.ok:
            raise ForgeC2Error.from_response(response)
        
        return response
    
    def _get(self, endpoint: str, params: Optional[Dict] = None) -> requests.Response:
        """Make GET request."""
        return self._request('GET', endpoint, params=params)
    
    def _post(self, endpoint: str, data: Optional[Dict] = None, **kwargs) -> requests.Response:
        """Make POST request."""
        return self._request('POST', endpoint, json=data, **kwargs)
    
    def _delete(self, endpoint: str) -> requests.Response:
        """Make DELETE request."""
        return self._request('DELETE', endpoint)
    
    # Authentication
    
    def login(self, username: str, password: str) -> User:
        """
        Login and obtain access token.
        
        Args:
            username: Username
            password: Password
        
        Returns:
            User object
        
        Raises:
            ForgeC2Error: If login fails
        """
        response = self._post('/api/auth/login', {
            'username': username,
            'password': password
        })
        
        data = response.json()
        self.token = data.get('token')
        
        return User.from_dict(data.get('user', {}))
    
    def logout(self):
        """Logout and invalidate token."""
        try:
            self._post('/api/auth/logout')
        except:
            pass
        finally:
            self.token = None
    
    # Agents
    
    def get_agents(self, status: Optional[str] = None, 
                   listener_id: Optional[int] = None) -> List[Agent]:
        """
        Get all agents.
        
        Args:
            status: Filter by status ('online' or 'offline')
            listener_id: Filter by listener ID
        
        Returns:
            List of Agent objects
        """
        params = {}
        if status:
            params['status'] = status
        if listener_id:
            params['listener_id'] = listener_id
        
        response = self._get('/api/agents', params=params)
        return [Agent.from_dict(a) for a in response.json()]
    
    def get_agent(self, agent_id: str) -> Agent:
        """Get a specific agent by ID."""
        response = self._get(f'/api/agents/{agent_id}')
        return Agent.from_dict(response.json())
    
    def delete_agent(self, agent_id: str) -> bool:
        """Delete an agent."""
        self._delete(f'/api/agents/{agent_id}')
        return True
    
    # Tasks
    
    def get_tasks(self, agent_id: str, status: Optional[str] = None) -> List[Task]:
        """
        Get all tasks for an agent.
        
        Args:
            agent_id: Agent ID
            status: Filter by status ('pending', 'completed', 'failed')
        
        Returns:
            List of Task objects
        """
        params = {}
        if status:
            params['status'] = status
        
        response = self._get(f'/api/agents/{agent_id}/tasks', params=params)
        return [Task.from_dict(t) for t in response.json()]
    
    def get_task(self, task_id: int) -> Task:
        """Get a specific task by ID."""
        response = self._get(f'/api/tasks/{task_id}')
        return Task.from_dict(response.json())
    
    def create_task(self, agent_id: str, task_type: str, **kwargs) -> Task:
        """
        Create a new task.
        
        Args:
            agent_id: Agent ID
            task_type: Task type (shell, screenshot, etc.)
            **kwargs: Additional task parameters
        
        Returns:
            Created Task object
        """
        data = {'type': task_type, **kwargs}
        response = self._post(f'/api/agents/{agent_id}/tasks', data)
        return Task.from_dict(response.json())
    
    def execute_shell(self, agent_id: str, command: str, 
                      shell: str = 'cmd.exe') -> Task:
        """
        Execute a shell command.
        
        Args:
            agent_id: Agent ID
            command: Command to execute
            shell: Shell to use (cmd.exe or powershell.exe)
        
        Returns:
            Created Task object
        """
        return self.create_task(agent_id, 'shell', 
                               command=command, shell=shell)
    
    def take_screenshot(self, agent_id: str) -> Task:
        """Take a screenshot."""
        return self.create_task(agent_id, 'screenshot')
    
    def get_processes(self, agent_id: str) -> Task:
        """List running processes."""
        return self.create_task(agent_id, 'ps')
    
    def list_files(self, agent_id: str, path: str) -> Task:
        """List files in a directory."""
        return self.create_task(agent_id, 'ls', command=path)
    
    def download_file(self, agent_id: str, path: str) -> Task:
        """Download a file from agent."""
        return self.create_task(agent_id, 'download', command=path)
    
    def upload_file(self, agent_id: str, remote_path: str, 
                    local_path: str) -> Task:
        """Upload a file to agent."""
        import base64
        
        with open(local_path, 'rb') as f:
            data = base64.b64encode(f.read()).decode()
        
        return self.create_task(agent_id, 'upload', 
                               command=remote_path, data=data)
    
    def wait_for_task(self, task_id: int, timeout: int = 60, 
                     poll_interval: float = 1.0) -> Task:
        """
        Wait for a task to complete.
        
        Args:
            task_id: Task ID
            timeout: Maximum wait time in seconds
            poll_interval: Polling interval in seconds
        
        Returns:
            Completed Task object
        
        Raises:
            TimeoutError: If task doesn't complete within timeout
        """
        start_time = time.time()
        
        while time.time() - start_time < timeout:
            task = self.get_task(task_id)
            
            if task.status != 'pending':
                return task
            
            time.sleep(poll_interval)
        
        raise TimeoutError(f"Task {task_id} did not complete within {timeout}s")
    
    # Listeners
    
    def get_listeners(self) -> List[Listener]:
        """Get all listeners."""
        response = self._get('/api/listeners')
        return [Listener.from_dict(l) for l in response.json()]
    
    def create_listener(self, name: str, protocol: str, host: str, 
                       port: int, profile: Optional[str] = None) -> Listener:
        """Create a new listener."""
        data = {
            'name': name,
            'protocol': protocol,
            'host': host,
            'port': port,
        }
        if profile:
            data['profile'] = profile
        
        response = self._post('/api/listeners', data)
        return Listener.from_dict(response.json())
    
    # Credentials
    
    def get_credentials(self) -> List[Credential]:
        """Get all collected credentials."""
        response = self._get('/api/credentials')
        return [Credential.from_dict(c) for c in response.json()]
    
    # Audit
    
    def get_audit_logs(self, user: Optional[str] = None, 
                       action: Optional[str] = None) -> List[AuditLog]:
        """Get audit logs."""
        params = {}
        if user:
            params['user'] = user
        if action:
            params['action'] = action
        
        response = self._get('/api/audit', params=params)
        return [AuditLog.from_dict(a) for a in response.json()]
    
    # Reports
    
    def generate_report(self, start_date: datetime, end_date: datetime,
                       format: str = 'html', 
                       sections: Optional[List[str]] = None) -> Report:
        """
        Generate a report.
        
        Args:
            start_date: Report start date
            end_date: Report end date
            format: Report format ('html', 'json', 'markdown')
            sections: Report sections to include
        
        Returns:
            Report object with download URL
        """
        data = {
            'date_range': {
                'start': start_date.isoformat(),
                'end': end_date.isoformat(),
            },
            'format': format,
        }
        
        if sections:
            data['sections'] = sections
        
        response = self._post('/api/report/generate', data)
        return Report.from_dict(response.json())
    
    # Utility methods
    
    def is_authenticated(self) -> bool:
        """Check if client is authenticated."""
        return self.token is not None
    
    def get_current_user(self) -> Optional[User]:
        """Get current authenticated user."""
        if not self.token:
            return None
        
        try:
            response = self._get('/api/auth/me')
            return User.from_dict(response.json())
        except:
            return None
