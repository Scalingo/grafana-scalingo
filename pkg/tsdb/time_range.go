package tsdb

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func NewTimeRange(from, to string) *TimeRange {
	return &TimeRange{
		From: from,
		To:   to,
		Now:  time.Now(),
	}
}

type TimeRange struct {
	From string
	To   string
	Now  time.Time
}

func (tr *TimeRange) GetFromAsMsEpoch() int64 {
	return tr.MustGetFrom().UnixNano() / int64(time.Millisecond)
}

func (tr *TimeRange) GetToAsMsEpoch() int64 {
	return tr.MustGetTo().UnixNano() / int64(time.Millisecond)
}

func (tr *TimeRange) MustGetFrom() time.Time {
	if res, err := tr.ParseFrom(); err != nil {
		return time.Unix(0, 0)
	} else {
		return res
	}
}

func (tr *TimeRange) MustGetTo() time.Time {
	if res, err := tr.ParseTo(); err != nil {
		return time.Unix(0, 0)
	} else {
		return res
	}
}

func tryParseUnixMsEpoch(val string) (time.Time, bool) {
	if val, err := strconv.ParseInt(val, 10, 64); err == nil {
		seconds := val / 1000
		nano := (val - seconds*1000) * 1000000
		return time.Unix(seconds, nano), true
	}
	return time.Time{}, false
}

func (tr *TimeRange) ParseFrom() (time.Time, error) {
	if res, ok := tryParseUnixMsEpoch(tr.From); ok {
		return res, nil
	}

	fromRaw := strings.Replace(tr.From, "now-", "", 1)
	diff, err := time.ParseDuration("-" + fromRaw)
	if err != nil {
		return time.Time{}, err
	}

	return tr.Now.Add(diff), nil
}

func (tr *TimeRange) ParseTo() (time.Time, error) {
	if tr.To == "now" {
		return tr.Now, nil
	} else if strings.HasPrefix(tr.To, "now-") {
		withoutNow := strings.Replace(tr.To, "now-", "", 1)

		diff, err := time.ParseDuration("-" + withoutNow)
		if err != nil {
			return time.Time{}, nil
		}

		return tr.Now.Add(diff), nil
	}

	if res, ok := tryParseUnixMsEpoch(tr.To); ok {
		return res, nil
	}

	return time.Time{}, fmt.Errorf("cannot parse to value %s", tr.To)
}
