package flo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"
)

// REST is used to make REST requests to the Fluxer API, respecting rate limits and updating cache.
type REST struct {
	// Auth specifies the Authorization header to use. For most endpoints, it is required.
	Auth string
	// Cache specifies the caching target. If nil is specified, nothing is cached.
	Cache *Cache
	// UserAgent overrides the used user agent. By default it is generated from the library version.
	UserAgent string
	// Client allows configuring the underlying HTTP client.
	Client http.Client
	// BaseURL specifies the base URL for requests. If not specified, the official Fluxer instance is used.
	BaseURL *url.URL

	buckets   map[RateLimitConfig]rateLimitBucket
	bucketsMu sync.Mutex
}

type RESTFormField struct {
	FieldName string
	FileName  string
	Content   io.ReadCloser
}

type RESTRequest struct {
	Method    string
	Path      string
	RateLimit RateLimitConfig

	// Payload specifies a JSON body.
	// If used in combination with Form, it will be added as payload_json.
	Payload any
	// Form specifies a set of form fields.
	// If it is not empty, the content type will be set to multipart/form-data and these fields will be sent as the payload.
	Form []RESTFormField

	AuditLogReason string
}

// RateLimitConfig specifies options to be used to rate limit the request together with other requests using the same config..
// If bucket is not provided, the request will not be rate limited.
// Appropriate rate limit configs can easily be found within [the Fluxer backend] at the time of writing.
// [the Fluxer backend]: https://github.com/fluxerapp/fluxer/tree/refactor/packages/api/src/rate_limit_configs
type RateLimitConfig struct {
	Bucket string
	Limit  int
	Window time.Duration
}

type rateLimitBucket struct {
	filled    int
	leakStart time.Time
}

