/*
Sniperkit-Bot
- Date: 2018-08-12 11:57:50.86147846 +0200 CEST m=+0.186676333
- Status: analyzed
*/

package models

type EventStatus string

const (
	EventStatusNew       = EventStatus("new")
	EventStatusProcessed = EventStatus("processed")
	EventStatusFailed    = EventStatus("failed")
)
