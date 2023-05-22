// SPDX-License-Identifier: ice License 1.0

package emaillink

import (
	"github.com/golang-jwt/jwt/v5"

	"github.com/ice-blockchain/wintr/time"
)

func (r *repository) generateLinkPayload(email, otp string, now time.Time) (string, error) {
	token := jwt.NewWithClaims(&jwt.SigningMethodHMAC{}, emailClaims{
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

	return token.SignedString([]byte(r.cfg.JWTSecret))
}
