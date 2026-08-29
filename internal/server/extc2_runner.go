package server

// extC2Runner is the minimal handle the delete/shutdown paths need for a live
// external C2 poller (Discord/Slack). Stop() is idempotent in both impls.
type extC2Runner interface {
	Stop()
}

// registerExtC2Runner tracks a started channel poller under the same key the
// metadata map uses ("extc2-<type>-<channelID>").
func (s *Server) registerExtC2Runner(key string, r extC2Runner) {
	s.extC2ChannelsMu.Lock()
	s.extC2Runners[key] = r
	s.extC2ChannelsMu.Unlock()
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
