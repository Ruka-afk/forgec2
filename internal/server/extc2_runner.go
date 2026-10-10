package server

// extC2Runner is the minimal handle the delete/shutdown paths need for a live
// external C2 poller (Discord/Slack). Stop() is idempotent in both impls.
type extC2Runner interface {
	Stop()
}

// registerExtC2Runner tracks a started channel poller under the same key the
// metadata map uses ("extc2-<type>-<channelID>"). A runner already living on
// that key is stopped first: reconfiguring a channel used to orphan its
// poller, which kept reconnecting with the stale token until restart.
func (s *Server) registerExtC2Runner(key string, r extC2Runner) {
	s.extC2ChannelsMu.Lock()
	prev, hadPrev := s.extC2Runners[key]
	s.extC2Runners[key] = r
	s.extC2ChannelsMu.Unlock()
	// Stop outside the lock: Stop can block on poller teardown. A runner
	// re-registering itself (restore on boot) is skipped.
	if hadPrev && prev != nil && prev != r {
		prev.Stop()
	}
}

// stopExtC2Runner stops and forgets the poller for key; returns whether one
// existed.
func (s *Server) stopExtC2Runner(key string) bool {
	s.extC2ChannelsMu.Lock()
	r, ok := s.extC2Runners[key]
	delete(s.extC2Runners, key)
	s.extC2ChannelsMu.Unlock()
	if ok && r != nil {
		r.Stop()
	}
	return ok
}
