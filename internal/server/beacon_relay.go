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
			// Chain integrity for relayed file transfers: a compromised
			// parent forwarding a child's exfil must present the child's
			// chain link (keyed per-child, which the parent never holds), or
			// the forged chunk is dropped before it reaches disk/parsers.
			// Same empty-MAC legacy tolerance as the direct path.
			if r.Type == "upload" || r.Type == "download" {
				chunk := []byte(r.Output)
				if r.Encoding == "base64" && r.Output != "" {
					if decoded, err := base64.StdEncoding.DecodeString(r.Output); err == nil {
						chunk = decoded
					}
				}
				if err := s.verifyAndCommitChain(rd.AgentID, task.ID, r.MAC, chunk); err != nil {
					slog.Warn("Relayed file chunk integrity failure", "child", rd.AgentID, "task_id", r.TaskID, "error", err)
					continue
				}
			}
			if len(task.Result) > MaxResultSize {
				task.Result = truncateString(task.Result, MaxResultSize)
			}
			// Keep the task object plaintext for operator-facing WebSocket
			// broadcasts. Encrypt only a copy destined for storage; mutating task
			// here leaked FC2ENC ciphertext into interactive shell output.
			dbTask := *task
			if err := s.encryptTaskFieldsOrBlank(&dbTask); err != nil {
				task.Status = "failed"
			}
			// Atomic first-final-wins: only pending/running/sent can transition to final
			res := s.withBusyRetryDB("result", func() *gorm.DB {
				return s.db.Model(&db.Task{}).Where("id = ? AND status IN ?", task.ID, []string{"pending", "running", "sent"}).Updates(map[string]interface{}{
					"status": task.Status, "result": dbTask.Result, "error": dbTask.Error, "last_result_id": task.LastResultID,
				})
			})
			if res.Error != nil {
				slog.Error("Failed to save relayed task result", "task_id", task.ID, "child", rd.AgentID, "error", res.Error)
				continue
			}
			if res.RowsAffected == 0 {
				continue
			}
			s.decrementPendingTasks(rd.AgentID)
			if s.metrics != nil {
				s.metrics.TaskExecuteDuration.WithLabelValues(task.Type).Observe(now.Sub(task.CreatedAt).Seconds())
			}
			s.fileChains.reset(task.ID)
			s.broadcastTaskUpdate(rd.AgentID, *task)
			slog.Info("P2P relayed task result processed", "child", rd.AgentID, "task_id", r.TaskID)
		}
		slog.Info("P2P relayed data processed for child", "parent", parentUUID, "child", rd.AgentID)
	}

	// last_seen advances only for verified children (present in childAgentMap
	// with this parent binding). Unverified IDs must not move presence: a
	// forged childID would otherwise paint topology/online status.
	verifiedIDs := make([]string, 0, len(childAgentMap))
	for id := range childAgentMap {
		verifiedIDs = append(verifiedIDs, id)
	}
	if len(verifiedIDs) > 0 {
		if err := s.withBusyRetryDB("enroll", func() *gorm.DB {
			return s.db.Model(&db.Implant{}).Where("id IN ?", verifiedIDs).Update("last_seen", now)
		}).Error; err != nil {
			slog.Error("Failed to batch-update child agent last_seen", "parent", parentUUID, "error", err)
		}
	}
}

// maxRelayEnvelopeSize caps a single relayed child envelope (JSON with a
// base64 ciphertext payload; a decrypted request is bounded separately by
// Crypto.MaxDecryptedPayloadSize).
const maxRelayEnvelopeSize = 16 << 20 // 16 MiB

// maxRelayDepth bounds P2P envelope relay nesting depth. Nested frames are
// flattened per beacon round (a child's own RelayedFrames are clipped before
// its request is processed), so legitimate traffic never nests; the bound is
// defense-in-depth against a future path reintroducing recursion.
const maxRelayDepth = 4

