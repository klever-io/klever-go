package middleware_test

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/klever-io/klever-go/network/api/middleware"
	"github.com/stretchr/testify/assert"
)

func createAuth(user string, pass string) gin.HandlerFunc {
	hasher := sha256.Sha256{}
	// hash password
	hashedPass := hasher.Compute(pass)

	return middleware.NewAuthenticationFunc(config.APIRoutesConfig{
		Credentials: []config.Credential{
			{
				Username: user,
				Password: hex.EncodeToString(hashedPass),
			},
		},
		Hasher: config.TypeConfig{
			Type: "sha256",
		},
	})
}

func TestNoCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authFunc := createAuth("test", "test")
	r := gin.New()
	r.Use(authFunc)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.JSONEq(t, `{"data": null, "error": "this endpoint requires Basic Authentication", "code": "bad_request"}`, w.Body.String())
}

func TestIncorrectPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authFunc := createAuth("test", "test")
	r := gin.New()
	r.Use(authFunc)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.SetBasicAuth("test", "wrongpassword")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.JSONEq(t, `{"data": null, "error": "invalid credentials", "code": "bad_request"}`, w.Body.String())

}

func TestIncorrectUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authFunc := createAuth("test", "test")
	r := gin.New()
	r.Use(authFunc)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.SetBasicAuth("another_user", "wrongpassword")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.JSONEq(t, `{"data": null, "error": "invalid credentials", "code": "bad_request"}`, w.Body.String())
}

func TestSuccessfulAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authFunc := createAuth("test", "test")

	r := gin.New()
	r.Use(authFunc)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.SetBasicAuth("test", "test")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"message": "success"}`, w.Body.String())
}

// An unknown username and a wrong password must be indistinguishable from outside: a
// distinct "username does not exist" body made usernames enumerable one request at a
// time. Pins the two responses as byte-identical, not merely both 401.
func TestUnknownUserIsIndistinguishableFromWrongPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authFunc := createAuth("test", "test")

	r := gin.New()
	r.Use(authFunc)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	respond := func(user, pass string) (int, string) {
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.SetBasicAuth(user, pass)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}

	unknownUserCode, unknownUserBody := respond("another_user", "wrongpassword")
	wrongPassCode, wrongPassBody := respond("test", "wrongpassword")

	assert.Equal(t, http.StatusUnauthorized, unknownUserCode)
	assert.Equal(t, wrongPassCode, unknownUserCode)
	assert.Equal(t, wrongPassBody, unknownUserBody,
		"an unknown username must not be distinguishable from a wrong password")
}