// acquireBucketSlot acquires a slot in the rate limit bucket, pausing if necessary.
// An error is returned if the passed context is cancelled.
func (r *REST) acquireBucketSlot(ctx context.Context, conf RateLimitConfig) error {
	r.bucketsMu.Lock()

	if r.buckets == nil {
		r.buckets = map[RateLimitConfig]rateLimitBucket{}
	}

	bucket := r.buckets[conf]

	rn := time.Now()

	leakRate := conf.Window / time.Duration(conf.Limit)
	effectiveFilled := bucket.filled - int(rn.Sub(bucket.leakStart)/leakRate)

	if effectiveFilled <= 0 {
		effectiveFilled = 0
		bucket.leakStart = time.Now()
	}

	bucket.filled++
	effectiveFilled++

	r.buckets[conf] = bucket
	r.bucketsMu.Unlock()

	if effectiveFilled > conf.Limit {
		refillDelay := time.Duration(effectiveFilled-conf.Limit)*leakRate + rn.Sub(bucket.leakStart)%leakRate

		select {
		case <-time.After(refillDelay):
			break
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func encodeRESTForm(req RESTRequest) (io.ReadCloser, string, error) {
	var buf bytes.Buffer
	var form multipart.Writer

	for _, field := range req.Form {
		var writer io.Writer
		var err error
		if field.FileName != "" {
			writer, err = form.CreateFormFile(field.FieldName, field.FileName)
		} else {
			writer, err = form.CreateFormField(field.FieldName)
		}
		if err != nil {
			return nil, "", fmt.Errorf("failed to create form field '%s': %w", field.FieldName, err)
		}

		_, err = io.Copy(writer, field.Content)
		if err != nil {
			return nil, "", fmt.Errorf("failed to copy form field content for '%s': %w", field.FieldName, err)
		}
	}

	if req.Payload != nil {
		writer, err := form.CreateFormField("payload_json")
		if err != nil {
			return nil, "", fmt.Errorf("failed to create form field for payload JSON: %w", err)
		}

		err = json.NewEncoder(writer).Encode(req.Payload)
		if err != nil {
			return nil, "", fmt.Errorf("failed to encode payload JSON for form: %w", err)
		}
	}

	err := form.Close()
	if err != nil {
		return nil, "", fmt.Errorf("failed to end form: %w", err)
	}

	contentType := mime.FormatMediaType("multipart/form-data", map[string]string{
		"boundary": form.Boundary(),
	})

	return io.NopCloser(&buf), contentType, nil
}

// Request sends a Request to a Fluxer endpoint. If the returned error is nil, the response body should be closed.
func (r *REST) Request(ctx context.Context, req RESTRequest) (*http.Response, error) {
	if req.RateLimit.Bucket != "" {
		err := r.acquireBucketSlot(ctx, req.RateLimit)
		if err != nil {
			return nil, err
		}
	}

	httpURL := r.BaseURL
	if httpURL == nil {
		httpURL = defaultAPIURL
	}
	if httpURL.Path == "" {
		// NOTE: without this an invalid URL will be generated without a leading slash
		httpURL.Path = "/"
	}
	httpURL = httpURL.JoinPath(req.Path)

	userAgent := r.UserAgent
	if userAgent == "" {
		userAgent = defaultUserAgent
	}

	httpReq := &http.Request{
		Method: req.Method,
		URL:    httpURL,
		Header: map[string][]string{
			"Authorization": {r.Auth},
			"User-Agent":    {userAgent},
		},
	}
	httpReq = httpReq.WithContext(ctx)
	if req.AuditLogReason != "" {
		httpReq.Header.Set("X-Audit-Log-Reason", req.AuditLogReason)
	}

	if len(req.Form) != 0 {
		body, contentType, err := encodeRESTForm(req)
		if err != nil {
			return nil, err
		}

		httpReq.Body = body
		httpReq.Header.Set("Content-Type", contentType)
	} else if req.Payload != nil {
		body, err := json.Marshal(req.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload JSON: %w", err)
		}

		httpReq.Body = io.NopCloser(bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")
	}

	resp, err := r.Client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 && resp.StatusCode <= 599 {
		if resp.Body == nil {
			return nil, &RESTHTTPError{req.Path, *resp}
		}
		defer resp.Body.Close()

		contentType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if err != nil {
			return nil, fmt.Errorf("failed to parse error response Content-Type: %w", err)
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read error response body")
		}

		if contentType != "application/json" {
			httpErr := RESTHTTPError{req.Path, *resp}
			httpErr.Response.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			return nil, &httpErr
		}

		var rawErr struct {
			Code    RESTErrorCode `json:"code"`
			Message string        `json:"message"`
		}

		err = json.Unmarshal(bodyBytes, &rawErr)
		if err != nil || rawErr.Code == "" {
			httpErr := RESTHTTPError{req.Path, *resp}
			httpErr.Response.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			return nil, &httpErr
		}

		return nil, &RESTAPIError{
			Path:    req.Path,
			Status:  resp.StatusCode,
			Code:    rawErr.Code,
			Message: rawErr.Message,
		}
	}

	return resp, nil
}

func (r *REST) RequestJSON(ctx context.Context, req RESTRequest, result any) error {
	resp, err := r.Request(ctx, req)
	if err != nil {
		return err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	contentType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return errors.New("failed to parse Content-Type: %w")
	}
	if contentType != "application/json" {
		return fmt.Errorf("Content-Type is not application/json: %s", contentType)
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

func (r *REST) RequestNoContent(ctx context.Context, req RESTRequest) error {
	resp, err := r.Request(ctx, req)
	if err != nil {
		return err
	}

	if resp.Body != nil {
		resp.Body.Close()
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("expected 204 No Content but got %s", resp.Status)
	}

	return nil
}

// RESTAPIError represents an API error from Fluxer in the expected format.
type RESTAPIError struct {
	Path    string
	Status  int
	Code    RESTErrorCode
	Message string
}

func (r *RESTAPIError) Error() string {
	return fmt.Sprintf("API error on %s: %s (%s)", r.Path, r.Message, r.Code)
}

// RESTHTTPError represents an error response in an unexpected format.
// Unlike a typical [http.Response], the body does not need to be closed.
type RESTHTTPError struct {
	Path     string
	Response http.Response
}

func (r *RESTHTTPError) Error() string {
	return fmt.Sprintf("unexpected HTTP %s on %s", r.Response.Status, r.Path)
}

// IsRESTError is a convinience method to check for one of a list of REST error codes.
func IsRESTError(err error, codes ...RESTErrorCode) bool {
	var restErr *RESTAPIError
	if !errors.As(err, &restErr) {
		return false
	}

	return len(codes) == 0 || slices.Contains(codes, restErr.Code)
}

type RESTErrorCode string

const (
	RESTErrAccessDenied                                 RESTErrorCode = "ACCESS_DENIED"
	RESTErrAccountDisabled                              RESTErrorCode = "ACCOUNT_DISABLED"
	RESTErrBadGateway                                   RESTErrorCode = "BAD_GATEWAY"
	RESTErrBadRequest                                   RESTErrorCode = "BAD_REQUEST"
	RESTErrBlueskyOAuthAuthorizationFailed              RESTErrorCode = "BLUESKY_OAUTH_AUTHORIZATION_FAILED"
	RESTErrBlueskyOAuthCallbackFailed                   RESTErrorCode = "BLUESKY_OAUTH_CALLBACK_FAILED"
	RESTErrBlueskyOAuthNotEnabled                       RESTErrorCode = "BLUESKY_OAUTH_NOT_ENABLED"
	RESTErrBlueskyOAuthSessionExpired                   RESTErrorCode = "BLUESKY_OAUTH_SESSION_EXPIRED"
	RESTErrBlueskyOAuthStateInvalid                     RESTErrorCode = "BLUESKY_OAUTH_STATE_INVALID"
	RESTErrAccountScheduledForDeletion                  RESTErrorCode = "ACCOUNT_SCHEDULED_FOR_DELETION"
	RESTErrAccountSuspendedPermanently                  RESTErrorCode = "ACCOUNT_SUSPENDED_PERMANENTLY"
	RESTErrAccountSuspendedTemporarily                  RESTErrorCode = "ACCOUNT_SUSPENDED_TEMPORARILY"
	RESTErrAccountSuspiciousActivity                    RESTErrorCode = "ACCOUNT_SUSPICIOUS_ACTIVITY"
	RESTErrAccountTooNewForGuild                        RESTErrorCode = "ACCOUNT_TOO_NEW_FOR_GUILD"
	RESTErrACLsMustBeNonEmpty                           RESTErrorCode = "ACLS_MUST_BE_NON_EMPTY"
	RESTErrAdminAPIKeyNotFound                          RESTErrorCode = "ADMIN_API_KEY_NOT_FOUND"
	RESTErrApplicationNotFound                          RESTErrorCode = "APPLICATION_NOT_FOUND"
	RESTErrApplicationNotOwned                          RESTErrorCode = "APPLICATION_NOT_OWNED"
	RESTErrAlreadyFriends                               RESTErrorCode = "ALREADY_FRIENDS"
	RESTErrAuditLogIndexing                             RESTErrorCode = "AUDIT_LOG_INDEXING"
	RESTErrBotsCannotSendFriendRequests                 RESTErrorCode = "BOTS_CANNOT_SEND_FRIEND_REQUESTS"
	RESTErrBotAlreadyInGuild                            RESTErrorCode = "BOT_ALREADY_IN_GUILD"
	RESTErrBotApplicationNotFound                       RESTErrorCode = "BOT_APPLICATION_NOT_FOUND"
	RESTErrBotIsPrivate                                 RESTErrorCode = "BOT_IS_PRIVATE"
	RESTErrBotUserAuthEndpointAccessDenied              RESTErrorCode = "BOT_USER_AUTH_ENDPOINT_ACCESS_DENIED"
	RESTErrBotUserAuthSessionCreationDenied             RESTErrorCode = "BOT_USER_AUTH_SESSION_CREATION_DENIED"
	RESTErrBotUserGenerationFailed                      RESTErrorCode = "BOT_USER_GENERATION_FAILED"
	RESTErrBotUserNotFound                              RESTErrorCode = "BOT_USER_NOT_FOUND"
	RESTErrCallAlreadyExists                            RESTErrorCode = "CALL_ALREADY_EXISTS"
	RESTErrCannotEditOtherUserMessage                   RESTErrorCode = "CANNOT_EDIT_OTHER_USER_MESSAGE"
	RESTErrCannotExecuteOnDM                            RESTErrorCode = "CANNOT_EXECUTE_ON_DM"
	RESTErrCannotModifySystemWebhook                    RESTErrorCode = "CANNOT_MODIFY_SYSTEM_WEBHOOK"
	RESTErrCannotModifyVoiceState                       RESTErrorCode = "CANNOT_MODIFY_VOICE_STATE"
	RESTErrCannotRedeemPlutoniumWithVisionary           RESTErrorCode = "CANNOT_REDEEM_PLUTONIUM_WITH_VISIONARY"
	RESTErrCannotReportOwnGuild                         RESTErrorCode = "CANNOT_REPORT_OWN_GUILD"
	RESTErrCannotReportOwnMessage                       RESTErrorCode = "CANNOT_REPORT_OWN_MESSAGE"
	RESTErrCannotReportYourself                         RESTErrorCode = "CANNOT_REPORT_YOURSELF"
	RESTErrCannotSendEmptyMessage                       RESTErrorCode = "CANNOT_SEND_EMPTY_MESSAGE"
	RESTErrCannotSendFriendRequestToBlockedUser         RESTErrorCode = "CANNOT_SEND_FRIEND_REQUEST_TO_BLOCKED_USER"
	RESTErrCannotSendFriendRequestToSelf                RESTErrorCode = "CANNOT_SEND_FRIEND_REQUEST_TO_SELF"
	RESTErrCannotSendMessagesInNonTextChannel           RESTErrorCode = "CANNOT_SEND_MESSAGES_IN_NON_TEXT_CHANNEL"
	RESTErrCannotSendMessagesToUser                     RESTErrorCode = "CANNOT_SEND_MESSAGES_TO_USER"
	RESTErrCannotTransferOwnershipToBot                 RESTErrorCode = "CANNOT_TRANSFER_OWNERSHIP_TO_BOT"
	RESTErrCannotShrinkReservedSlots                    RESTErrorCode = "CANNOT_SHRINK_RESERVED_SLOTS"
	RESTErrCaptchaRequired                              RESTErrorCode = "CAPTCHA_REQUIRED"
	RESTErrChannelIndexing                              RESTErrorCode = "CHANNEL_INDEXING"
	RESTErrCommunicationDisabled                        RESTErrorCode = "COMMUNICATION_DISABLED"
	RESTErrConnectionAlreadyExists                      RESTErrorCode = "CONNECTION_ALREADY_EXISTS"
	RESTErrConnectionInitiationTokenInvalid             RESTErrorCode = "CONNECTION_INITIATION_TOKEN_INVALID"
	RESTErrConnectionInvalidIdentifier                  RESTErrorCode = "CONNECTION_INVALID_IDENTIFIER"
	RESTErrConnectionInvalidType                        RESTErrorCode = "CONNECTION_INVALID_TYPE"
	RESTErrConnectionLimitReached                       RESTErrorCode = "CONNECTION_LIMIT_REACHED"
	RESTErrConnectionNotFound                           RESTErrorCode = "CONNECTION_NOT_FOUND"
	RESTErrConnectionVerificationFailed                 RESTErrorCode = "CONNECTION_VERIFICATION_FAILED"
	RESTErrConflict                                     RESTErrorCode = "CONFLICT"
	RESTErrContentBlocked                               RESTErrorCode = "CONTENT_BLOCKED"
	RESTErrCreationFailed                               RESTErrorCode = "CREATION_FAILED"
	RESTErrCSAMScanFailed                               RESTErrorCode = "CSAM_SCAN_FAILED"
	RESTErrCSAMScanParseError                           RESTErrorCode = "CSAM_SCAN_PARSE_ERROR"
	RESTErrCSAMScanSubscriptionError                    RESTErrorCode = "CSAM_SCAN_SUBSCRIPTION_ERROR"
	RESTErrCSAMScanTimeout                              RESTErrorCode = "CSAM_SCAN_TIMEOUT"
	RESTErrDecryptionFailed                             RESTErrorCode = "DECRYPTION_FAILED"
	RESTErrDeletionFailed                               RESTErrorCode = "DELETION_FAILED"
	RESTErrDiscoveryAlreadyApplied                      RESTErrorCode = "DISCOVERY_ALREADY_APPLIED"
	RESTErrDiscoveryApplicationAlreadyReviewed          RESTErrorCode = "DISCOVERY_APPLICATION_ALREADY_REVIEWED"
	RESTErrDiscoveryApplicationNotFound                 RESTErrorCode = "DISCOVERY_APPLICATION_NOT_FOUND"
	RESTErrDiscoveryDescriptionRequired                 RESTErrorCode = "DISCOVERY_DESCRIPTION_REQUIRED"
	RESTErrDiscoveryDisabled                            RESTErrorCode = "DISCOVERY_DISABLED"
	RESTErrDiscoveryInsufficientMembers                 RESTErrorCode = "DISCOVERY_INSUFFICIENT_MEMBERS"
	RESTErrDiscoveryInvalidCategory                     RESTErrorCode = "DISCOVERY_INVALID_CATEGORY"
	RESTErrDiscoveryNotDiscoverable                     RESTErrorCode = "DISCOVERY_NOT_DISCOVERABLE"
	RESTErrDiscriminatorRequired                        RESTErrorCode = "DISCRIMINATOR_REQUIRED"
	RESTErrEmailServiceNotTestable                      RESTErrorCode = "EMAIL_SERVICE_NOT_TESTABLE"
	RESTErrEmailVerificationRequired                    RESTErrorCode = "EMAIL_VERIFICATION_REQUIRED"
	RESTErrEmptyEncryptedBody                           RESTErrorCode = "EMPTY_ENCRYPTED_BODY"
	RESTErrEncryptionFailed                             RESTErrorCode = "ENCRYPTION_FAILED"
	RESTErrExplicitContentCannotBeSent                  RESTErrorCode = "EXPLICIT_CONTENT_CANNOT_BE_SENT"
	RESTErrFeatureNotAvailableSelfHosted                RESTErrorCode = "FEATURE_NOT_AVAILABLE_SELF_HOSTED"
	RESTErrFeatureTemporarilyDisabled                   RESTErrorCode = "FEATURE_TEMPORARILY_DISABLED"
	RESTErrFileSizeTooLarge                             RESTErrorCode = "FILE_SIZE_TOO_LARGE"
	RESTErrForbidden                                    RESTErrorCode = "FORBIDDEN"
	RESTErrFriendRequestBlocked                         RESTErrorCode = "FRIEND_REQUEST_BLOCKED"
	RESTErrGatewayTimeout                               RESTErrorCode = "GATEWAY_TIMEOUT"
	RESTErrGeneralError                                 RESTErrorCode = "GENERAL_ERROR"
	RESTErrGone                                         RESTErrorCode = "GONE"
	RESTErrGiftCodeAlreadyRedeemed                      RESTErrorCode = "GIFT_CODE_ALREADY_REDEEMED"
	RESTErrGuildPhoneVerificationRequired               RESTErrorCode = "GUILD_PHONE_VERIFICATION_REQUIRED"
	RESTErrGuildVerificationRequired                    RESTErrorCode = "GUILD_VERIFICATION_REQUIRED"
	RESTErrHandoffCodeExpired                           RESTErrorCode = "HANDOFF_CODE_EXPIRED"
	RESTErrHarvestExpired                               RESTErrorCode = "HARVEST_EXPIRED"
	RESTErrHarvestFailed                                RESTErrorCode = "HARVEST_FAILED"
	RESTErrHarvestNotReady                              RESTErrorCode = "HARVEST_NOT_READY"
	RESTErrHarvestOnCooldown                            RESTErrorCode = "HARVEST_ON_COOLDOWN"
	RESTErrHttpGetAuthorizeNotSupported                 RESTErrorCode = "HTTP_GET_AUTHORIZE_NOT_SUPPORTED"
	RESTErrInstanceVersionMismatch                      RESTErrorCode = "INSTANCE_VERSION_MISMATCH"
	RESTErrInternalServerError                          RESTErrorCode = "INTERNAL_SERVER_ERROR"
	RESTErrInvalidACLsFormat                            RESTErrorCode = "INVALID_ACLS_FORMAT"
	RESTErrInvalidAPIOrigin                             RESTErrorCode = "INVALID_API_ORIGIN"
	RESTErrInvalidAuthToken                             RESTErrorCode = "INVALID_AUTH_TOKEN"
	RESTErrInvalidBotFlag                               RESTErrorCode = "INVALID_BOT_FLAG"
	RESTErrInvalidCaptcha                               RESTErrorCode = "INVALID_CAPTCHA"
	RESTErrInvalidChannelTypeForCall                    RESTErrorCode = "INVALID_CHANNEL_TYPE_FOR_CALL"
	RESTErrInvalidChannelType                           RESTErrorCode = "INVALID_CHANNEL_TYPE"
	RESTErrInvalidClient                                RESTErrorCode = "INVALID_CLIENT"
	RESTErrInvalidClientSecret                          RESTErrorCode = "INVALID_CLIENT_SECRET"
	RESTErrInvalidDSAReportTarget                       RESTErrorCode = "INVALID_DSA_REPORT_TARGET"
	RESTErrInvalidDSATicket                             RESTErrorCode = "INVALID_DSA_TICKET"
	RESTErrInvalidDSAVerificationCode                   RESTErrorCode = "INVALID_DSA_VERIFICATION_CODE"
	RESTErrInvalidDecryptedJson                         RESTErrorCode = "INVALID_DECRYPTED_JSON"
	RESTErrInvalidEphemeralKey                          RESTErrorCode = "INVALID_EPHEMERAL_KEY"
	RESTErrInvalidFlagsFormat                           RESTErrorCode = "INVALID_FLAGS_FORMAT"
	RESTErrInvalidIV                                    RESTErrorCode = "INVALID_IV"
	RESTErrInvalidFormBody                              RESTErrorCode = "INVALID_FORM_BODY"
	RESTErrInvalidGrant                                 RESTErrorCode = "INVALID_GRANT"
	RESTErrInvalidHandoffCode                           RESTErrorCode = "INVALID_HANDOFF_CODE"
	RESTErrInvalidPackType                              RESTErrorCode = "INVALID_PACK_TYPE"
	RESTErrInvalidPermissionsInteger                    RESTErrorCode = "INVALID_PERMISSIONS_INTEGER"
	RESTErrInvalidPermissionsNegative                   RESTErrorCode = "INVALID_PERMISSIONS_NEGATIVE"
	RESTErrInvalidPhoneNumber                           RESTErrorCode = "INVALID_PHONE_NUMBER"
	RESTErrInvalidPhoneVerificationCode                 RESTErrorCode = "INVALID_PHONE_VERIFICATION_CODE"
	RESTErrInvalidRedirectURI                           RESTErrorCode = "INVALID_REDIRECT_URI"
	RESTErrInvalidRequest                               RESTErrorCode = "INVALID_REQUEST"
	RESTErrInvalidResponseTypeForNonBot                 RESTErrorCode = "INVALID_RESPONSE_TYPE_FOR_NON_BOT"
	RESTErrInvalidScope                                 RESTErrorCode = "INVALID_SCOPE"
	RESTErrInvalidStreamKeyFormat                       RESTErrorCode = "INVALID_STREAM_KEY_FORMAT"
	RESTErrInvalidStreamThumbnailPayload                RESTErrorCode = "INVALID_STREAM_THUMBNAIL_PAYLOAD"
	RESTErrInvalidSudoToken                             RESTErrorCode = "INVALID_SUDO_TOKEN"
	RESTErrInvalidSuspiciousFlagsFormat                 RESTErrorCode = "INVALID_SUSPICIOUS_FLAGS_FORMAT"
	RESTErrInvalidSystemFlag                            RESTErrorCode = "INVALID_SYSTEM_FLAG"
	RESTErrInvalidTimestamp                             RESTErrorCode = "INVALID_TIMESTAMP"
	RESTErrInvalidToken                                 RESTErrorCode = "INVALID_TOKEN"
	RESTErrInvalidWebAuthnAuthenticationCounter         RESTErrorCode = "INVALID_WEBAUTHN_AUTHENTICATION_COUNTER"
	RESTErrInvalidWebAuthnCredentialCounter             RESTErrorCode = "INVALID_WEBAUTHN_CREDENTIAL_COUNTER"
	RESTErrInvalidWebAuthnCredential                    RESTErrorCode = "INVALID_WEBAUTHN_CREDENTIAL"
	RESTErrInvalidWebAuthnPublicKeyFormat               RESTErrorCode = "INVALID_WEBAUTHN_PUBLIC_KEY_FORMAT"
	RESTErrInvitesDisabled                              RESTErrorCode = "INVITES_DISABLED"
	RESTErrIPAuthorizationRequired                      RESTErrorCode = "IP_AUTHORIZATION_REQUIRED"
	RESTErrIPAuthorizationResendCooldown                RESTErrorCode = "IP_AUTHORIZATION_RESEND_COOLDOWN"
	RESTErrIPAuthorizationResendLimitExceeded           RESTErrorCode = "IP_AUTHORIZATION_RESEND_LIMIT_EXCEEDED"
	RESTErrIPBanned                                     RESTErrorCode = "IP_BANNED"
	RESTErrMaxAnimatedEmojis                            RESTErrorCode = "MAX_ANIMATED_EMOJIS"
	RESTErrMaxBookmarks                                 RESTErrorCode = "MAX_BOOKMARKS"
	RESTErrMaxCategoryChannels                          RESTErrorCode = "MAX_CATEGORY_CHANNELS"
	RESTErrMaxEmojis                                    RESTErrorCode = "MAX_EMOJIS"
	RESTErrMaxFavoriteMemes                             RESTErrorCode = "MAX_FAVORITE_MEMES"
	RESTErrMaxFriends                                   RESTErrorCode = "MAX_FRIENDS"
	RESTErrMaxGroupDMRecipients                         RESTErrorCode = "MAX_GROUP_DM_RECIPIENTS"
	RESTErrMaxGroupDMs                                  RESTErrorCode = "MAX_GROUP_DMS"
	RESTErrMaxGuildChannels                             RESTErrorCode = "MAX_GUILD_CHANNELS"
	RESTErrMaxGuildMembers                              RESTErrorCode = "MAX_GUILD_MEMBERS"
	RESTErrMaxGuildRoles                                RESTErrorCode = "MAX_GUILD_ROLES"
	RESTErrMaxGuilds                                    RESTErrorCode = "MAX_GUILDS"
	RESTErrMaxInvites                                   RESTErrorCode = "MAX_INVITES"
	RESTErrMaxPackExpressions                           RESTErrorCode = "MAX_PACK_EXPRESSIONS"
	RESTErrMaxPacks                                     RESTErrorCode = "MAX_PACKS"
	RESTErrMaxPinsPerChannel                            RESTErrorCode = "MAX_PINS_PER_CHANNEL"
	RESTErrMessageTotalAttachmentSizeTooLarge           RESTErrorCode = "MESSAGE_TOTAL_ATTACHMENT_SIZE_TOO_LARGE"
	RESTErrMaxReactions                                 RESTErrorCode = "MAX_REACTIONS"
	RESTErrMaxStickers                                  RESTErrorCode = "MAX_STICKERS"
	RESTErrMaxWebhooksPerChannel                        RESTErrorCode = "MAX_WEBHOOKS_PER_CHANNEL"
	RESTErrMaxWebhooksPerGuild                          RESTErrorCode = "MAX_WEBHOOKS_PER_GUILD"
	RESTErrMaxWebhooks                                  RESTErrorCode = "MAX_WEBHOOKS"
	RESTErrNcmecAlreadySubmitted                        RESTErrorCode = "NCMEC_ALREADY_SUBMITTED"
	RESTErrNcmecSubmissionFailed                        RESTErrorCode = "NCMEC_SUBMISSION_FAILED"
	RESTErrMediaMetadataError                           RESTErrorCode = "MEDIA_METADATA_ERROR"
	RESTErrMethodNotAllowed                             RESTErrorCode = "METHOD_NOT_ALLOWED"
	RESTErrMissingAccess                                RESTErrorCode = "MISSING_ACCESS"
	RESTErrMissingACL                                   RESTErrorCode = "MISSING_ACL"
	RESTErrMissingAuthorization                         RESTErrorCode = "MISSING_AUTHORIZATION"
	RESTErrMissingClientSecret                          RESTErrorCode = "MISSING_CLIENT_SECRET"
	RESTErrMissingEphemeralKey                          RESTErrorCode = "MISSING_EPHEMERAL_KEY"
	RESTErrMissingIV                                    RESTErrorCode = "MISSING_IV"
	RESTErrMissingOAuthAdminScope                       RESTErrorCode = "MISSING_OAUTH_ADMIN_SCOPE"
	RESTErrMissingOAuthFields                           RESTErrorCode = "MISSING_OAUTH_FIELDS"
	RESTErrMissingOAuthScope                            RESTErrorCode = "MISSING_OAUTH_SCOPE"
	RESTErrMissingPermissions                           RESTErrorCode = "MISSING_PERMISSIONS"
	RESTErrMissingRedirectURI                           RESTErrorCode = "MISSING_REDIRECT_URI"
	RESTErrNoActiveCall                                 RESTErrorCode = "NO_ACTIVE_CALL"
	RESTErrNoActiveSubscription                         RESTErrorCode = "NO_ACTIVE_SUBSCRIPTION"
	RESTErrNotFound                                     RESTErrorCode = "NOT_FOUND"
	RESTErrNotImplemented                               RESTErrorCode = "NOT_IMPLEMENTED"
	RESTErrNoPasskeysRegistered                         RESTErrorCode = "NO_PASSKEYS_REGISTERED"
	RESTErrNoPendingDeletion                            RESTErrorCode = "NO_PENDING_DELETION"
	RESTErrNoUsersWithFluxertagExist                    RESTErrorCode = "NO_USERS_WITH_FLUXERTAG_EXIST"
	RESTErrNoVisionarySlotsAvailable                    RESTErrorCode = "NO_VISIONARY_SLOTS_AVAILABLE"
	RESTErrNotABotApplication                           RESTErrorCode = "NOT_A_BOT_APPLICATION"
	RESTErrNotFriendsWithUser                           RESTErrorCode = "NOT_FRIENDS_WITH_USER"
	RESTErrNotOwnerOfAdminAPIKey                        RESTErrorCode = "NOT_OWNER_OF_ADMIN_API_KEY"
	RESTErrNSFWContentAgeRestricted                     RESTErrorCode = "NSFW_CONTENT_AGE_RESTRICTED"
	RESTErrPackAccessDenied                             RESTErrorCode = "PACK_ACCESS_DENIED"
	RESTErrPasskeyAuthenticationFailed                  RESTErrorCode = "PASSKEY_AUTHENTICATION_FAILED"
	RESTErrPasskeysDisabled                             RESTErrorCode = "PASSKEYS_DISABLED"
	RESTErrPhoneAlreadyUsed                             RESTErrorCode = "PHONE_ALREADY_USED"
	RESTErrPhoneRateLimitExceeded                       RESTErrorCode = "PHONE_RATE_LIMIT_EXCEEDED"
	RESTErrPhoneRequiredForSMSMFA                       RESTErrorCode = "PHONE_REQUIRED_FOR_SMS_MFA"
	RESTErrPhoneVerificationRequired                    RESTErrorCode = "PHONE_VERIFICATION_REQUIRED"
	RESTErrPremiumPurchaseBlocked                       RESTErrorCode = "PREMIUM_PURCHASE_BLOCKED"
	RESTErrPreviewMustBeJPEG                            RESTErrorCode = "PREVIEW_MUST_BE_JPEG"
	RESTErrProcessingFailed                             RESTErrorCode = "PROCESSING_FAILED"
	RESTErrRateLimited                                  RESTErrorCode = "RATE_LIMITED"
	RESTErrRedirectURIRequiredForNonBot                 RESTErrorCode = "REDIRECT_URI_REQUIRED_FOR_NON_BOT"
	RESTErrReportAlreadyResolved                        RESTErrorCode = "REPORT_ALREADY_RESOLVED"
	RESTErrReportBanned                                 RESTErrorCode = "REPORT_BANNED"
	RESTErrResponseValidationError                      RESTErrorCode = "RESPONSE_VALIDATION_ERROR"
	RESTErrServiceUnavailable                           RESTErrorCode = "SERVICE_UNAVAILABLE"
	RESTErrSessionTokenMismatch                         RESTErrorCode = "SESSION_TOKEN_MISMATCH"
	RESTErrSlowmodeRateLimited                          RESTErrorCode = "SLOWMODE_RATE_LIMITED"
	RESTErrSMSMFANotEnabled                             RESTErrorCode = "SMS_MFA_NOT_ENABLED"
	RESTErrSMSMFARequiresTOTP                           RESTErrorCode = "SMS_MFA_REQUIRES_TOTP"
	RESTErrSMSVerificationUnavailable                   RESTErrorCode = "SMS_VERIFICATION_UNAVAILABLE"
	RESTErrSsoRequired                                  RESTErrorCode = "SSO_REQUIRED"
	RESTErrStreamKeyChannelMismatch                     RESTErrorCode = "STREAM_KEY_CHANNEL_MISMATCH"
	RESTErrStreamKeyScopeMismatch                       RESTErrorCode = "STREAM_KEY_SCOPE_MISMATCH"
	RESTErrStreamThumbnailPayloadEmpty                  RESTErrorCode = "STREAM_THUMBNAIL_PAYLOAD_EMPTY"
	RESTErrStripeError                                  RESTErrorCode = "STRIPE_ERROR"
	RESTErrStripeGiftRedemptionInProgress               RESTErrorCode = "STRIPE_GIFT_REDEMPTION_IN_PROGRESS"
	RESTErrStripeInvalidProduct                         RESTErrorCode = "STRIPE_INVALID_PRODUCT"
	RESTErrStripeInvalidProductConfiguration            RESTErrorCode = "STRIPE_INVALID_PRODUCT_CONFIGURATION"
	RESTErrStripeNoActiveSubscription                   RESTErrorCode = "STRIPE_NO_ACTIVE_SUBSCRIPTION"
	RESTErrStripeNoPurchaseHistory                      RESTErrorCode = "STRIPE_NO_PURCHASE_HISTORY"
	RESTErrStripeNoSubscription                         RESTErrorCode = "STRIPE_NO_SUBSCRIPTION"
	RESTErrStripePaymentNotAvailable                    RESTErrorCode = "STRIPE_PAYMENT_NOT_AVAILABLE"
	RESTErrStripeSubscriptionAlreadyCanceling           RESTErrorCode = "STRIPE_SUBSCRIPTION_ALREADY_CANCELING"
	RESTErrStripeSubscriptionNotCanceling               RESTErrorCode = "STRIPE_SUBSCRIPTION_NOT_CANCELING"
	RESTErrStripeSubscriptionPeriodEndMissing           RESTErrorCode = "STRIPE_SUBSCRIPTION_PERIOD_END_MISSING"
	RESTErrStripeWebhookNotAvailable                    RESTErrorCode = "STRIPE_WEBHOOK_NOT_AVAILABLE"
	RESTErrStripeWebhookSignatureInvalid                RESTErrorCode = "STRIPE_WEBHOOK_SIGNATURE_INVALID"
	RESTErrStripeWebhookSignatureMissing                RESTErrorCode = "STRIPE_WEBHOOK_SIGNATURE_MISSING"
	RESTErrDonationAmountInvalid                        RESTErrorCode = "DONATION_AMOUNT_INVALID"
	RESTErrDonationMagicLinkExpired                     RESTErrorCode = "DONATION_MAGIC_LINK_EXPIRED"
	RESTErrDonationMagicLinkInvalid                     RESTErrorCode = "DONATION_MAGIC_LINK_INVALID"
	RESTErrDonationMagicLinkUsed                        RESTErrorCode = "DONATION_MAGIC_LINK_USED"
	RESTErrDonorNotFound                                RESTErrorCode = "DONOR_NOT_FOUND"
	RESTErrSudoModeRequired                             RESTErrorCode = "SUDO_MODE_REQUIRED"
	RESTErrTagAlreadyTaken                              RESTErrorCode = "TAG_ALREADY_TAKEN"
	RESTErrTemporaryInviteRequiresPresence              RESTErrorCode = "TEMPORARY_INVITE_REQUIRES_PRESENCE"
	RESTErrTestHarnessDisabled                          RESTErrorCode = "TEST_HARNESS_DISABLED"
	RESTErrTestHarnessForbidden                         RESTErrorCode = "TEST_HARNESS_FORBIDDEN"
	RESTErrTwoFaNotEnabled                              RESTErrorCode = "TWO_FA_NOT_ENABLED"
	RESTErrTwoFactorRequired                            RESTErrorCode = "TWO_FACTOR_REQUIRED"
	RESTErrUnauthorized                                 RESTErrorCode = "UNAUTHORIZED"
	RESTErrUnclaimedAccountCannotAcceptFriendRequests   RESTErrorCode = "UNCLAIMED_ACCOUNT_CANNOT_ACCEPT_FRIEND_REQUESTS"
	RESTErrUnclaimedAccountCannotAddReactions           RESTErrorCode = "UNCLAIMED_ACCOUNT_CANNOT_ADD_REACTIONS"
	RESTErrUnclaimedAccountCannotCreateApplications     RESTErrorCode = "UNCLAIMED_ACCOUNT_CANNOT_CREATE_APPLICATIONS"
	RESTErrUnclaimedAccountCannotJoinGroupDMs           RESTErrorCode = "UNCLAIMED_ACCOUNT_CANNOT_JOIN_GROUP_DMS"
	RESTErrUnclaimedAccountCannotJoinOneOnOneVoiceCalls RESTErrorCode = "UNCLAIMED_ACCOUNT_CANNOT_JOIN_ONE_ON_ONE_VOICE_CALLS"
	RESTErrUnclaimedAccountCannotJoinVoiceChannels      RESTErrorCode = "UNCLAIMED_ACCOUNT_CANNOT_JOIN_VOICE_CHANNELS"
	RESTErrUnclaimedAccountCannotMakePurchases          RESTErrorCode = "UNCLAIMED_ACCOUNT_CANNOT_MAKE_PURCHASES"
	RESTErrUnclaimedAccountCannotSendDirectMessages     RESTErrorCode = "UNCLAIMED_ACCOUNT_CANNOT_SEND_DIRECT_MESSAGES"
	RESTErrUnclaimedAccountCannotSendFriendRequests     RESTErrorCode = "UNCLAIMED_ACCOUNT_CANNOT_SEND_FRIEND_REQUESTS"
	RESTErrUnclaimedAccountCannotSendMessages           RESTErrorCode = "UNCLAIMED_ACCOUNT_CANNOT_SEND_MESSAGES"
	RESTErrUnknownChannel                               RESTErrorCode = "UNKNOWN_CHANNEL"
	RESTErrUnknownEmoji                                 RESTErrorCode = "UNKNOWN_EMOJI"
	RESTErrUnknownFavoriteMeme                          RESTErrorCode = "UNKNOWN_FAVORITE_MEME"
	RESTErrUnknownGiftCode                              RESTErrorCode = "UNKNOWN_GIFT_CODE"
	RESTErrUnknownGuild                                 RESTErrorCode = "UNKNOWN_GUILD"
	RESTErrUnknownHarvest                               RESTErrorCode = "UNKNOWN_HARVEST"
	RESTErrUnknownInvite                                RESTErrorCode = "UNKNOWN_INVITE"
	RESTErrUnknownMember                                RESTErrorCode = "UNKNOWN_MEMBER"
	RESTErrUnknownMessage                               RESTErrorCode = "UNKNOWN_MESSAGE"
	RESTErrUnknownPack                                  RESTErrorCode = "UNKNOWN_PACK"
	RESTErrUnknownReport                                RESTErrorCode = "UNKNOWN_REPORT"
	RESTErrUnknownRole                                  RESTErrorCode = "UNKNOWN_ROLE"
	RESTErrUnknownSticker                               RESTErrorCode = "UNKNOWN_STICKER"
	RESTErrUnknownSuspiciousFlag                        RESTErrorCode = "UNKNOWN_SUSPICIOUS_FLAG"
	RESTErrUnknownUserFlag                              RESTErrorCode = "UNKNOWN_USER_FLAG"
	RESTErrUnknownUser                                  RESTErrorCode = "UNKNOWN_USER"
	RESTErrUnknownVoiceRegion                           RESTErrorCode = "UNKNOWN_VOICE_REGION"
	RESTErrUnknownVoiceServer                           RESTErrorCode = "UNKNOWN_VOICE_SERVER"
	RESTErrUnknownWebAuthnCredential                    RESTErrorCode = "UNKNOWN_WEBAUTHN_CREDENTIAL"
	RESTErrUnknownApplication                           RESTErrorCode = "UNKNOWN_APPLICATION"
	RESTErrUnknownWebhook                               RESTErrorCode = "UNKNOWN_WEBHOOK"
	RESTErrUnsupportedResponseType                      RESTErrorCode = "UNSUPPORTED_RESPONSE_TYPE"
	RESTErrUsernameNotAvailable                         RESTErrorCode = "USERNAME_NOT_AVAILABLE"
	RESTErrUpdateFailed                                 RESTErrorCode = "UPDATE_FAILED"
	RESTErrUserBannedFromGuild                          RESTErrorCode = "USER_BANNED_FROM_GUILD"
	RESTErrUserIPBannedFromGuild                        RESTErrorCode = "USER_IP_BANNED_FROM_GUILD"
	RESTErrUserNotInVoice                               RESTErrorCode = "USER_NOT_IN_VOICE"
	RESTErrUserOwnsGuilds                               RESTErrorCode = "USER_OWNS_GUILDS"
	RESTErrValidationError                              RESTErrorCode = "VALIDATION_ERROR"
	RESTErrVoiceChannelFull                             RESTErrorCode = "VOICE_CHANNEL_FULL"
	RESTErrWebAuthnCredentialLimitReached               RESTErrorCode = "WEBAUTHN_CREDENTIAL_LIMIT_REACHED"
)
