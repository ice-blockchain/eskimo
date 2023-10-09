// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"
	"fmt"
	"strings"

	"dario.cat/mergo"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/auth"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:funlen // .
func (c *client) SignIn(ctx context.Context, emailLinkPayload, confirmationCode string) error {
	var token magicLinkToken
	if err := parseJwtToken(emailLinkPayload, c.cfg.EmailValidation.JwtSecret, &token); err != nil {
		return errors.Wrapf(err, "invalid email token:%v", emailLinkPayload)
	}
	email := token.Subject
	id := loginID{Email: email, DeviceUniqueID: token.DeviceUniqueID}
	els, err := c.getEmailLinkSignInByPk(ctx, &id, token.OldEmail)
	if err != nil {
		if storage.IsErr(err, storage.ErrNotFound) {
			return errors.Wrapf(ErrNoConfirmationRequired, "[getEmailLinkSignInByPk] no pending confirmation for email:%v", email)
		}

		return errors.Wrapf(err, "failed to get user info by email:%v(old email:%v)", email, token.OldEmail)
	}
	if vErr := c.verifySignIn(ctx, els, &id, emailLinkPayload, confirmationCode, token.OTP); vErr != nil {
		return errors.Wrapf(vErr, "can't verify sign in for id:%#v", id)
	}
	var emailConfirmed bool
	if token.OldEmail != "" {
		if err = c.handleEmailModification(ctx, els, email, token.OldEmail, token.NotifyEmail); err != nil {
			return errors.Wrapf(err, "failed to handle email modification:%v", email)
		}
		emailConfirmed = true
		els.Email = email
	}
	if fErr := c.finishAuthProcess(ctx, &id, *els.UserID, token.OTP, els.IssuedTokenSeq, emailConfirmed, els.Metadata); fErr != nil {
		var mErr *multierror.Error
		if token.OldEmail != "" {
			mErr = multierror.Append(mErr,
				errors.Wrapf(c.resetEmailModification(ctx, *els.UserID, token.OldEmail),
					"[reset] resetEmailModification failed for email:%v", token.OldEmail),
				errors.Wrapf(c.resetFirebaseEmailModification(ctx, els.Metadata, token.OldEmail),
					"[reset] resetEmailModification failed for email:%v", token.OldEmail),
			)
		}
		mErr = multierror.Append(mErr, errors.Wrapf(fErr, "can't finish auth process for userID:%v,email:%v,otp:%v", els.UserID, email, token.OTP))

		return mErr.ErrorOrNil() //nolint:wrapcheck // .
	}

	return nil
}

//nolint:revive,gocognit // .
func (c *client) verifySignIn(ctx context.Context, els *emailLinkSignIn, id *loginID, emailLinkPayload, confirmationCode, tokenOTP string) error {
	if els.OTP == *els.UserID || els.OTP != tokenOTP {
		return errors.Wrapf(ErrNoConfirmationRequired, "no pending confirmation for email:%v", id.Email)
	}
	var shouldBeBlocked bool
	var mErr *multierror.Error
	if els.ConfirmationCodeWrongAttemptsCount >= c.cfg.ConfirmationCode.MaxWrongAttemptsCount {
		blockEndTime := time.Now().Add(c.cfg.EmailValidation.BlockDuration)
		blockTimeFitsNow := (els.BlockedUntil.Before(blockEndTime) && els.BlockedUntil.After(*els.CreatedAt.Time))
		if els.BlockedUntil == nil || !blockTimeFitsNow {
			shouldBeBlocked = true
		}
		if !shouldBeBlocked {
			return errors.Wrapf(ErrConfirmationCodeAttemptsExceeded, "confirmation code wrong attempts count exceeded for id:%#v", id)
		}
		mErr = multierror.Append(mErr, errors.Wrapf(ErrConfirmationCodeAttemptsExceeded, "confirmation code wrong attempts count exceeded for id:%#v", id))
	}
	if els.ConfirmationCode != confirmationCode || shouldBeBlocked {
		if els.ConfirmationCodeWrongAttemptsCount+1 >= c.cfg.ConfirmationCode.MaxWrongAttemptsCount {
			shouldBeBlocked = true
		}
		if iErr := c.increaseWrongConfirmationCodeAttemptsCount(ctx, id, shouldBeBlocked); iErr != nil {
			mErr = multierror.Append(mErr, errors.Wrapf(iErr,
				"can't increment wrong confirmation code attempts count for email:%v,deviceUniqueID:%v", id.Email, id.DeviceUniqueID))
		}
		mErr = multierror.Append(mErr, errors.Wrapf(ErrConfirmationCodeWrong, "wrong confirmation code:%v for linkPayload:%v", confirmationCode, emailLinkPayload))

		return mErr.ErrorOrNil() //nolint:wrapcheck // Not needed.
	}

	return nil
}

