package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigFromVar(t *testing.T) {
	varName := "WHALEWATCHER_CONFIG"
	yamlBody := `
containers:
  foo:
    pattern: 'ABC 123'
  bar:
    pattern: 'DEF 234'
`

	os.Setenv(varName, yamlBody)
	conf, err := FromVar(varName)
	require.NoError(t, err)

	foo, found := conf.Services["foo"]
	require.True(t, found)
	require.Equal(t, "ABC 123", foo.Pattern)

	bar, found := conf.Services["bar"]
	require.True(t, found)
	require.Equal(t, "DEF 234", bar.Pattern)

	_, found = conf.Services["does_not_exist"]
	require.False(t, found)
}

func TestConfigFromFile(t *testing.T) {
	fileName := "whalewatcher.yaml"
	yamlBody := `
containers:
  foo:
    pattern: 'ABC 123'
  bar:
    pattern: 'DEF 234'
`

	dir, err := ioutil.TempDir("", "wwconf")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, fileName)
	err = ioutil.WriteFile(path, []byte(yamlBody), 0666)
	require.NoError(t, err)

	conf, err := FromFile(path)
	require.NoError(t, err)

	foo, found := conf.Services["foo"]
	require.True(t, found)
	require.Equal(t, "ABC 123", foo.Pattern)

	bar, found := conf.Services["bar"]
	require.True(t, found)
	require.Equal(t, "DEF 234", bar.Pattern)

	_, found = conf.Services["does_not_exist"]
	require.False(t, found)
}
