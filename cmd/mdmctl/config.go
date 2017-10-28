package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"crypto/tls"
	"github.com/pkg/errors"
	"net/http"
)

type configCommand struct {
	config *ClientConfig
}

// skipVerifyHTTPClient returns an *http.Client with InsecureSkipVerify set
// to true for its TLS config. This allows self-signed SSL certificates.
func skipVerifyHTTPClient(skipVerify bool) *http.Client {
	if skipVerify {
		tlsConfig := &tls.Config{InsecureSkipVerify: true}
		transport := &http.Transport{TLSClientConfig: tlsConfig}
		return &http.Client{Transport: transport}
	}
	return http.DefaultClient
}

func (cmd *configCommand) Run(args []string) error {
	if len(args) < 1 {
		cmd.Usage()
		os.Exit(1)
	}

	var run func([]string) error
	switch strings.ToLower(args[0]) {
	case "set":
		run = setCmd
	case "switch":
		run = switchCmd
	case "print":
		printConfig()
		return nil
	default:
		cmd.Usage()
		os.Exit(1)
	}

	return run(args[1:])
}

func printConfig() {
	path, err := clientConfigPath()
	if err != nil {
		log.Fatal(err)
	}
	cfgData, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(cfgData))
}

func (cmd *configCommand) Usage() error {
	const help = `
mdmctl config print
mdmctl config set -h
`
	fmt.Println(help)
	return nil
}

func setCmd(args []string) error {
	flagset := flag.NewFlagSet("set", flag.ExitOnError)
	var (
		flName       = flagset.String("name", "", "name of the server")
		flToken      = flagset.String("api-token", "", "api token to connect to micromdm server")
		flServerURL  = flagset.String("server-url", "", "server url of micromdm server")
		flSkipVerify = flagset.Bool("skip-verify", false, "skip verification of server certificate (insecure)")
	)

	flagset.Usage = usageFor(flagset, "mdmctl config set [flags]")
	if err := flagset.Parse(args); err != nil {
		return err
	}

	cfg := new(ServerConfig)

	if *flToken != "" {
		cfg.APIToken = *flToken
	}

	validatedURL, err := validateServerURL(*flServerURL)
	if err != nil {
		return err
	}
	cfg.ServerURL = validatedURL

	cfg.SkipVerify = *flSkipVerify

	return saveServerConfig(cfg, *flName)
}

func switchCmd(args []string) error {
	flagset := flag.NewFlagSet("set", flag.ExitOnError)
	var (
		flName = flagset.String("name", "", "name of the server to switch to")
	)

	flagset.Usage = usageFor(flagset, "mdmctl config switch [flags]")
	if err := flagset.Parse(args); err != nil {
		return err
	}

	return SwitchServerConfig(*flName)
}

func validateServerURL(serverURL string) (string, error) {
	if serverURL != "" {
		if !(strings.HasPrefix(serverURL, "http") ||
			strings.HasPrefix(serverURL, "https")) {
			serverURL = "https://" + serverURL
		}
		u, err := url.Parse(serverURL)
		if err != nil {
			return "", err
		}
		u.Path = "/"
		serverURL = u.String()
	}
	return serverURL, nil

}

func clientConfigPath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(usr.HomeDir, ".micromdm", "servers.json"), err
}

func SaveClientConfig(clientCfg *ClientConfig) error {
	configPath, err := clientConfigPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Dir(configPath)); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(configPath), 0777); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(clientCfg)
}

func saveServerConfig(cfg *ServerConfig, name string) error {
	clientCfg, err := LoadClientConfig()
	if err != nil {
		if os.IsNotExist(errors.Cause(err)) {
			clientCfg = new(ClientConfig)
			clientCfg.Servers = make(map[string]ServerConfig)
		} else {
			return err
		}
	}
	fmt.Println(clientCfg)
	if cfg == nil {
		cfg = new(ServerConfig)
	}
	clientCfg.Servers[name] = *cfg
	return SaveClientConfig(clientCfg)
}

func SwitchServerConfig(name string) error {
	clientCfg, err := LoadClientConfig()
	if err != nil {
		return err
	}
	clientCfg.Active = name
	return SaveClientConfig(clientCfg)
}

func LoadClientConfig() (*ClientConfig, error) {
	path, err := clientConfigPath()
	if err != nil {
		return nil, err
	}
	cfgData, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "unable to load default config file")
	}
	var cfg ClientConfig
	err = json.Unmarshal(cfgData, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s : %s", path, err)
	}
	return &cfg, nil
}

func LoadServerConfig() (*ServerConfig, error) {
	cfg, err := LoadClientConfig()
	if err != nil {
		return nil, err
	}
	var serverCfg ServerConfig
	serverCfg = cfg.Servers[cfg.Active]
	return &serverCfg, nil
}

type ClientConfig struct {
	Active  string                  `json:"active"`
	Servers map[string]ServerConfig `json:"servers"`
}

type ServerConfig struct {
	APIToken   string `json:"api_token"`
	ServerURL  string `json:"server_url"`
	SkipVerify bool   `json:"skip_verify"`
}
