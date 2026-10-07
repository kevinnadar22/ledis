package utils

import "time"

func CalculateExpireAt(expiry_in_seconds int64) int64 {
	return time.Now().Add(time.Duration(expiry_in_seconds) * time.Second).UnixMilli()
}

func IsExpired(exp int64) bool {
	return time.Now().UnixMilli() >= exp
}

func RemainingTTL(exp int64) int64 {

	if exp < 0 {
		return -1
	}
	return (exp - time.Now().UnixMilli()) / 1000
}
