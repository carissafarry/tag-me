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

func addRepoFilter(query *string, args *[]any, fieldName string, value string, paramCount *int) {
	if value != "" {
		*query += fmt.Sprintf(" AND %s = $%d", fieldName, *paramCount)
		*args = append(*args, value)
		(*paramCount)++
	}
}
