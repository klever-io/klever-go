package tools

import "time"

// FileModeUserReadWrite represents the permission for a file which allows the user for reading and writing
const FileModeUserReadWrite = 0600

// pkPrefixSize specifies the max numbers of chars to be displayed from one publc key
const pkPrefixSize = 12

// PublicKeyBlacklistDuration represents the time to keep a public key in the black list if it will degrade its
// rating to a minimum threshold due to improper messages
const PublicKeyBlacklistDuration = time.Second * 7200
