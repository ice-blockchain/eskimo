// SPDX-License-Identifier: ice License 1.0

package device

// Public API.

type (
	UserID = string
	ID     struct {
		UserID         UserID `json:"userId,omitempty" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		DeviceUniqueID string `json:"deviceUniqueId,omitempty" example:"FCDBD8EF-62FC-4ECB-B2F5-92C9E79AC7F9"`
	}
)
