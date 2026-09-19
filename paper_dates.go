package arxiv

import (
	"strconv"
	"strings"
	"time"
)

// DatesNeedReview detects missing or internally inconsistent cached dates. This
// is a plausibility check, not verification of a paper's submission history.
// arXiv identifiers encode an announcement month, never an exact submission day.
func (p Paper) DatesNeedReview() bool {
	if p.Created.IsZero() || (!p.Updated.IsZero() && p.Updated.Before(p.Created)) {
		return true
	}
	id := p.ID
	if slash := strings.LastIndexByte(id, '/'); slash >= 0 {
		id = id[slash+1:]
	}
	if len(id) < 4 {
		return false
	}
	year, yerr := strconv.Atoi(id[:2])
	month, merr := strconv.Atoi(id[2:4])
	if yerr != nil || merr != nil || month < 1 || month > 12 {
		return false
	}
	if year >= 91 {
		year += 1900
	} else {
		year += 2000
	}
	announcement := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	// Allow month-boundary submissions; a years-later revision is not v1.
	return p.Created.Before(announcement.AddDate(0, -1, 0)) || !p.Created.Before(announcement.AddDate(0, 2, 0))
}
