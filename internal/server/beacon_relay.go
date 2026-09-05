package server

import (
	"encoding/base64"
	"log/slog"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"gorm.io/gorm"
)

// ── P2P relay (parent-forwarded child results + opaque envelopes) ───────────

func (s *Server) processRelayedResults(relayed []relayedData, parentUUID string, now time.Time) {
	// Batch-load all tasks referenced by any relayed child
	relayTaskIDs := make([]uint, 0, len(relayed)*2)
	relayTaskIDSet := make(map[uint]struct{})
	for _, rd := range relayed {
		for _, r := range rd.Results {
			if r.TaskID > 0 {
				if _, ok := relayTaskIDSet[r.TaskID]; !ok {
					relayTaskIDs = append(relayTaskIDs, r.TaskID)
					relayTaskIDSet[r.TaskID] = struct{}{}
				}
			}
		}
	}
	var relayedTasks []db.Task
	if len(relayTaskIDs) > 0 {
		if err := s.db.Where("id IN ?", relayTaskIDs).Limit(len(relayTaskIDs)).Find(&relayedTasks).Error; err != nil {
			slog.Error("Failed to batch-load relayed tasks", "error", err)
		}
	}
	relayTaskMap := make(map[uint]*db.Task, len(relayedTasks))
	for i := range relayedTasks {
		relayTaskMap[relayedTasks[i].ID] = &relayedTasks[i]
	}

	childIDs := make([]string, 0, len(relayed))
	childAgentMap := make(map[string]*db.Implant, len(relayed))

	for _, rd := range relayed {
		childIDs = append(childIDs, rd.AgentID)
	}
	if len(childIDs) > 0 {
		var childAgents []db.Implant
		if err := s.db.Where("id IN ? AND parent_id = ?", childIDs, parentUUID).Limit(len(childIDs)).Find(&childAgents).Error; err != nil {
			slog.Error("Failed to batch-load relayed child agents", "parent", parentUUID, "error", err)
		}
		childAgentMap = make(map[string]*db.Implant, len(childAgents))
		for i := range childAgents {
			childAgentMap[childAgents[i].ID] = &childAgents[i]
		}
	}

	for _, rd := range relayed {
		if _, ok := childAgentMap[rd.AgentID]; !ok {
			slog.Warn("P2P relay from non-child agent", "parent", parentUUID, "child", rd.AgentID)
			continue
		}
		s.processTaskAcknowledgements(rd.AgentID, rd.AckTaskIDs, now)
		for _, r := range rd.Results {
			task, ok := relayTaskMap[r.TaskID]
			if !ok || !strings.EqualFold(task.AgentID, rd.AgentID) {
				continue
			}
			// Finality guard: cancelled is terminal even for relayed children
			if task.Status == "cancelled" {
				continue
			}
			// Durable idempotency: exact rid already applied once.
			if r.ResultID != "" && task.LastResultID == r.ResultID && (task.Status == "completed" || task.Status == "failed") {
				continue
			}
			task.Status = "completed"
			if r.Error != "" {
				task.Status = "failed"
				task.Error = r.Error
			}
			task.UpdatedAt = now
			if r.ResultID != "" {
				task.LastResultID = r.ResultID
			}
			if r.Encoding == "base64" && r.Output != "" {
				decoded, err := base64.StdEncoding.DecodeString(r.Output)
				if err == nil {
					task.Result = string(decoded)
				} else {
					task.Result = r.Output
				}
			} else {
				task.Result = r.Output
			}
			if len(task.Result) > MaxResultSize {
				task.Result = truncateString(task.Result, MaxResultSize)
			}
			// Keep the task object plaintext for operator-facing WebSocket
			// broadcasts. Encrypt only a copy destined for storage; mutating task
			// here leaked FC2ENC ciphertext into interactive shell output.
			dbTask := *task
			dbTask.EncryptTaskFields()
			// Atomic first-final-wins: only pending/running/sent can transition to final
			res := s.db.Model(&db.Task{}).Where("id = ? AND status IN ?", task.ID, []string{"pending", "running", "sent"}).Updates(map[string]interface{}{
				"status": task.Status, "result": dbTask.Result, "error": dbTask.Error, "last_result_id": task.LastResultID,
			})
			if res.Error != nil {
				slog.Error("Failed to save relayed task result", "task_id", task.ID, "child", rd.AgentID, "error", res.Error)
				continue
			}
			if res.RowsAffected == 0 {
				continue
			}
			s.decrementPendingTasks(rd.AgentID)
			s.fileChains.reset(task.ID)
			s.broadcastTaskUpdate(rd.AgentID, *task)
			slog.Info("P2P relayed task result processed", "child", rd.AgentID, "task_id", r.TaskID)
		}
		slog.Info("P2P relayed data processed for child", "parent", parentUUID, "child", rd.AgentID)
	}

	if len(childIDs) > 0 {
		if err := s.db.Model(&db.Implant{}).Where("id IN ?", childIDs).Update("last_seen", now).Error; err != nil {
			slog.Error("Failed to batch-update child agent last_seen", "parent", parentUUID, "error", err)
		}
	}
}

// maxRelayEnvelopeSize caps a single relayed child envelope (JSON with a
// base64 ciphertext payload; a decrypted request is bounded separately by
// Crypto.MaxDecryptedPayloadSize).
const maxRelayEnvelopeSize = 16 << 20 // 16 MiB

