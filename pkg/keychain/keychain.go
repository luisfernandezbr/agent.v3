package keychain

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"

	"github.com/99designs/keyring"

	pauth "github.com/pinpt/go-common/auth"
)

type Encryptor struct {
	KeyChain  KeyChain
	keyCached string
}

func NewEncryptor(keychain KeyChain) *Encryptor {
	s := &Encryptor{}
	s.KeyChain = keychain
	return s
}
func (s *Encryptor) getKey() (string, error) {
	if s == nil {
		return "", errors.New("trying to call getKey when Encryptor is nil")
	}
	if s.keyCached != "" {
		return s.keyCached, nil
	}
	res, err := s.getKeyUncached()
	if err != nil {
		return "", err
	}
	s.keyCached = res
	return res, nil
}

func (s *Encryptor) getKeyUncached() (string, error) {
	res, err := s.KeyChain.Get()
	if err != nil {
		return "", err
	}
	if res == "" {
		// no key, create it
		b, err := randBytes(32)
		if err != nil {
			return "", err
		}
		key := hex.EncodeToString(b)
		err = s.KeyChain.Set(key)
		if err != nil {
			return "", err
		}
		return key, nil
	}
	return res, nil
}

func randBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

// EncryptToFile will encrypt the contents to outputfile
func (s *Encryptor) EncryptToFile(content string, outputFile string) error {
	key, err := s.getKey()
	if err != nil {
		return err
	}
	contentEncrypted, err := pauth.EncryptString(content, key)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(outputFile, []byte(contentEncrypted), 0755)
}

// DecryptFile returns decrypted content of file.
func (s *Encryptor) DecryptFile(srcFile string) (string, error) {
	key, err := s.getKey()
	if err != nil {
		return "", err
	}
	ciphertext, err := ioutil.ReadFile(srcFile)
	if err != nil {
		return "", err
	}
	plaintext, err := pauth.DecryptString(string(ciphertext), key)
	if err != nil {
		return "", err
	}
	return plaintext, nil
}

type KeyChain interface {
	Get() (programKey string, _ error)
	Set(programKey string) error
}

type OSKeyChain struct {
	kr keyring.Keyring
}

const encrKeyName = "enckey"

func NewOSKeyChain() (*OSKeyChain, error) {
	home, _ := homedir.Dir()
	if home == "" {
		return nil, errors.New("can't get homedir")
	}

	kr, err := keyring.Open(keyring.Config{
		ServiceName:                    "Pinpoint Agent",
		KeychainTrustApplication:       true,
		KeychainSynchronizable:         false,
		KeychainAccessibleWhenUnlocked: false,
		//FileDir:                        home,
		//FilePasswordFunc:               terminalPrompt,
	})

	if err != nil {
		return nil, err
	}

	return &OSKeyChain{kr: kr}, nil
}

/*
func terminalPrompt(prompt string) (string, error) {
	fmt.Printf("%s: ", prompt)
	b, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	fmt.Println()
	return string(b), nil
}
*/

func (s *OSKeyChain) Get() (programKey string, _ error) {
	v, err := s.kr.Get(encrKeyName)
	if err != nil {
		if err == keyring.ErrKeyNotFound {
			return "", nil
		}
		return "", err
	}
	return string(v.Data), nil
}

func (s *OSKeyChain) Set(programKey string) error {
	return s.kr.Set(keyring.Item{
		Key:  encrKeyName,
		Data: []byte(programKey),
	})
}

type CustomKeyChain struct {
	externalScript string
}

func NewCustomKeyChain(externalScript string) (*CustomKeyChain, error) {
	return &CustomKeyChain{externalScript: externalScript}, nil
}

func (s *CustomKeyChain) Get() (programKey string, _ error) {
	return runCommand(s.externalScript, []string{"get"})
}

func (s *CustomKeyChain) Set(programKey string) error {
	_, err := runCommand(s.externalScript, []string{"set", programKey})
	return err
}

func runCommand(script string, args []string) (res string, _ error) {
	if len(script) == 0 {
		return "", errors.New("empty command passed")
	}
	parts := strings.Split(script, " ")
	if len(parts[0]) == 0 {
		return "", errors.New("could not split using space")
	}
	cmdStr := parts[0]
	if parts[0][0] == '.' {
		var err error
		cmdStr, err = filepath.Abs(parts[0])
		if err != nil {
			return "", err
		}
	}

	args2 := parts[1:]
	args2 = append(args2, args...)
	cmd := exec.Command(cmdStr, args2...)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to run external config encryption key integration err: %v command output: %v cmd: %v %v", err, stderr.String(), cmdStr, args2)
	}
	return strings.TrimSpace(stdout.String()), nil
}
