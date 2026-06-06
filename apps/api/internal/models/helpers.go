package models

import "time"

var jakartaLocation = func() *time.Location {
	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("GMT+7", 7*60*60)
	}
	return location
}()

func formatJakartaTime(value time.Time) string {
	return value.In(jakartaLocation).Format("2006-01-02 15:04:05")
}

func formatJakartaTimePtr(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatJakartaTime(*value)
}