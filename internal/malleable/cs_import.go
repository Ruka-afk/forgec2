package malleable

import (
	"fmt"
)

// ParseCSFull parses a full Cobalt Strike .profile text into v2. It is the
// compatibility wrapper around ParseCSProfile used by import/build callers
// that only need an error/no-error result.
func ParseCSFull(name, text string) (*ProfileV2, error) {
	res := ParseCSProfile(name, text)
	if res.Profile == nil {
		return nil, fmt.Errorf("CS profile could not be parsed")
	}
	if res.HasErrors() {
		msgs := res.ErrorMessages()
		if len(msgs) > 0 {
			return res.Profile, fmt.Errorf("CS profile has %d error(s): %s", len(msgs), msgs[0])
		}
		return res.Profile, fmt.Errorf("CS profile has syntax errors")
	}
	return res.Profile, nil
}
