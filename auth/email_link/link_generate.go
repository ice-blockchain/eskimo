// SPDX-License-Identifier: ice License 1.0

package emaillink

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/time"
)

func (r *repository) generateLinkPayload(email, otp string, now *time.Time) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, emailClaims{
		RegisteredClaims: &jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   email,
			Audience:  nil,
			ExpiresAt: jwt.NewNumericDate(now.Add(r.cfg.ExpirationTime)),
			NotBefore: jwt.NewNumericDate(*now.Time),
			IssuedAt:  jwt.NewNumericDate(*now.Time),
		},
		OTP: otp,
	})

	payload, err := token.SignedString([]byte(r.cfg.JWTSecret))

	return payload, errors.Wrapf(err, "can't generate link payload for email:%v,otp:%v,now:%v", email, otp, now)
}
