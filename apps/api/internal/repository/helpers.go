package repository

import (
	"fmt"
	"strconv"
)

const (
	stateCountField      = "count"
	stateLastSentAtField = "last_sent_at"
)

func parseOptionalUnix(value interface{}) (int64, error) {
	text := fmt.Sprint(value)
	if text == "" || text == "0" {
		return 0, nil
	}

	return strconv.ParseInt(text, 10, 64)
}
