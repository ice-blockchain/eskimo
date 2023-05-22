// Code generated by swaggo/swag. DO NOT EDIT.

package api

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {
            "name": "ice.io",
            "url": "https://ice.io"
        },
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/auth/finish/{payload}": {
            "get": {
                "description": "Finishes login flow using magic link",
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Auth"
                ],
                "parameters": [
                    {
                        "type": "string",
                        "description": "Request params",
                        "name": "payload",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/main.RefreshedToken"
                        }
                    },
                    "400": {
                        "description": "if invalid or expired payload provided",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "404": {
                        "description": "if email does not need to be confirmed by magic link",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "422": {
                        "description": "if syntax fails",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "504": {
                        "description": "if request times out",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    }
                }
            }
        },
        "/users": {
            "post": {
                "description": "Creates an user account",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Accounts"
                ],
                "parameters": [
                    {
                        "type": "string",
                        "default": "Bearer \u003cAdd access token here\u003e",
                        "description": "Insert your access token",
                        "name": "Authorization",
                        "in": "header",
                        "required": true
                    },
                    {
                        "type": "string",
                        "default": "1.1.1.1",
                        "description": "Client IP",
                        "name": "X-Forwarded-For",
                        "in": "header"
                    },
                    {
                        "description": "Request params",
                        "name": "request",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/main.CreateUserRequestBody"
                        }
                    }
                ],
                "responses": {
                    "201": {
                        "description": "Created",
                        "schema": {
                            "$ref": "#/definitions/main.User"
                        }
                    },
                    "400": {
                        "description": "if validations fail",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "401": {
                        "description": "if not authorized",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "409": {
                        "description": "user already exists with that ID, email or phone number",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "422": {
                        "description": "if syntax fails",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "504": {
                        "description": "if request times out",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    }
                }
            }
        },
        "/users/{userId}": {
            "delete": {
                "description": "Deletes an user account",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Accounts"
                ],
                "parameters": [
                    {
                        "type": "string",
                        "default": "Bearer \u003cAdd access token here\u003e",
                        "description": "Insert your access token",
                        "name": "Authorization",
                        "in": "header",
                        "required": true
                    },
                    {
                        "type": "string",
                        "description": "ID of the User",
                        "name": "userId",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK - found and deleted"
                    },
                    "204": {
                        "description": "No Content - already deleted"
                    },
                    "400": {
                        "description": "if validations fail",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "401": {
                        "description": "if not authorized",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "403": {
                        "description": "not allowed",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "422": {
                        "description": "if syntax fails",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "504": {
                        "description": "if request times out",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    }
                }
            },
            "patch": {
                "description": "Modifies an user account",
                "consumes": [
                    "multipart/form-data"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Accounts"
                ],
                "parameters": [
                    {
                        "type": "string",
                        "default": "Bearer \u003cAdd access token here\u003e",
                        "description": "Insert your access token",
                        "name": "Authorization",
                        "in": "header",
                        "required": true
                    },
                    {
                        "type": "string",
                        "description": "ID of the user",
                        "name": "userId",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "string",
                        "description": "Optional. Example:` + "`" + `Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2` + "`" + `.",
                        "name": "agendaPhoneNumberHashes",
                        "in": "formData"
                    },
                    {
                        "type": "string",
                        "description": "Optional. Example:` + "`" + `some hash` + "`" + `.",
                        "name": "blockchainAccountAddress",
                        "in": "formData"
                    },
                    {
                        "type": "string",
                        "description": "Optional. Example:` + "`" + `1232412415326543647657` + "`" + `.",
                        "name": "checksum",
                        "in": "formData"
                    },
                    {
                        "type": "string",
                        "description": "Optional. Example:` + "`" + `New York` + "`" + `.",
                        "name": "city",
                        "in": "formData"
                    },
                    {
                        "type": "boolean",
                        "name": "clearHiddenProfileElements",
                        "in": "formData"
                    },
                    {
                        "type": "string",
                        "description": "Optional. Example: ` + "`" + `{\"key1\":{\"something\":\"somethingElse\"},\"key2\":\"value\"}` + "`" + `.",
                        "name": "clientData",
                        "in": "formData"
                    },
                    {
                        "type": "string",
                        "description": "Optional. Example:` + "`" + `US` + "`" + `.",
                        "name": "country",
                        "in": "formData"
                    },
                    {
                        "type": "string",
                        "description": "Optional. Example:` + "`" + `jdoe@gmail.com` + "`" + `.",
                        "name": "email",
                        "in": "formData"
                    },
                    {
                        "type": "string",
                        "description": "Optional. Example:` + "`" + `John` + "`" + `.",
                        "name": "firstName",
                        "in": "formData"
                    },
                    {
                        "type": "array",
                        "items": {
                            "enum": [
                                "globalRank",
                                "referralCount",
                                "level",
                                "role",
                                "badges"
                            ],
                            "type": "string"
                        },
                        "collectionFormat": "multi",
                        "description": "Optional. Example: Array of [` + "`" + `globalRank` + "`" + `,` + "`" + `referralCount` + "`" + `,` + "`" + `level` + "`" + `,` + "`" + `role` + "`" + `,` + "`" + `badges` + "`" + `].",
                        "name": "hiddenProfileElements",
                        "in": "formData"
                    },
                    {
                        "type": "string",
                        "description": "Optional. Example:` + "`" + `en` + "`" + `.",
                        "name": "language",
                        "in": "formData"
                    },
                    {
                        "type": "string",
                        "description": "Optional. Example:` + "`" + `Doe` + "`" + `.",
                        "name": "lastName",
                        "in": "formData"
                    },
                    {
                        "type": "string",
                        "description": "Optional. Example:` + "`" + `+12099216581` + "`" + `.",
                        "name": "phoneNumber",
                        "in": "formData"
                    },
                    {
                        "type": "string",
                        "description": "Optional. Required only if ` + "`" + `phoneNumber` + "`" + ` is set. Example:` + "`" + `Ef86A6021afCDe5673511376B2` + "`" + `.",
                        "name": "phoneNumberHash",
                        "in": "formData"
                    },
                    {
                        "type": "string",
                        "description": "Optional. Example:` + "`" + `did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2` + "`" + `.",
                        "name": "referredBy",
                        "in": "formData"
                    },
                    {
                        "type": "boolean",
                        "description": "Optional. Example:` + "`" + `true` + "`" + `.",
                        "name": "resetProfilePicture",
                        "in": "formData"
                    },
                    {
                        "type": "string",
                        "description": "Optional. Example:` + "`" + `jdoe` + "`" + `.",
                        "name": "username",
                        "in": "formData"
                    },
                    {
                        "type": "file",
                        "description": "The new profile picture for the user",
                        "name": "profilePicture",
                        "in": "formData"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/main.User"
                        }
                    },
                    "400": {
                        "description": "if validations fail",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "401": {
                        "description": "if not authorized",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "403": {
                        "description": "not allowed",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "404": {
                        "description": "user is not found; or the referred by is not found",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "409": {
                        "description": "if username, email or phoneNumber conflict with another other user's",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "422": {
                        "description": "if syntax fails",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "504": {
                        "description": "if request times out",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    }
                }
            }
        },
        "/users/{userId}/devices/{deviceUniqueId}/metadata": {
            "put": {
                "description": "Replaces existing device metadata with the provided one.",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Devices"
                ],
                "parameters": [
                    {
                        "type": "string",
                        "default": "Bearer \u003cAdd access token here\u003e",
                        "description": "Insert your access token",
                        "name": "Authorization",
                        "in": "header",
                        "required": true
                    },
                    {
                        "type": "string",
                        "default": "1.1.1.1",
                        "description": "Client IP",
                        "name": "X-Forwarded-For",
                        "in": "header"
                    },
                    {
                        "type": "string",
                        "description": "ID of the user",
                        "name": "userId",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "string",
                        "description": "ID of the device",
                        "name": "deviceUniqueId",
                        "in": "path",
                        "required": true
                    },
                    {
                        "description": "Request params",
                        "name": "request",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/main.ReplaceDeviceMetadataRequestBody"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK"
                    },
                    "400": {
                        "description": "if validations fail",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "401": {
                        "description": "if not authorized",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "403": {
                        "description": "if not allowed",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "404": {
                        "description": "if user not found",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "422": {
                        "description": "if syntax fails",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "504": {
                        "description": "if request times out",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    }
                }
            }
        },
        "/users/{userId}/devices/{deviceUniqueId}/metadata/location": {
            "put": {
                "description": "Returns the device's geolocation based on its IP or based on account information if userId is also provided.",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Devices"
                ],
                "parameters": [
                    {
                        "type": "string",
                        "default": "Bearer \u003cAdd access token here\u003e",
                        "description": "Insert your access token. Required only if userId is set",
                        "name": "Authorization",
                        "in": "header"
                    },
                    {
                        "type": "string",
                        "default": "1.1.1.1",
                        "description": "Client IP",
                        "name": "X-Forwarded-For",
                        "in": "header"
                    },
                    {
                        "type": "string",
                        "description": "ID of the user. Is optional, set an ` + "`" + `-` + "`" + ` if none.",
                        "name": "userId",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "string",
                        "description": "ID of the device. Is optional, set an ` + "`" + `-` + "`" + ` if none.",
                        "name": "deviceUniqueId",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/users.DeviceLocation"
                        }
                    },
                    "400": {
                        "description": "if validations fail",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "401": {
                        "description": "if not authenticated",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "403": {
                        "description": "if not allowed",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "422": {
                        "description": "if syntax fails",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    },
                    "504": {
                        "description": "if request times out",
                        "schema": {
                            "$ref": "#/definitions/server.ErrorResponse"
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "main.CreateUserRequestBody": {
            "type": "object",
            "properties": {
                "clientData": {
                    "description": "Optional. Example: ` + "`" + `{\"key1\":{\"something\":\"somethingElse\"},\"key2\":\"value\"}` + "`" + `.",
                    "allOf": [
                        {
                            "$ref": "#/definitions/users.JSON"
                        }
                    ]
                },
                "email": {
                    "description": "Optional.",
                    "type": "string",
                    "example": "jdoe@gmail.com"
                },
                "firstName": {
                    "description": "Optional.",
                    "type": "string",
                    "example": "John"
                },
                "language": {
                    "description": "Optional. Defaults to ` + "`" + `en` + "`" + `.",
                    "type": "string",
                    "example": "en"
                },
                "lastName": {
                    "description": "Optional.",
                    "type": "string",
                    "example": "Doe"
                },
                "phoneNumber": {
                    "description": "Optional.",
                    "type": "string",
                    "example": "+12099216581"
                },
                "phoneNumberHash": {
                    "description": "Optional. Required only if ` + "`" + `phoneNumber` + "`" + ` is set.",
                    "type": "string",
                    "example": "Ef86A6021afCDe5673511376B2"
                }
            }
        },
        "main.RefreshedToken": {
            "type": "object",
            "properties": {
                "accessToken": {
                    "type": "string",
                    "example": ""
                },
                "refreshToken": {
                    "type": "string",
                    "example": ""
                }
            }
        },
        "main.ReplaceDeviceMetadataRequestBody": {
            "type": "object",
            "properties": {
                "apiLevel": {
                    "type": "integer"
                },
                "baseOs": {
                    "type": "string"
                },
                "bootloader": {
                    "type": "string"
                },
                "brand": {
                    "type": "string"
                },
                "buildId": {
                    "type": "string"
                },
                "carrier": {
                    "type": "string"
                },
                "codename": {
                    "type": "string"
                },
                "device": {
                    "type": "string"
                },
                "deviceId": {
                    "type": "string"
                },
                "deviceName": {
                    "type": "string"
                },
                "deviceType": {
                    "type": "string"
                },
                "deviceUniqueId": {
                    "type": "string",
                    "example": "FCDBD8EF-62FC-4ECB-B2F5-92C9E79AC7F9"
                },
                "emulator": {
                    "type": "boolean"
                },
                "fingerprint": {
                    "type": "string"
                },
                "firstInstallTime": {
                    "type": "integer"
                },
                "hardware": {
                    "type": "string"
                },
                "installerPackageName": {
                    "type": "string"
                },
                "instanceId": {
                    "type": "string"
                },
                "lastUpdateTime": {
                    "type": "integer"
                },
                "manufacturer": {
                    "type": "string"
                },
                "pinOrFingerprintSet": {
                    "type": "boolean"
                },
                "product": {
                    "type": "string"
                },
                "pushNotificationToken": {
                    "type": "string"
                },
                "readableVersion": {
                    "type": "string"
                },
                "systemName": {
                    "type": "string"
                },
                "systemVersion": {
                    "type": "string"
                },
                "tablet": {
                    "type": "boolean"
                },
                "tags": {
                    "type": "string"
                },
                "type": {
                    "type": "string"
                },
                "tz": {
                    "type": "string"
                },
                "updatedAt": {
                    "description": "Read Only.",
                    "type": "string"
                },
                "userAgent": {
                    "type": "string"
                },
                "userId": {
                    "type": "string",
                    "example": "did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"
                }
            }
        },
        "main.User": {
            "type": "object",
            "properties": {
                "agendaPhoneNumberHashes": {
                    "type": "string",
                    "example": "Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2"
                },
                "blockchainAccountAddress": {
                    "type": "string",
                    "example": "0x4B73C58370AEfcEf86A6021afCDe5673511376B2"
                },
                "checksum": {
                    "type": "string",
                    "example": "1232412415326543647657"
                },
                "city": {
                    "type": "string",
                    "example": "New York"
                },
                "clientData": {
                    "$ref": "#/definitions/users.JSON"
                },
                "country": {
                    "type": "string",
                    "example": "US"
                },
                "createdAt": {
                    "type": "string",
                    "example": "2022-01-03T16:20:52.156534Z"
                },
                "email": {
                    "type": "string",
                    "example": "jdoe@gmail.com"
                },
                "firstName": {
                    "type": "string",
                    "example": "John"
                },
                "hiddenProfileElements": {
                    "type": "array",
                    "items": {
                        "type": "string",
                        "enum": [
                            "globalRank",
                            "referralCount",
                            "level",
                            "role",
                            "badges"
                        ]
                    },
                    "example": [
                        "level"
                    ]
                },
                "id": {
                    "type": "string",
                    "example": "did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"
                },
                "language": {
                    "type": "string",
                    "example": "en"
                },
                "lastName": {
                    "type": "string",
                    "example": "Doe"
                },
                "miningBlockchainAccountAddress": {
                    "type": "string",
                    "example": "0x4B73C58370AEfcEf86A6021afCDe5673511376B2"
                },
                "phoneNumber": {
                    "type": "string",
                    "example": "+12099216581"
                },
                "profilePictureUrl": {
                    "type": "string",
                    "example": "https://somecdn.com/p1.jpg"
                },
                "referredBy": {
                    "type": "string",
                    "example": "did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"
                },
                "updatedAt": {
                    "type": "string",
                    "example": "2022-01-03T16:20:52.156534Z"
                },
                "username": {
                    "type": "string",
                    "example": "jdoe"
                }
            }
        },
        "server.ErrorResponse": {
            "type": "object",
            "properties": {
                "code": {
                    "type": "string",
                    "example": "SOMETHING_NOT_FOUND"
                },
                "data": {
                    "type": "object",
                    "additionalProperties": {}
                },
                "error": {
                    "type": "string",
                    "example": "something is missing"
                }
            }
        },
        "users.DeviceLocation": {
            "type": "object",
            "properties": {
                "city": {
                    "type": "string",
                    "example": "New York"
                },
                "country": {
                    "type": "string",
                    "example": "US"
                }
            }
        },
        "users.JSON": {
            "type": "object",
            "additionalProperties": {}
        }
    }
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
	Version:          "latest",
	Host:             "",
	BasePath:         "/v1w",
	Schemes:          []string{"https"},
	Title:            "User Accounts, User Devices, User Statistics API",
	Description:      "API that handles everything related to write only operations for user's account, user's devices and statistics about those.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
	LeftDelim:        "{{",
	RightDelim:       "}}",
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
