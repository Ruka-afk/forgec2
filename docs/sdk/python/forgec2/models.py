"""
ForgeC2 SDK - Data Models
=========================

Data models representing ForgeC2 API resources.
"""

from dataclasses import dataclass, field
from typing import Optional, List
from datetime import datetime


@dataclass
class User:
    """User model representing an operator account."""
    id: int
    username: str
    role: str  # admin, operator, viewer
    is_active: bool = True
    created_at: Optional[datetime] = None
    
    @classmethod
    def from_dict(cls, data: dict) -> 'User':
        """Create User from dictionary."""
        return cls(
            id=data.get('id'),
            username=data.get('username'),
            role=data.get('role'),
            is_active=data.get('is_active', True),
            created_at=datetime.fromisoformat(data.get('created_at', '').replace('Z', '+00:00'))
                if data.get('created_at') else None
        )


@dataclass
class Agent:
    """Agent model representing a connected implant."""
    id: str
    hostname: str
    username: str
    os: str
    ip: str
    status: str = 'offline'
    arch: Optional[str] = None
    last_seen: Optional[datetime] = None
    listener_id: Optional[int] = None
    parent_id: Optional[str] = None
    p2p_mode: Optional[str] = None
    version: Optional[str] = None
    pid: Optional[int] = None
    process_name: Optional[str] = None
    integrity: Optional[str] = None
    elevated: bool = False
    domain: Optional[str] = None
    current_interval: int = 5
    current_jitter: int = 0
    created_at: Optional[datetime] = None
    updated_at: Optional[datetime] = None
    
    @classmethod
    def from_dict(cls, data: dict) -> 'Agent':
        """Create Agent from dictionary."""
        def parse_datetime(dt_str):
            if not dt_str:
                return None
            try:
                return datetime.fromisoformat(dt_str.replace('Z', '+00:00'))
            except:
                return None
        
        return cls(
            id=data.get('id'),
            hostname=data.get('hostname', ''),
            username=data.get('username', ''),
            os=data.get('os', ''),
            ip=data.get('ip', ''),
            status=data.get('status', 'offline'),
            arch=data.get('arch'),
            last_seen=parse_datetime(data.get('last_seen')),
            listener_id=data.get('listener_id'),
            parent_id=data.get('parent_id'),
            p2p_mode=data.get('p2p_mode'),
            version=data.get('version'),
            pid=data.get('pid'),
            process_name=data.get('process_name'),
            integrity=data.get('integrity'),
            elevated=data.get('elevated', False),
            domain=data.get('domain'),
            current_interval=data.get('current_interval', 5),
            current_jitter=data.get('current_jitter', 0),
            created_at=parse_datetime(data.get('created_at')),
            updated_at=parse_datetime(data.get('updated_at')),
        )
    
    def is_online(self) -> bool:
        """Check if agent is online."""
        return self.status == 'online'
    
    def is_admin(self) -> bool:
        """Check if agent is running with elevated privileges."""
        return self.elevated or self.integrity in ['High', 'System']


@dataclass
class Task:
    """Task model representing a command sent to an agent."""
    id: int
    agent_id: str
    type: str
    command: Optional[str] = None
    shell: Optional[str] = None
    path: Optional[str] = None
    data: Optional[str] = None
    status: str = 'pending'
    result: Optional[str] = None
    created_at: Optional[datetime] = None
    updated_at: Optional[datetime] = None
    
    @classmethod
    def from_dict(cls, data: dict) -> 'Task':
        """Create Task from dictionary."""
        def parse_datetime(dt_str):
            if not dt_str:
                return None
            try:
                return datetime.fromisoformat(dt_str.replace('Z', '+00:00'))
            except:
                return None
        
        return cls(
            id=data.get('id'),
            agent_id=data.get('agent_id', ''),
            type=data.get('type', ''),
            command=data.get('command'),
            shell=data.get('shell'),
            path=data.get('path'),
            data=data.get('data'),
            status=data.get('status', 'pending'),
            result=data.get('result'),
            created_at=parse_datetime(data.get('created_at')),
            updated_at=parse_datetime(data.get('updated_at')),
        )
    
    def is_pending(self) -> bool:
        """Check if task is pending."""
        return self.status == 'pending'
    
    def is_completed(self) -> bool:
        """Check if task is completed."""
        return self.status == 'completed'
    
    def is_failed(self) -> bool:
        """Check if task failed."""
        return self.status == 'failed'


@dataclass
class Listener:
    """Listener model representing a C2 listener configuration."""
    id: int
    name: str
    protocol: str  # http, https, ws, wss
    host: str
    port: int
    profile: Optional[str] = None
    status: str = 'inactive'
    created_at: Optional[datetime] = None
    
    @classmethod
    def from_dict(cls, data: dict) -> 'Listener':
        """Create Listener from dictionary."""
        return cls(
            id=data.get('id'),
            name=data.get('name', ''),
            protocol=data.get('protocol', 'http'),
            host=data.get('host', ''),
            port=data.get('port'),
            profile=data.get('profile'),
            status=data.get('status', 'inactive'),
            created_at=datetime.fromisoformat(data.get('created_at', '').replace('Z', '+00:00'))
                if data.get('created_at') else None
        )
    
    def is_active(self) -> bool:
        """Check if listener is active."""
        return self.status == 'active'
    
    def get_url(self) -> str:
        """Get listener URL."""
        return f"{self.protocol}://{self.host}:{self.port}"


@dataclass
class Credential:
    """Credential model representing collected credentials."""
    id: int
    agent_id: str
    source: str
    username: str
    password: Optional[str] = None
    hash: Optional[str] = None
    created_at: Optional[datetime] = None
    
    @classmethod
    def from_dict(cls, data: dict) -> 'Credential':
        """Create Credential from dictionary."""
        return cls(
            id=data.get('id'),
            agent_id=data.get('agent_id', ''),
            source=data.get('source', ''),
            username=data.get('username', ''),
            password=data.get('password'),
            hash=data.get('hash'),
            created_at=datetime.fromisoformat(data.get('created_at', '').replace('Z', '+00:00'))
                if data.get('created_at') else None
        )
    
    def has_password(self) -> bool:
        """Check if credential has plaintext password."""
        return bool(self.password)
    
    def has_hash(self) -> bool:
        """Check if credential has hash."""
        return bool(self.hash)


@dataclass
class AuditLog:
    """Audit log entry."""
    id: int
    user: str
    action: str
    details: Optional[str] = None
    ip_address: Optional[str] = None
    created_at: Optional[datetime] = None
    
    @classmethod
    def from_dict(cls, data: dict) -> 'AuditLog':
        """Create AuditLog from dictionary."""
        return cls(
            id=data.get('id'),
            user=data.get('user', ''),
            action=data.get('action', ''),
            details=data.get('details'),
            ip_address=data.get('ip_address'),
            created_at=datetime.fromisoformat(data.get('created_at', '').replace('Z', '+00:00'))
                if data.get('created_at') else None
        )


@dataclass
class Report:
    """Report generation result."""
    download_url: str
    format: str  # html, json, markdown
    generated_at: datetime = field(default_factory=datetime.now)
    
    @classmethod
    def from_dict(cls, data: dict) -> 'Report':
        """Create Report from dictionary."""
        return cls(
            download_url=data.get('download_url', ''),
            format=data.get('format', 'html'),
            generated_at=datetime.fromisoformat(data.get('generated_at', '').replace('Z', '+00:00'))
                if data.get('generated_at') else datetime.now()
        )
