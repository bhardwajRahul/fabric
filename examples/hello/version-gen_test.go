// Code generated by Microbus. DO NOT EDIT.

package hello

import (
	"os"
	"testing"

	"github.com/microbus-io/fabric/utils"
	"github.com/stretchr/testify/assert"
)

func TestHello_Versioning(t *testing.T) {
	t.Parallel()
	
	hash, err := utils.SourceCodeSHA256(".")
	if assert.NoError(t, err) {
		assert.Equal(t, hash, SourceCodeSHA256, "SourceCodeSHA256 is not up to date")
	}
	buf, err := os.ReadFile("version-gen.go")
	if assert.NoError(t, err) {
		assert.Contains(t, string(buf), hash, "SHA256 in version-gen.go is not up to date")
	}
}
