package service

import "time"

// Settings are the business-rule knobs the service needs.
type Settings struct {
	RefreshTokenTTL   time.Duration
	MaxFailedAttempts int           // consecutive failures before a lock
	BaseLockout       time.Duration // first lock duration; doubles on each further lock
	MaxLockout        time.Duration // upper bound for the lock duration
}
