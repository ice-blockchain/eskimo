// SPDX-License-Identifier: BUSL-1.1

package device

// Public API.

type (
	UserID = string
	ID     struct {
		//nolint:unused,revive,tagliatelle,nosnakecase // Because it is used by the msgpack library for marshalling/unmarshalling.
		_msgpack       struct{} `msgpack:",asArray"`
		UserID         UserID   `json:"userId" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		DeviceUniqueID string   `json:"deviceUniqueId" example:"FCDBD8EF-62FC-4ECB-B2F5-92C9E79AC7F9"`
	}
)
