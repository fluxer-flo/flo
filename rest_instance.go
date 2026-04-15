package flo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

type InstanceInfo struct {
	APICodeVersion int                     `json:"api_code_version"`
	Endpoints      InstanceEndpoints       `json:"endpoints"`
	Captcha        InstanceCaptchaConfig   `json:"captcha"`
	Features       InstanceFeatures        `json:"features"`
	GIF            InstanceGIFConfig       `json:"gif"`
	SSO            InstanceSSOConfig       `json:"sso"`
	Limits         InstanceLimitConfig     `json:"limits"`
	Push           InstancePushNotifConfig `json:"push"`
	AppPublic      InstanceAppPublicConfig `json:"app_public"`
}

type InstanceEndpoints struct {
	API       *url.URL
	APIClient *url.URL
	APIPublic *url.URL
	Gateway   *url.URL
	Media     *url.URL
	StaticCDN *url.URL
	Marketing *url.URL
	Admin     *url.URL
	Invite    *url.URL
	Gift      *url.URL
	WebApp    *url.URL
}

func (e *InstanceEndpoints) UnmarshalJSON(data []byte) error {
	var raw struct {
		API       string `json:"api"`
		APIClient string `json:"api_client"`
		APIPublic string `json:"api_public"`
		Gateway   string `json:"gateway"`
		Media     string `json:"media"`
		StaticCDN string `json:"static_cdn"`
		Marketing string `json:"marketing"`
		Admin     string `json:"admin"`
		Invite    string `json:"invite"`
		Gift      string `json:"gift"`
		WebApp    string `json:"webapp"`
	}
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return fmt.Errorf("failed to unmarshal raw InstanceEndpoints: %w", err)
	}

	e.API, err = url.Parse(raw.API)
	if err != nil {
		return fmt.Errorf("failed to parse InstanceEndpoints.API: %w", err)
	}

	e.APIClient, err = url.Parse(raw.APIClient)
	if err != nil {
		return fmt.Errorf("failed to parse InstanceEndpoints.APIClient: %w", err)
	}

	e.APIPublic, err = url.Parse(raw.APIPublic)
	if err != nil {
		return fmt.Errorf("failed to parse InstanceEndpoints.APIPublic: %w", err)
	}

	e.Gateway, err = url.Parse(raw.Gateway)
	if err != nil {
		return fmt.Errorf("failed to parse InstanceEndpoints.Gateway: %w", err)
	}

	e.Media, err = url.Parse(raw.Media)
	if err != nil {
		return fmt.Errorf("failed to parse InstanceEndpoints.Media: %w", err)
	}

	e.StaticCDN, err = url.Parse(raw.StaticCDN)
	if err != nil {
		return fmt.Errorf("failed to parse InstanceEndpoints.StaticCDN: %w", err)
	}

	e.Marketing, err = url.Parse(raw.Marketing)
	if err != nil {
		return fmt.Errorf("failed to parse InstanceEndpoints.Marketing: %w", err)
	}

	e.Admin, err = url.Parse(raw.Admin)
	if err != nil {
		return fmt.Errorf("failed to parse InstanceEndpoints.Admin: %w", err)
	}

	e.Invite, err = url.Parse(raw.Invite)
	if err != nil {
		return fmt.Errorf("failed to parse InstanceEndpoints.Invite: %w", err)
	}

	e.Gift, err = url.Parse(raw.Gift)
	if err != nil {
		return fmt.Errorf("failed to parse InstanceEndpoints.Gift: %w", err)
	}

	e.WebApp, err = url.Parse(raw.WebApp)
	if err != nil {
		return fmt.Errorf("failed to parse InstanceEndpoints.WebApp: %w", err)
	}

	return nil
}

type InstanceCaptchaConfig struct {
	Provider         string  `json:"provider"`
	HCaptchaSiteKey  *string `json:"hcaptcha_site_key"`
	TurnstileSiteKey *string `json:"turnstile_site_key"`
}

type InstanceFeatures struct {
	SMSMFAEnabled       bool `json:"sms_mfa_enabled"`
	VoiceEnabled        bool `json:"voice_enabled"`
	StripeEnabled       bool `json:"stripe_enabled"`
	SelfHosted          bool `json:"self_hosted"`
	ManualReviewEnabled bool `json:"manual_review_enabled"`
}

type InstanceGIFConfig struct {
	Provider string `json:"provider"`
}

type InstanceSSOConfig struct {
	Enabled     bool    `json:"enabled"`
	Enforced    bool    `json:"enforced"`
	DisplayName *string `json:"display_name"`
	RedirectURI string  `json:"redirect_uri"`
}

type InstanceLimitConfig struct {
	TraitDefinitions []string
	Rules            []InstanceLimitRule
	DefaultsHash     string
}

type InstanceLimitRule struct {
	ID        string                   `json:"id"`
	Filters   InstanceLimitRuleFilters `json:"filters"`
	Overrides map[string]int           `json:"overrides"`
}

type InstanceLimitRuleFilters struct {
	Traits        []string `json:"traits"`
	GuildFeatures []string `json:"guildFeatures"`
}

func (c *InstanceLimitConfig) UnmarshalJSON(data []byte) error {
	var raw struct {
		Version          int                 `json:"version"`
		TraitDefinitions []string            `json:"traitDefinitions"`
		Rules            []InstanceLimitRule `json:"rules"`
		DefaultsHash     string              `json:"defaultsHash"`
	}
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return fmt.Errorf("failed to unmarshal raw InstanceLimitConfig: %w", err)
	}

	if raw.Version != 2 {
		return fmt.Errorf("InstanceLimitConfig version %d not supported", raw.Version)
	}

	*c = InstanceLimitConfig{
		TraitDefinitions: raw.TraitDefinitions,
		Rules:            raw.Rules,
		DefaultsHash:     raw.DefaultsHash,
	}
	return nil
}

type InstancePushNotifConfig struct {
	PublicVAPIDKey *string `json:"public_vapid_key"`
}

type InstanceAppPublicConfig struct {
	SentryDSN *string `json:"sentry_dsn"`
}

var rateLimitInstanceInfo = RESTRateLimitConfig{
	Bucket: "instance:info",
	Limit:  60,
	Window: time.Minute,
}

func (r *REST) GetInstanceInfo(ctx context.Context) (InstanceInfo, error) {
	var resp InstanceInfo
	err := r.RequestJSON(ctx, RESTRequest{
		Method:    "GET",
		Path:      "/v1/.well-known/fluxer",
		RateLimit: rateLimitInstanceInfo,
	}, &resp)
	if err != nil {
		return InstanceInfo{}, err
	}

	return resp, nil
}
