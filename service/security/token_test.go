package security

import (
	"math/rand"
	"os"
	"testing"

	"github.com/danglnh07/TaskManagement/db"
	"github.com/danglnh07/TaskManagement/util"
	"github.com/stretchr/testify/require"
)

var (
	config  *util.Config
	service *JWTService
)

func TestMain(m *testing.M) {
	config = util.LoadConfig("../../.env")
	service = NewJWTService(config)
	os.Exit(m.Run())
}

func TestToken(t *testing.T) {
	// Create test data
	id := uint(rand.Intn(1000))
	role := []db.Role{db.User, db.Admin}[rand.Intn(2)]
	tokenType := []TokenType{AccessToken, RefreshToken}[rand.Intn(2)]
	version := rand.Intn(10)

	// Create token
	token, err := service.CreateToken(id, role, tokenType, version)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Verify token
	result, err := service.VerifyToken(token)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// Compare the test data with the extract claims
	require.Equal(t, id, result.ID)
	require.Equal(t, role, result.Role)
	require.Equal(t, tokenType, result.TokenType)
	require.Equal(t, version, result.Version)
}
