package keychain

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testChain struct {
	v string
}

func newTestChain() *testChain {
	return &testChain{}
}

func (s *testChain) Get() (programKey string, _ error) {
	return s.v, nil
}

func (s *testChain) Set(programKey string) error {
	s.v = programKey
	return nil
}

func TestEncryption(t *testing.T) {
	enc := NewEncryptor(newTestChain())

	content := "This is test content"

	tmpFolder, err := ioutil.TempDir("", "ut")
	assert.NoError(t, err)

	encryptedFile, err := ioutil.TempFile(tmpFolder, "")
	assert.NoError(t, err)

	err = enc.EncryptToFile(content, encryptedFile.Name())
	assert.NoError(t, err)

	decryptedContent, err := enc.DecryptFile(encryptedFile.Name())
	assert.NoError(t, err)

	assert.Equal(t, decryptedContent, content)
}
