// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/auth"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	time "github.com/ice-blockchain/wintr/time"
)

//nolint:funlen,gocognit // .
func (c *client) RegenerateTokens(ctx context.Context, previousRefreshToken string, customClaims *users.JSON) (tokens *Tokens, err error) {
	token, err := c.authClient.ParseToken(previousRefreshToken)
	if err != nil {
		if errors.Is(err, auth.ErrExpiredToken) {
			return nil, errors.Wrapf(ErrExpiredToken, "failed to verify due to expired token:%v", previousRefreshToken)
		}
		if errors.Is(err, auth.ErrInvalidToken) {
			return nil, errors.Wrapf(ErrInvalidToken, "failed to verify due to invalid token:%v", previousRefreshToken)
		}

		return nil, errors.Wrapf(err, "failed to verify token:%v", previousRefreshToken)
	}
	id := loginID{Email: token.Email, DeviceUniqueID: token.DeviceUniqueID}
	usr, err := c.getUserByIDOrPk(ctx, token.Subject, &id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, errors.Wrapf(ErrUserNotFound, "user with userID:%v or email:%v not found", token.Subject, token.Email)
		}

		return nil, errors.Wrapf(err, "failed to get user by userID:%v", token.Subject)
	}
	if usr.Email != token.Email || usr.DeviceUniqueID != token.DeviceUniqueID {
		return nil, errors.Wrapf(ErrUserDataMismatch,
			"user's email:%v does not match token's email:%v or deviceID:%v", usr.Email, token.Email, token.DeviceUniqueID)
	}
	now := time.Now()
	refreshTokenSeq, mergedCustomClaims, err := c.incrementRefreshTokenSeq(ctx, &id, token.Subject, token.Seq, now, customClaims)
	if err != nil {
		if storage.IsErr(err, storage.ErrNotFound) {
			return nil, errors.Wrapf(ErrInvalidToken, "refreshToken with wrong sequence:%v provided", token.Seq)
		}

		return nil, errors.Wrapf(err, "failed to update email link sign ins for email:%v", token.Email)
	}
	usr.CustomClaims = mergedCustomClaims
	tokens, err = c.generateTokens(now, usr, refreshTokenSeq)

	return tokens, errors.Wrapf(err, "can't generate tokens for userID:%v, email:%v", token.Subject, token.Email)
}

//nolint:revive // .
func (c *client) incrementRefreshTokenSeq(
	ctx context.Context,
	id *loginID,
	userID string,
	currentSeq int64,
	now *time.Time,
	customClaims *users.JSON,
) (tokenSeq int64, mergedCustomClaims *users.JSON, err error) {
	params := []any{id.Email, id.DeviceUniqueID, now.Time, userID, currentSeq}
	customClaimsClause := ""
	if customClaims != nil {
		params = append(params, customClaims)
		customClaimsClause = ",\n\t\t\t\tcustom_claims = (COALESCE(email_link_sign_ins.custom_claims,'{}'::jsonb)||$6::jsonb)"
	}
	type resp struct {
		IssuedTokenSeq int64
		CustomClaims   *users.JSON
	}
	sql := fmt.Sprintf(`
		UPDATE email_link_sign_ins
			SET token_issued_at = $3,
				user_id = $4,
				issued_token_seq = COALESCE(email_link_sign_ins.issued_token_seq, 0) + 1
				%v
			WHERE  (email_link_sign_ins.email = $1 AND email_link_sign_ins.device_unique_id = $2) 
				   AND (email_link_sign_ins.user_id = $4 AND email_link_sign_ins.issued_token_seq = $5)
			RETURNING issued_token_seq, custom_claims`, customClaimsClause)
	updatedValue, err := storage.ExecOne[resp](ctx, c.db, sql, params...)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "failed to assign refreshed token to email link sign ins for params:%#v", params) //nolint:asasalint // Not this output.
	}

	return updatedValue.IssuedTokenSeq, updatedValue.CustomClaims, nil
}

func (c *client) generateTokens(now *time.Time, els *emailLinkSignIn, seq int64) (tokens *Tokens, err error) {
	var claims map[string]any
	if els.CustomClaims != nil {
		claims = *els.CustomClaims
	}
	refreshToken, accessToken, err := c.authClient.GenerateTokens(now, *els.UserID, els.DeviceUniqueID, els.Email, els.HashCode, seq, claims)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate tokens for user:%#v", els)
	}

	return &Tokens{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}