// maxRelayDepth bounds recursive P2P envelope relay nesting. Each parent hop
// embeds child envelopes in its own envelope; an unbounded chain would let
// frames recurse indefinitely server-side.
const maxRelayDepth = 4

// processRelayedEnvelopes handles opaque child envelopes forwarded by a P2P
// parent. Each envelope is authenticated against the child's own session key
// (never derived from the parent), so a compromised parent cannot read,
// forge, or alter a child's traffic — it is a transparent byte relay only.
// Valid frames are processed exactly like a direct beacon (registration,
// handshake and encrypted frames all work), and the child's response
// envelope is returned to the parent for verbatim forwarding.
func (s *Server) processRelayedEnvelopes(frames []relayedFrame, parentUUID, publicIP string, now time.Time) []relayedReply {
	if len(frames) == 0 {
		return nil
	}

	s.relayDepthMu.Lock()
	s.relayDepth++
	depth := s.relayDepth
	s.relayDepthMu.Unlock()
	defer func() {
		s.relayDepthMu.Lock()
		s.relayDepth--
		s.relayDepthMu.Unlock()
	}()

	if depth > maxRelayDepth {
		slog.Warn("P2P relay depth exceeded, dropping envelvelope relay", "parent", parentUUID, "depth", depth)
		return nil
	}

	replies := make([]relayedReply, 0, len(frames))
	for _, rf := range frames {
		if rf.AgentID == "" || !isValidAgentID(rf.AgentID) {
			slog.Warn("P2P relay dropped: invalid child agent id", "parent", parentUUID)
			continue
		}
		if len(rf.Envelope) == 0 || len(rf.Envelope) > maxRelayEnvelopeSize {
			slog.Warn("P2P relay dropped: bad envelope size", "parent", parentUUID, "child", rf.AgentID, "size", len(rf.Envelope))
			continue
		}

		childEnv, childReq, kind := s.decodeBeaconEnvelope(rf.Envelope)
		if kind == frameRejected {
			slog.Warn("P2P relay dropped: child frame rejected", "parent", parentUUID, "child", rf.AgentID)
			continue
		}

		// Enforce the parent-child relationship. A child may only be relayed
		// by its registered parent (the operator links children in the
		// topology view); an unbound child is bound on first relay so that
		// register/handshake frames can bootstrap through the parent.
		if !s.bindRelayChildToParent(rf.AgentID, parentUUID) {
			slog.Warn("P2P relay dropped: child not linked to this parent", "parent", parentUUID, "child", rf.AgentID)
			continue
		}

		var replyBytes []byte
		var ok bool
		if kind == frameEncrypted {
			// Clip the child's nested relay fields: children relaying in turn
			// is handled by the next beacon round, not recursive inlining.
			childReq.RelayedFrames = nil
			resp := s.processBeacon(childReq, publicIP)
			if s.sessionManager != nil && s.sessionManager.NeedsRekey(rf.AgentID, BeaconSessionRekeyMessages) {
				resp.Rekey = true
			}
			replyBytes, ok = s.buildBeaconResponse(rf.AgentID, childEnv.Seq, resp)
		} else {
			replyBytes, ok = s.processAuthFrame(childEnv, kind)
		}
		if !ok {
			slog.Warn("P2P relay failed to build child response", "parent", parentUUID, "child", rf.AgentID)
			continue
		}
		replies = append(replies, relayedReply{AgentID: rf.AgentID, Envelope: replyBytes})
	}
	return replies
}

// bindRelayChildToParent enforces/lazily binds the parent-child link for a
// relayed child. A child with no DB row (fresh registration through the
// parent) is bound to the relaying parent; a child already linked to a
// different parent is rejected (protects against relay hijacking).
func (s *Server) bindRelayChildToParent(childID, parentUUID string) bool {
	var agent db.Implant
	// Unscoped so a soft-deleted child row is still found and restored rather
	// than causing a primary-key conflict on Create.
	err := s.db.Unscoped().Where("id = ?", childID).First(&agent).Error
	if err == gorm.ErrRecordNotFound {
		row := db.Implant{ID: childID, ParentID: parentUUID, TenantID: s.defaultTenantID(), LastSeen: time.Now(), Status: "online"}
		if cerr := s.db.Create(&row).Error; cerr != nil {
			// Concurrent create raced: re-check the winner's parent binding.
			if rerr := s.db.Unscoped().Where("id = ?", childID).First(&agent).Error; rerr != nil {
				slog.Debug("bindRelayChildToParent create raced", "child", childID, "error", cerr)
				return false
			}
			if agent.ParentID != "" && agent.ParentID != parentUUID {
				return false
			}
			if agent.ParentID == "" {
				res := s.db.Unscoped().Model(&db.Implant{}).Where("id = ? AND (parent_id = '' OR parent_id IS NULL)", childID).Update("parent_id", parentUUID)
				return res.Error == nil && res.RowsAffected == 1
			}
			return true
		}
		return true
	}
	if err != nil {
		slog.Error("Failed to check relay child link", "child", childID, "error", err)
		return false
	}
	if agent.DeletedAt.Valid {
		s.db.Unscoped().Model(&db.Implant{}).Where("id = ?", childID).Update("deleted_at", nil)
	}
	if agent.ParentID == "" {
		res := s.db.Unscoped().Model(&db.Implant{}).Where("id = ? AND (parent_id = '' OR parent_id IS NULL)", childID).Update("parent_id", parentUUID)
		return res.Error == nil && res.RowsAffected == 1
	}
	return agent.ParentID == parentUUID
}
