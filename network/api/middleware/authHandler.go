package middleware

import (
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
		log.Warn("authentication requested but no credentials are configured; secured endpoints will reject all requests")
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

		userPassword, ok := accounts[user]
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, shared.GenericAPIResponse{
				Data:  nil,
				Error: "username does not exist",
				Code:  shared.ReturnCodeRequestError,
			})
			return
		}

		if userPassword != hex.EncodeToString(hasher.Compute(pass)) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, shared.GenericAPIResponse{
				Data:  nil,
				Error: "invalid password",
				Code:  shared.ReturnCodeRequestError,
			})
			return
		}
		// If authentication passes, allow the request to proceed
		c.Next()
	}

	return authenticationFunction
}