// processRelayedEnvelopes handles opaque child envelopes forwarded by a P2P
// parent. Each envelope is authenticated against the child's own session key
// (never derived from the parent), so a compromised parent cannot read,
// forge, or alter a child's traffic — it is a transparent byte relay only.
// Valid frames are processed exactly like a direct beacon (registration,
// handshake and encrypted frames all work), and the child's response
// envelope is returned to the parent for verbatim forwarding.
//
// depth is the explicit nesting level (0 for a direct beacon's frames):
// the old process-global counter misfired under concurrency (N simultaneous
// parent beacons read as depth N and dropped legitimate relays), while true
// nesting is flattened per round and can never legitimately exceed 1.
func (s *Server) processRelayedEnvelopes(frames []relayedFrame, parentUUID, publicIP string, now time.Time, depth int, frameSize, budget int) []relayedReply {
	if len(frames) == 0 {
		return nil
	}

	if depth > maxRelayDepth {
		slog.Warn("P2P relay depth exceeded, dropping envelope relay", "parent", parentUUID, "depth", depth)
		s.LogAuditRecord(nil, "relay_depth_exceeded", "agent", parentUUID, "dropping P2P envelope relay batch", true, nil)
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
			// Embed the resync as this child's reply so relayed children
			// recover exactly like direct ones.
			if body, ok := s.resyncResponseFor(childEnv); ok {
				replies = append(replies, relayedReply{AgentID: rf.AgentID, Envelope: body})
			} else {
				slog.Warn("P2P relay dropped: child frame rejected", "parent", parentUUID, "child", rf.AgentID)
			}
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
		resp := s.processBeaconWithBudget(childReq, publicIP, frameSize, budget)
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

// reapOrphanedRelayChildren unbinds children of long-dead parents and
// requeues their unacked tasks claimed by the dead parent. Without this a
// parent outage starves its children: fetchRelayedChildTasks only serves the
// bound parent, while the tasks sit running/claimed forever. A re-beaconing
// parent re-binds children lazily via bindRelayChildToParent, so an
// intervening unbind is self-healing.
func (s *Server) reapOrphanedRelayChildren() {
	var parents []db.Implant
	if err := s.db.Where("id IN (?)", s.db.Model(&db.Implant{}).Select("DISTINCT parent_id").Where("parent_id <> ''")).Find(&parents).Error; err != nil {
		slog.Error("Relay orphan sweep: parent lookup failed", "err", err)
		return
	}
	now := time.Now()
	for _, p := range parents {
		// Dead = past twice its own effective offline threshold.
		deadline := p.LastSeen.Add(2 * s.offlineThresholdFor(p))
		if now.Before(deadline) {
			continue
		}
		// Unbind its children (conditional: a concurrent re-beacon that
		// flipped the parent back online does not stop this row-level
		// update, but bindRelayChildToParent re-binds on next relay).
		var childIDs []string
		if err := s.db.Model(&db.Implant{}).Where("parent_id = ?", p.ID).Pluck("id", &childIDs).Error; err != nil {
			continue
		}
		if len(childIDs) == 0 {
			continue
		}
		if err := s.db.Model(&db.Implant{}).Where("parent_id = ?", p.ID).Update("parent_id", "").Error; err != nil {
			slog.Error("Relay orphan sweep: unbind failed", "parent", p.ID, "err", err)
			continue
		}
		// Requeue unacked tasks the dead parent had claimed so they become
		// claimable again (by a new parent or direct beacon).
		res := s.db.Model(&db.Task{}).
			Where("agent_id IN ? AND status = ? AND claimed_by = ? AND acknowledged_at IS NULL", childIDs, "running", p.ID).
			Updates(map[string]interface{}{
				"status": "pending", "claimed_by": "", "claimed_at": time.Time{},
				"delivery_attempts": gorm.Expr("delivery_attempts + 1"),
			})
		if res.Error != nil {
			slog.Error("Relay orphan sweep: requeue failed", "parent", p.ID, "err", res.Error)
			continue
		}
		slog.Info("Relay orphan sweep: unbound dead parent", "parent", p.ID,
			"children", len(childIDs), "requeued", res.RowsAffected)
		s.LogAuditRecord(nil, "relay_orphan_reap", "agent", p.ID, "unbound dead relay parent", true, nil)
	}
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
		if cerr := s.withBusyRetryDB("enroll", func() *gorm.DB {
			return s.db.Create(&row)
		}).Error; cerr != nil {
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
