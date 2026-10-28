package domain

import "time"

type Options struct{
	GuestSessionTTL time.Duration
	AccountSessionTTL time.Duration
}
