"""ForgeC2 Python Client Library

Provides a typed wrapper around the ForgeC2 REST API.

Usage:
    from forgec2 import ForgeC2Client

    client = ForgeC2Client("http://localhost:8000", "admin", "admin")
    agents = client.agents.list()
    task = client.agents.shell(agents[0]["id"], "whoami")
"""

import requests
from typing import Any, Dict, List, Optional


class ForgeC2Client:
    """Top-level client that authenticates and exposes resource APIs."""

    def __init__(self, base_url: str, username: str, password: str):
        self.base_url = base_url.rstrip("/")
        self.session = requests.Session()
        self._login(username, password)

    def _login(self, username: str, password: str):
        resp = self.session.post(
            f"{self.base_url}/login",
            data={"username": username, "password": password},
        )
        resp.raise_for_status()

    def _get(self, path: str, **kwargs) -> Dict[str, Any]:
        resp = self.session.get(f"{self.base_url}{path}", **kwargs)
        resp.raise_for_status()
        return resp.json()

    def _post(self, path: str, **kwargs) -> Dict[str, Any]:
        resp = self.session.post(f"{self.base_url}{path}", **kwargs)
        resp.raise_for_status()
        return resp.json()

    def _put(self, path: str, **kwargs) -> Dict[str, Any]:
        resp = self.session.put(f"{self.base_url}{path}", **kwargs)
        resp.raise_for_status()
        return resp.json()

    def _delete(self, path: str, **kwargs) -> Dict[str, Any]:
        resp = self.session.delete(f"{self.base_url}{path}", **kwargs)
        resp.raise_for_status()
        return resp.json()

    @property
    def agents(self) -> "AgentAPI":
        return AgentAPI(self)

    @property
    def listeners(self) -> "ListenerAPI":
        return ListenerAPI(self)

    @property
    def generate(self) -> "GenerateAPI":
        return GenerateAPI(self)

    @property
    def credentials(self) -> "CredentialAPI":
        return CredentialAPI(self)

    @property
    def workflows(self) -> "WorkflowAPI":
        return WorkflowAPI(self)


class AgentAPI:
    """Agent-related operations."""

    def __init__(self, client: ForgeC2Client):
        self.client = client

    def list(self, page: int = 1, page_size: int = 20) -> List[Dict]:
        data = self.client._get(f"/api/agents?page={page}&page_size={page_size}")
        return data.get("data", {}).get("agents", [])

    def get(self, agent_id: str) -> Dict:
        return self.client._get(f"/api/agents/{agent_id}")

    def shell(self, agent_id: str, command: str, shell: str = "cmd") -> Dict:
        return self.client._post(f"/agents/{agent_id}/command", json={
            "command": command, "shell": shell,
        })

    def kill(self, agent_id: str) -> Dict:
        return self.client._post(f"/agents/{agent_id}/kill")

    def delete(self, agent_id: str) -> Dict:
        return self.client._delete(f"/agents/{agent_id}")

    def tasks(self, agent_id: str) -> List[Dict]:
        return self.client._get(f"/agents/{agent_id}/tasks").get("tasks", [])

    def task_status(self, agent_id: str, task_id: str) -> Dict:
        return self.client._get(f"/agents/{agent_id}/tasks/{task_id}")

    def batch(self, agent_ids: List[str], command: str, shell: str = "cmd") -> Dict:
        return self.client._post("/agents/batch", json={
            "agent_ids": agent_ids, "command": command, "shell": shell,
        })


class ListenerAPI:
    """Listener management operations."""

    def __init__(self, client: ForgeC2Client):
        self.client = client

    def list(self, page: int = 1, page_size: int = 20) -> List[Dict]:
        data = self.client._get(f"/api/listeners?page={page}&page_size={page_size}")
        return data.get("data", {}).get("listeners", [])

    def create(self, name: str, host: str, port: int, scheme: str = "http", **kwargs) -> Dict:
        return self.client._post("/api/listeners", json={
            "name": name, "host": host, "port": port, "scheme": scheme, **kwargs,
        })

    def update(self, listener_id: int, **kwargs) -> Dict:
        return self.client._put(f"/api/listeners/{listener_id}", json=kwargs)

    def delete(self, listener_id: int) -> Dict:
        return self.client._delete(f"/api/listeners/{listener_id}")

    def enable(self, listener_id: int) -> Dict:
        return self.client._post(f"/api/listeners/{listener_id}/enable")

    def disable(self, listener_id: int) -> Dict:
        return self.client._post(f"/api/listeners/{listener_id}/disable")


class GenerateAPI:
    """Payload generation operations."""

    def __init__(self, client: ForgeC2Client):
        self.client = client

    def exe(self, c2_url: str, **kwargs) -> requests.Response:
        return self.client.session.post(
            f"{self.client.base_url}/generate/exe",
            data={"c2_url": c2_url, **kwargs},
        )

    def dll(self, c2_url: str, **kwargs) -> requests.Response:
        return self.client.session.post(
            f"{self.client.base_url}/generate/dll",
            data={"c2_url": c2_url, **kwargs},
        )

    def ps1(self, c2_url: str, **kwargs) -> requests.Response:
        return self.client.session.post(
            f"{self.client.base_url}/generate/ps1",
            data={"c2_url": c2_url, **kwargs},
        )

    def linux(self, c2_url: str, **kwargs) -> requests.Response:
        return self.client.session.post(
            f"{self.client.base_url}/generate/linux",
            data={"c2_url": c2_url, **kwargs},
        )

    def macos(self, c2_url: str, **kwargs) -> requests.Response:
        return self.client.session.post(
            f"{self.client.base_url}/generate/macos",
            data={"c2_url": c2_url, **kwargs},
        )

    def profiles(self) -> List[Dict]:
        return self.client._get("/api/generate/profiles").get("data", {}).get("profiles", [])


class CredentialAPI:
    """Credential management operations."""

    def __init__(self, client: ForgeC2Client):
        self.client = client

    def list(self, page: int = 1, page_size: int = 20) -> List[Dict]:
        data = self.client._get(f"/credentials?page={page}&page_size={page_size}")
        return data.get("credentials", [])

    def add(self, **kwargs) -> Dict:
        return self.client._post("/credentials/add", json=kwargs)

    def delete(self, cred_id: int) -> Dict:
        return self.client._delete(f"/credentials/{cred_id}")


class WorkflowAPI:
    """Workflow management operations."""

    def __init__(self, client: ForgeC2Client):
        self.client = client

    def list(self) -> List[Dict]:
        return self.client._get("/workflows").get("workflows", [])

    def get(self, workflow_id: str) -> Dict:
        return self.client._get(f"/workflows/{workflow_id}")

    def create(self, name: str, steps: List[Dict], description: str = "",
               scope_type: str = "all", **kwargs) -> Dict:
        return self.client._post("/workflows", json={
            "name": name, "description": description,
            "scope_type": scope_type, "steps": steps, **kwargs,
        })

    def update(self, workflow_id: str, **kwargs) -> Dict:
        return self.client._put(f"/workflows/{workflow_id}", json=kwargs)

    def delete(self, workflow_id: str) -> Dict:
        return self.client._delete(f"/workflows/{workflow_id}")

    def toggle(self, workflow_id: str) -> Dict:
        return self.client._post(f"/workflows/{workflow_id}/toggle")

    def execute(self, workflow_id: str) -> Dict:
        return self.client._post(f"/workflows/{workflow_id}/execute", json={})
