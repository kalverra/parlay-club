package main

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunPrintsAShellSourceableKeyPair(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	require.NoError(t, run(&out))

	values := make(map[string]string)
	for line := range strings.SplitSeq(strings.TrimSpace(out.String()), "\n") {
		name, value, found := strings.Cut(line, "=")
		require.True(t, found, "line %q is KEY=value", line)
		values[name] = value
	}
	require.Len(t, values, 2)

	publicKey, err := base64.RawURLEncoding.DecodeString(values["VAPID_PUBLIC_KEY"])
	require.NoError(t, err)
	assert.Len(t, publicKey, 65, "the public key is an uncompressed P-256 point")
	assert.Equal(t, byte(4), publicKey[0])

	privateKey, err := base64.RawURLEncoding.DecodeString(values["VAPID_PRIVATE_KEY"])
	require.NoError(t, err)
	assert.Len(t, privateKey, 32)
}

func TestRunPrintsANewPairEachTime(t *testing.T) {
	t.Parallel()

	var first, second bytes.Buffer
	require.NoError(t, run(&first))
	require.NoError(t, run(&second))

	assert.NotEqual(t, first.String(), second.String())
}