//nolint:revive // Not to create duplicated function with/without bool flag.
func (c *client) increaseWrongConfirmationCodeAttemptsCount(ctx context.Context, id *loginID, shouldBeBlocked bool) error {
	params := []any{id.Email, id.DeviceUniqueID}
	var blockSQL string
	if shouldBeBlocked {
		blockSQL = ",blocked_until = $3"
		params = append(params, time.Now().Add(c.cfg.EmailValidation.BlockDuration))
	}
	sql := fmt.Sprintf(`UPDATE email_link_sign_ins
				SET confirmation_code_wrong_attempts_count = confirmation_code_wrong_attempts_count + 1
				%v
			WHERE email = $1
				  AND device_unique_id = $2`, blockSQL)
	_, err := storage.Exec(ctx, c.db, sql, params...)

	return errors.Wrapf(err, "can't update email link sign ins for the user with pk:%#v", id)
}

//nolint:revive,funlen // .
func (c *client) finishAuthProcess(
	ctx context.Context,
	id *loginID, userID, otp string, issuedTokenSeq int64,
	emailConfirmed bool, md *users.JSON,
) error {
	emailConfirmedAt := "null"
	if emailConfirmed {
		emailConfirmedAt = "$2"
	}
	mdToUpdate := users.JSON(map[string]any{auth.IceIDClaim: userID})
	if md == nil {
		empty := users.JSON(map[string]any{})
		md = &empty
	}
	if _, hasRegisteredWith := (*md)[auth.RegisteredWithProviderClaim]; !hasRegisteredWith {
		if firebaseID, hasFirebaseID := (*md)[auth.FirebaseIDClaim]; hasFirebaseID {
			if !strings.HasPrefix(firebaseID.(string), iceIDPrefix) && !strings.HasPrefix(userID, iceIDPrefix) { //nolint:forcetypeassert // .
				mdToUpdate[auth.RegisteredWithProviderClaim] = auth.ProviderFirebase
			}
		}
	}
	if err := mergo.Merge(&mdToUpdate, md, mergo.WithOverride, mergo.WithTypeCheck); err != nil {
		return errors.Wrapf(err, "failed to merge %#v and %v:%v", md, auth.IceIDClaim, userID)
	}
	params := []any{id.Email, time.Now().Time, userID, otp, id.DeviceUniqueID, issuedTokenSeq, mdToUpdate}
	sql := fmt.Sprintf(`
			with metadata_update as (
				INSERT INTO account_metadata(user_id, metadata)
				VALUES ($3, $7::jsonb) ON CONFLICT(user_id) DO UPDATE
					SET metadata = EXCLUDED.metadata
				WHERE account_metadata.metadata != EXCLUDED.metadata
			) 
			UPDATE email_link_sign_ins
				SET token_issued_at = $2,
					user_id = $3,
					otp = $3,
					email_confirmed_at = %[1]v,
					issued_token_seq = COALESCE(issued_token_seq, 0) + 1,
					previously_issued_token_seq = COALESCE(issued_token_seq, 0) + 1
			WHERE email_link_sign_ins.email = $1
				  AND otp = $4
				  AND device_unique_id = $5
				  AND issued_token_seq = $6
			`, emailConfirmedAt)

	rowsUpdated, err := storage.Exec(ctx, c.db, sql, params...)
	if err != nil {
		return errors.Wrapf(err, "failed to insert generated token data for:%#v", params...)
	}
	if rowsUpdated == 0 {
		return errors.Wrapf(ErrNoConfirmationRequired, "[finishAuthProcess] No records were updated to finish: race condition")
	}

	return nil
}
