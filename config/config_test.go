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
    max_wait_millis: 45000
  bar:
    patterns:
     - 'DEF 234'
     - 'XYZ 345'
`

	os.Setenv(varName, yamlBody)
	conf, err := FromVar(varName)
	require.NoError(t, err)

	foo, found := conf.Containers["foo"]
	require.True(t, found)
	require.Equal(t, "ABC 123", foo.Pattern)
	require.Equal(t, 45000, foo.MaxWaitMillis)

	bar, found := conf.Containers["bar"]
	require.True(t, found)
	require.Len(t, bar.Patterns, 2)
	require.Equal(t, "DEF 234", bar.Patterns[0])
	require.Equal(t, "XYZ 345", bar.Patterns[1])

	_, found = conf.Containers["does_not_exist"]
	require.False(t, found)
}

func TestConfigFromFile(t *testing.T) {
	fileName := "whalewatcher.yaml"
	yamlBody := `
containers:
  foo:
    pattern: 'ABC 123'
    max_wait_millis: 30000
  bar:
    patterns:
      - 'DEF 234'
      - 'XYZ 345'
`

	dir, err := ioutil.TempDir("", "wwconf")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, fileName)
	err = ioutil.WriteFile(path, []byte(yamlBody), 0666)
	require.NoError(t, err)

	conf, err := FromFile(path)
	require.NoError(t, err)

	foo, found := conf.Containers["foo"]
	require.True(t, found)
	require.Equal(t, "ABC 123", foo.Pattern)
	require.Equal(t, 30000, foo.MaxWaitMillis)

	bar, found := conf.Containers["bar"]
	require.True(t, found)
	require.Len(t, bar.Patterns, 2)
	require.Equal(t, "DEF 234", bar.Patterns[0])
	require.Equal(t, "XYZ 345", bar.Patterns[1])

	_, found = conf.Containers["does_not_exist"]
	require.False(t, found)
}
