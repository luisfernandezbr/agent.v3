package agentconf

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/pinpt/agent.next/pkg/keychain"
	"github.com/pinpt/go-common/fileutil"
	"gopkg.in/yaml.v2"
)

type Opts struct {
	File                string
	NoEncryption        bool
	EncryptionKeyAccess string
}

type Config struct {
	opts Opts

	pinpoint     map[string]interface{}
	integrations map[string]map[string]interface{}

	encryptor *keychain.Encryptor
}

const configFileYaml = ".pinpoint-agent.yaml"
const configFileEncr = ".pinpoint-agent.encr"

func New(opts Opts) (*Config, error) {
	s := &Config{}
	s.opts = opts

	wantEncryption := false
	mayNeedAutoEncrypt := false

	if !opts.NoEncryption {
		if opts.File == "" {
			wantEncryption = true
			mayNeedAutoEncrypt = true
		} else if strings.HasSuffix(opts.File, ".encr") {
			wantEncryption = true
		}
	}

	if wantEncryption {
		err := s.setupEncryptor()
		if err != nil {
			return nil, err
		}
	}
	if mayNeedAutoEncrypt && s.encryptor != nil {
		err := s.autoencryptConfigFileIfNeededInDefaultDir()
		if err != nil {
			return nil, err
		}
	}

	var data []byte

	if s.encryptor != nil {
		loc := opts.File
		if loc == "" {
			dir, _ := homedir.Dir()
			defaultEncr := filepath.Join(dir, configFileEncr)
			loc = defaultEncr
		}
		if !fileutil.FileExists(loc) {
			return nil, fmt.Errorf("config does not exist at loc: %v", loc)
		}
		b, err := s.encryptor.DecryptFile(loc)
		if err != nil {
			return nil, fmt.Errorf("could not decrypt config file at loc: %v %v", loc, err)
		}
		data = []byte(b)
	} else {
		loc := opts.File
		if loc == "" {
			dir, _ := homedir.Dir()
			loc = filepath.Join(dir, configFileYaml)
		}
		if !fileutil.FileExists(loc) {
			return nil, fmt.Errorf("config does not exist at loc: %v", loc)
		}
		b, err := ioutil.ReadFile(loc)
		if err != nil {
			return nil, fmt.Errorf("could not read config file at loc: %v %v", loc, err)
		}
		data = b
	}

	err := s.parseYaml(data)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Config) setupEncryptor() error {
	if s.opts.EncryptionKeyAccess != "" {
		// using key retrieved with config encryption script
		kc, err := keychain.NewCustomKeyChain(s.opts.EncryptionKeyAccess)
		if err != nil {
			return err
		}
		s.encryptor = keychain.NewEncryptor(kc)
		return nil
	}
	// no default encryption on linux
	if runtime.GOOS == "linux" {
		return nil
	}
	kc, err := keychain.NewOSKeyChain()
	if err != nil {
		return err
	}
	s.encryptor = keychain.NewEncryptor(kc)
	return nil
}

func (s *Config) autoencryptConfigFileIfNeededInDefaultDir() error {
	dir, _ := homedir.Dir()

	defaultEncr := filepath.Join(dir, configFileEncr)
	if fileutil.FileExists(defaultEncr) {
		return nil
	}

	defaultYaml := filepath.Join(dir, configFileYaml)
	if !fileutil.FileExists(defaultYaml) {
		// config file does not exist
		return errors.New("config file does not exist at: " + defaultYaml)
	}

	// default yaml file exists, but is not encrypted, auto-encrypt it
	data, err := ioutil.ReadFile(defaultYaml)
	if err != nil {
		return fmt.Errorf("Error reading file %s %v", defaultYaml, err)
	}

	err = s.encryptor.EncryptToFile(string(data), defaultEncr)
	if err != nil {
		return fmt.Errorf("Error encrypting file %v %v", defaultEncr, err)
	}

	err = os.Remove(defaultYaml)
	if err != nil {
		return fmt.Errorf("Error removing old config file %s %v", defaultYaml, err)
	}
	return nil
}

type configFormat struct {
	Services []map[string]interface{} `yaml:"services"`
}

func (s *Config) parseYaml(yamlBytes []byte) error {
	var data configFormat
	err := yaml.Unmarshal(yamlBytes, &data)
	if err != nil {
		return err
	}
	rerr := func(msg string, args ...interface{}) error {
		return fmt.Errorf("config validation error: "+msg, args...)
	}
	if len(data.Services) == 0 {
		return rerr("no services defined")
	}
	var pp map[string]interface{}
	integrations := map[string]map[string]interface{}{}
	for _, ig := range data.Services {
		name, _ := ig["name"].(string)
		if name == "" {
			return rerr("missing required name attribute")
		}
		if name == "pinpoint" {
			if pp != nil {
				return rerr("multiple services have name pinpoint, only one allowed")
			}
			pp = ig
			continue
		}
		_, ok := integrations[name]
		if ok {
			return rerr("multiple service using same name: %v", name)
		}
		disabled, _ := ig["disabled"].(bool)
		if disabled {
			continue
		}
		integrations[name] = ig

	}
	s.pinpoint = pp
	s.integrations = integrations
	return nil
}

func (s *Config) GetEnabledIntegrations() (res []string) {
	for k := range s.integrations {
		res = append(res, k)
	}
	return
}

func (s *Config) IntegrationConfig(integrationName string) (map[string]interface{}, error) {
	return s.integrations[integrationName], nil
}
