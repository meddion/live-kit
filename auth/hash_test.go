package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHash(t *testing.T) {
	target := "$2a$10$NafPEaNyDop3fTfc08Pd0.EVBdwizUKTAGa0W.EPavalWiZ2Zk09."
	resHash, err := Hash("bob-secret")
	require.NoError(t, err)
	require.Equal(t, target, resHash)
}
