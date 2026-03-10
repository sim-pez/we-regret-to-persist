package entity

import "time"

type Email struct {
	From    string
	Subject string
	Date    time.Time
	Text    string
}
