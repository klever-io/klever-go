package middleware

import (
	"crypto/subtle"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/crypto/hashing/factory"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/klever-io/klever-go/network/api/shared"
)

func NewAuthenticationFunc(credentialsConfig config.APIRoutesConfig) gin.HandlerFunc {
	if len(credentialsConfig.Credentials) == 0 {
		return func(c *gin.Context) {
			c.AbortWithStatusJSON(
				http.StatusInternalServerError,
				shared.GenericAPIResponse{
					Data:  nil,
					Error: "no credentials found on server",
					Code:  shared.ReturnCodeInternalError,
				},
			)
		}
	}

	var hasher hashing.Hasher
	var err error
	hasher, err = factory.NewHasher(credentialsConfig.Hasher.Type)
	if err != nil {
		log.Warn("cannot create hasher from config. Will use Sha256 as default", "error", err)
		hasher = sha256.Sha256{} // fallback in case the hasher creation failed
	}

	accounts := gin.Accounts{}
	for _, pair := range credentialsConfig.Credentials {
		accounts[pair.Username] = pair.Password
	}

	// Stand-in digest for an unknown username, the same length as a real one so the
	// comparison below cannot short-circuit on length. No password hashes to it.
	unknownUserDigest := hex.EncodeToString(make([]byte, hasher.Size()))

	authenticationFunction := func(c *gin.Context) {
		user, pass, ok := c.Request.BasicAuth()
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, shared.GenericAPIResponse{
				Data:  nil,
				Error: "this endpoint requires Basic Authentication",
				Code:  shared.ReturnCodeRequestError,
			})
			return
		}

		// An unknown username must cost and answer exactly what a wrong password does.
		// Returning early with a distinct message made usernames enumerable one request
		// at a time, and skipping the hash widened the timing gap on top of that.
		userPassword, userExists := accounts[user]
		if !userExists {
			userPassword = unknownUserDigest
		}

		expected := hex.EncodeToString(hasher.Compute(pass))
		if subtle.ConstantTimeCompare([]byte(userPassword), []byte(expected)) != 1 || !userExists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, shared.GenericAPIResponse{
				Data:  nil,
				Error: "invalid credentials",
				Code:  shared.ReturnCodeRequestError,
			})
			return
		}
		// If authentication passes, allow the request to proceed
		c.Next()
	}

	return authenticationFunction
}
