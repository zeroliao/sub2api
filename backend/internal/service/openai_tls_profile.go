package service

import "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"

const defaultOpenAIOAuthTLSProfileName = "Built-in Default (Node.js 24.x)"

func ensureOpenAIOAuthTLSProfile(account *Account, profile *tlsfingerprint.Profile) *tlsfingerprint.Profile {
	if profile != nil {
		return profile
	}
	if account == nil || account.Platform != PlatformOpenAI || !account.IsOAuth() {
		return nil
	}
	return &tlsfingerprint.Profile{Name: defaultOpenAIOAuthTLSProfileName}
}
