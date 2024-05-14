package tools

import "time"

// pkPrefixSize specifies the max numbers of chars to be displayed from one publc key
const pkPrefixSize = 12

// PublicKeyBlacklistDuration represents the time to keep a public key in the black list if it will degrade its
// rating to a minimum threshold due to improper messages
const PublicKeyBlacklistDuration = time.Second * 7200
