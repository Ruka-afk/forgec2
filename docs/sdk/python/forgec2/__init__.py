"""
ForgeC2 Python SDK
==================

A comprehensive Python client library for the ForgeC2 Command & Control API.

Installation:
    pip install forgec2

Quick Start:
    from forgec2 import ForgeC2Client
    
    client = ForgeC2Client('http://localhost:8080')
    client.login('admin', 'admin')
    
    agents = client.get_agents()
    for agent in agents:
        print(f"Agent {agent.hostname} is {agent.status}")

Advanced Usage:
    # Use context manager for automatic cleanup
    with ForgeC2Client('http://localhost:8080') as client:
        client.login('admin', 'admin')
        
        # Execute shell command
        task = client.execute_shell(agent_id, 'whoami')
        result = client.wait_for_task(task.id)
        print(result)
        
        # Take screenshot
        screenshot = client.take_screenshot(agent_id)
        client.download_file(screenshot.path, 'screenshot.png')
"""

__version__ = '2.0.0'
__author__ = 'ForgeC2 Team'

from .client import ForgeC2Client, ForgeC2Error
from .models import Agent, Task, Listener, Credential, AuditLog, User

__all__ = [
    'ForgeC2Client',
    'ForgeC2Error',
    'Agent',
    'Task',
    'Listener',
    'Credential',
    'AuditLog',
    'User',
]
