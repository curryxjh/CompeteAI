package domain

import "time"

type User struct {
	Id       int64
	Email    string
	UserName string
	Password string
	Ctime    time.Time
}
