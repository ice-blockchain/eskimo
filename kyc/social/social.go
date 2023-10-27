// SPDX-License-Identifier: ice License 1.0

package social

import (
	social "github.com/ice-blockchain/eskimo/kyc/social/internal"
)

func New(s Type) Verifier {
	return social.New(s)
}
