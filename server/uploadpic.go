package server

import (
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"time"

	"github.com/hoshinonyaruko/gensokyo-kook/config"
)

const (
	MaximumImageSize        = 10 * 1024 * 1024
	AllowedUploadsPerMinute = 100
	RequestInterval         = time.Minute
)

type RateLimiter struct {
	Counts map[string][]time.Time
}

// 频率限制器
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		Counts: make(map[string][]time.Time),
	}
}

// 检查是否超过调用频率限制
func (rl *RateLimiter) CheckAndUpdateRateLimit(ipAddress string) bool {
	// 获取 MaxRequests 的当前值
	maxRequests := config.GetImageLimitB()

	now := time.Now()
	rl.Counts[ipAddress] = append(rl.Counts[ipAddress], now)

	// Remove expired entries
	for len(rl.Counts[ipAddress]) > 0 && now.Sub(rl.Counts[ipAddress][0]) > RequestInterval {
		rl.Counts[ipAddress] = rl.Counts[ipAddress][1:]
	}

	return len(rl.Counts[ipAddress]) <= maxRequests
}
