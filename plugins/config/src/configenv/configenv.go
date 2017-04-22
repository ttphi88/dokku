package configenv

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"archive/tar"

	common "github.com/dokku/dokku/plugins/common"
)

//Env is a representation for global or app environment
type Env struct {
	name           string
	env            map[string]string
	filename       string
	EscapeNewlines bool
}

func (e *Env) String() string {
	return e.EnvfileString()
}

//EnvfileString returns the contents of this Env in ENVFILE format
func (e *Env) EnvfileString() string {
	return e.StringWithPrefixAndSeparator("", "\n")
}

//ExportfileString returns the contents of this Env as bash exports
func (e *Env) ExportfileString() string {
	return e.StringWithPrefixAndSeparator("export ", "\n")
}

//StringWithPrefixAndSeparator makes a string of the environment
// with the given prefix and separator for each entry
func (e *Env) StringWithPrefixAndSeparator(prefix string, separator string) string {
	keys := e.Keys()
	entries := make([]string, len(keys))
	for i, k := range keys {
		v := SingleQuoteEscape(e.env[k])
		if e.EscapeNewlines {
			v = strings.Replace(v, "\n", "'$'\\n''", -1)
		}
		entries[i] = fmt.Sprintf("%s%s='%s'", prefix, k, v)
	}
	return strings.Join(entries, separator)
}

//SingleQuoteEscape escapes the value as if it were shell-quoted in single quotes
func SingleQuoteEscape(value string) string { // so that 'esc'apped' -> 'esc'\''aped'
	return strings.Replace(value, "'", "'\\''", -1)
}

//ExportBundle writes a tarfile of the environmnet to the given io.Writer.
// for every environment variable there is a file with the variable's key
// with its content set to the variable's value
func (e *Env) ExportBundle(dest io.Writer) error {
	tarfile := tar.NewWriter(dest)
	defer tarfile.Close()

	for _, k := range e.Keys() {
		val, _ := e.Get(k)
		valbin := []byte(val)

		header := &tar.Header{
			Name: k,
			Mode: 0600,
			Size: int64(len(valbin)),
		}
		tarfile.WriteHeader(header)
		tarfile.Write(valbin)
	}
	return nil
}

//NewFromTarget creates an env from the given target. Target is either "--global" or an app name
func NewFromTarget(target string) (*Env, error) {
	if target == "--global" {
		return LoadGlobal()
	}
	return LoadApp(target)
}

//LoadApp loads an environment for the given app
func LoadApp(appName string) (*Env, error) {
	appfile, err := getAppFile(appName)
	if err != nil {
		return nil, err
	}
	return parseEnv(appName, appfile)
}

//LoadGlobal loads the global environmen
func LoadGlobal() (*Env, error) {
	return parseEnv("global", getGlobalFile())
}

//NewFromString creates an env from the given ENVFILE contents representation
func NewFromString(rep string) (*Env, error) {
	return parseEnvFromReader("<unknown>", "", strings.NewReader(rep))
}

//Merge merges the given environment on top of the reciever
func (e *Env) Merge(other *Env) {
	for _, k := range other.Keys() {
		e.Set(k, other.GetDefault(k, ""))
	}
}

//Set an environment variable
func (e *Env) Set(key string, value string) {
	e.env[key] = value
}

//Unset an environment variable
func (e *Env) Unset(key string) {
	delete(e.env, key)
}

//Keys gets the keys in this environment
func (e *Env) Keys() []string {
	keys := make([]string, 0, len(e.env))
	for k := range e.env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

//Get an environment variable
func (e *Env) Get(key string) (string, bool) {
	v, ok := e.env[key]
	return v, ok
}

//GetDefault an environment variable or a default if it doesnt exist
func (e *Env) GetDefault(key string, defaultValue string) string {
	v, ok := e.env[key]
	if !ok {
		return defaultValue
	}
	return v
}

//GetBoolDefault gets the bool value of the given key with the given default
//right now that is evaluated as `value != "0"`
func (e *Env) GetBoolDefault(key string, defaultValue bool) bool {
	v, ok := e.Get(key)
	if !ok {
		return defaultValue
	}
	return v != "0"
}

//Len return the number of items in this environment
func (e *Env) Len() int {
	return len(e.env)
}

//Map return the Env as a map
func (e *Env) Map() map[string]string {
	return e.env
}

//Write an Env back to the file it was read from as an exportfile
func (e *Env) Write() error {
	if e.filename == "" {
		return errors.New("this Env was created unbound to a file")
	}
	file, err := os.Create(e.filename)
	defer file.Close()
	if err != nil {
		return err
	}
	_, err = file.WriteString(e.ExportfileString())
	return err
}

func getAppFile(appName string) (string, error) {
	err := common.VerifyAppName(appName)
	if err != nil {
		return "", err
	}
	return filepath.Join(common.MustGetEnv("DOKKU_ROOT"), appName, "ENV"), nil
}

func getGlobalFile() string {
	return filepath.Join(common.MustGetEnv("DOKKU_ROOT"), "ENV")
}
