package protocol

// CurrentProtocolVersion is the supported protocol version.
// Increment when making breaking changes to the wire format.
const CurrentProtocolVersion uint = 1

// MinSupportedProtocolVersion is the oldest protocol version the server
// will accept. Agents below this threshold are rejected immediately.
const MinSupportedProtocolVersion uint = 1

// BeaconRequest is sent by agent to server on each check-in.
type BeaconRequest struct {
	UUID            string            `json:"uuid"`
	ProtocolVersion uint              `json:"pv,omitempty"`
	AgentVersion    string            `json:"av,omitempty"`
	Info            map[string]string `json:"info,omitempty"`
	Results         []TaskResult      `json:"results,omitempty"`
	AckTaskIDs      []uint            `json:"acks,omitempty"`
	TaskCapacity    *int              `json:"task_capacity,omitempty"`
	SocksData       []SocksFrame      `json:"socks_data,omitempty"`
	Relayed         []RelayedData     `json:"relayed,omitempty"`

	ECDHPub   string `json:"ecdh_pub,omitempty"`
	CipherB64 string `json:"c,omitempty"`
}

// RelayedData carries child agent results forwarded by parent (P2P).
type RelayedData struct {
	AgentID    string       `json:"agent_id"`
	Results    []TaskResult `json:"results"`
	AckTaskIDs []uint       `json:"acks,omitempty"`
}

// TaskResult is the result output from a completed agent task.
type TaskResult struct {
	TaskID   uint   `json:"task_id"`
	Type     string `json:"type"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
	Encoding string `json:"encoding,omitempty"`
	Filename string `json:"filename,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Offset   int64  `json:"offset,omitempty"`
	Path     string `json:"path,omitempty"`
}

// BeaconResponse is sent by server to agent in reply to a check-in.
type BeaconResponse struct {
	Tasks         []Task        `json:"tasks"`
	ProtocolVersion uint        `json:"pv,omitempty"`
	SocksFrames   []SocksFrame  `json:"socks_frames,omitempty"`
	SocksFastMode bool          `json:"socks_fast,omitempty"`
	Relayed       []RelayedTask `json:"relayed,omitempty"`
	ExtC2Data     []string      `json:"extc2_data,omitempty"`

	ECDHPub   string `json:"ecdh_pub,omitempty"`
	CipherB64 string `json:"c,omitempty"`
}

// RelayedTask carries tasks destined for child agents (P2P).
type RelayedTask struct {
	AgentID string `json:"agent_id"`
	Tasks   []Task `json:"tasks"`
}

// Task is a command or operation dispatched to an agent.
type Task struct {
	ID        uint   `json:"id"`
	Type      string `json:"type"`
	Command   string `json:"command"`
	Encrypted bool   `json:"enc,omitempty"`
	Shell     string `json:"shell"`
	Path      string `json:"path,omitempty"`
	Data      string `json:"data,omitempty"`
	Offset    int64  `json:"offset,omitempty"`
	Size      int64  `json:"size,omitempty"`
}

// SocksFrame carries SOCKS5 proxy relay data between agent and server.
type SocksFrame struct {
	ConnID uint64 `json:"conn_id"`
	Action string `json:"action"`
	Data   []byte `json:"data,omitempty"`
}

// UDPAssociateFrame describes a single UDP datagram relayed through the SOCKS
// tunnel. The server parses the SOCKS5 UDP request header from the operator,
// extracts (DstAddr, DstPort, Data), and forwards them to the agent via this
// struct (binary-encoded inside SocksFrame.Data). The agent sends back the
// response payload with the original destination address so the server can
// re-wrap the SOCKS5 UDP header.
type UDPAssociateFrame struct {
	ConnID  uint64 `json:"conn_id"`
	Data    []byte `json:"data,omitempty"`
	DstAddr string `json:"dst_addr,omitempty"`
	DstPort int    `json:"dst_port,omitempty"`
}
