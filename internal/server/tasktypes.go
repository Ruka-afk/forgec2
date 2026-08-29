package server

import (
	"net/http"

	"github.com/forgec2/forgec2/pkg/protocol"
	"github.com/gin-gonic/gin"
)

// TaskTypeParam describes a single parameter for a task type.
// Alias of the shared protocol type: declarations live in the pkg/protocol
// spec registry (taskspec_data.go), not here.
type TaskTypeParam = protocol.TaskParam

// TaskTypeInfo describes a task type registered on the server.
type TaskTypeInfo struct {
	Type          string `json:"type"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	Category      string `json:"category,omitempty"`
	RequiresShell bool   `json:"requires_shell,omitempty"`
	RequiresElev  bool   `json:"requires_elevation,omitempty"`
	// RequiresApproval marks high-impact / irreversible operations. When
	// security.require_approval is enabled such tasks are created with
	// status pending_approval and need a SECOND operator (different from
	// the creator) to approve them before the beacon can claim them.
	RequiresApproval bool            `json:"requires_approval,omitempty"`
	Parameters       []TaskTypeParam `json:"parameters,omitempty"`
}

var registeredTaskTypes []TaskTypeInfo

// registeredTaskTypes is DERIVED from the single-point spec registry in
// pkg/protocol. To add or change a command, declare a TaskSpec there —
// this view, the metadata API, agent aliases and help all follow
// automatically.
func init() {
	registeredTaskTypes = make([]TaskTypeInfo, 0, len(protocol.AllSpecs()))
	for _, sp := range protocol.AllSpecs() {
		info := taskSpecToInfo(sp)
		if dangerousTaskTypes[info.Type] {
			info.RequiresApproval = true
		}
		registeredTaskTypes = append(registeredTaskTypes, info)
	}
}

func taskSpecToInfo(sp protocol.TaskSpec) TaskTypeInfo {
	params := make([]TaskTypeParam, len(sp.Parameters))
	for i, p := range sp.Parameters {
		params[i] = p
	}
	return TaskTypeInfo{
		Type:             sp.Type,
		Name:             sp.Name,
		Description:      sp.Description,
		Category:         sp.Category,
		RequiresShell:    sp.RequiresShell,
		RequiresElev:     sp.RequiresElev,
		RequiresApproval: sp.RequiresApproval,
		Parameters:       params,
	}
}

// dangerousTaskTypes are operations that are irreversible or high-impact:
// self-destruction, malware manipulation of the host, credential theft from
// the target environment, persistence, lateral movement, and ticket forgery.
var dangerousTaskTypes = map[string]bool{
	protocol.TaskTypeUninstall:        true,
	protocol.TaskTypeKill:             true,
	protocol.TaskTypeSelfDelete:       true,
	protocol.TaskTypeMigrate:          true,
	protocol.TaskTypeKillAV:           true,
	protocol.TaskTypeLogWipe:          true,
	protocol.TaskTypeTrackWipe:        true,
	protocol.TaskTypeDCSync:           true,
	protocol.TaskTypeDCSyncMachine:    true,
	protocol.TaskTypeGoldenTicket:     true,
	protocol.TaskTypeSilverTicket:     true,
	protocol.TaskTypeShadowCreds:      true,
	protocol.TaskTypeInject:           true,
	protocol.TaskTypeShinject:         true,
	protocol.TaskTypeShspawn:          true,
	protocol.TaskTypeReflectDLLInject: true,
	protocol.TaskTypeLateral:          true,
	protocol.TaskTypeLateralWMI:       true,
	protocol.TaskTypeLateralWinRM:     true,
	protocol.TaskTypeLateralPsexec:    true,
	protocol.TaskTypeLateralDCOM:      true,
	protocol.TaskTypeSSHLateral:       true,
	protocol.TaskTypePassTheHash:      true,
	protocol.TaskTypePassTheTicket:    true,
	protocol.TaskTypePersistenceAdd:   true,
	protocol.TaskTypeContainerEscape:  true,
	protocol.TaskTypeBrowserSteal:     true,
	protocol.TaskTypeCloudSteal:       true,
	protocol.TaskTypeCoercePrinterBug: true,
	protocol.TaskTypeCoercePetitPotam: true,
	protocol.TaskTypeCoerceDFS:        true,
	protocol.TaskTypeRelayNTLMStart:   true,
	protocol.TaskTypeConstrainedDeleg: true,
	protocol.TaskTypeRBCD:             true,
	protocol.TaskTypeBronzeBit:        true,
	protocol.TaskTypeAdminSDHolder:    true,
	protocol.TaskTypeADCSESC1:         true,
	protocol.TaskTypeADCSESC2:         true,
	protocol.TaskTypeADCSESC3:         true,
	protocol.TaskTypeADCSESC4:         true,
	protocol.TaskTypeADCSESC5:         true,
	protocol.TaskTypeADCSESC6:         true,
	protocol.TaskTypeADCSESC7:         true,
	protocol.TaskTypeADCSESC8:         true,

	// Credential-access operations that were previously missing from the
	// dangerous list: dumping creds, Mimikatz, Kerberoast, and all DPAPI
	// decryptions are high-impact and must inherit the two-man rule.
	protocol.TaskTypeCreds:          true,
	protocol.TaskTypeMimikatz:       true,
	protocol.TaskTypeKerberoast:     true,
	protocol.TaskTypePasswordSpray:  true,
	protocol.TaskTypeDPAPIMasterKey: true,
	protocol.TaskTypeDPAPIBlob:      true,
	protocol.TaskTypeDPAPIBrowser:   true,
	protocol.TaskTypeCookieExport:   true,
	protocol.TaskTypeChromeCookies:  true,
	protocol.TaskTypeUSBDrop:        true,

	// Destructive / irreversible filesystem operations: recursive delete must
	// be gated so a single operator cannot wipe host data without a second
	// approval (S4: "delete recursive no confirm").
	protocol.TaskTypeDelete: true,
}

// GetRegisteredTaskTypes returns a copy of the registered task type list.
func GetRegisteredTaskTypes() []TaskTypeInfo {
	out := make([]TaskTypeInfo, len(registeredTaskTypes))
	copy(out, registeredTaskTypes)
	return out
}

// IsKnownTaskType returns true if the type exists in the registry or is
// a known internal/plugin type that bypasses normal validation.
func IsKnownTaskType(t string) bool {
	for _, info := range registeredTaskTypes {
		if info.Type == t {
			return true
		}
	}
	return false
}

// getTaskTypeInfo returns the TaskTypeInfo for a given type string.
// Returns the info and true if found, zero value and false otherwise.
func getTaskTypeInfo(t string) (TaskTypeInfo, bool) {
	for _, info := range registeredTaskTypes {
		if info.Type == t {
			return info, true
		}
	}
	return TaskTypeInfo{}, false
}

// apiListTaskTypes returns the full task type registry for frontend consumption.
func (s *Server) apiListTaskTypes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": GetRegisteredTaskTypes()})
}
