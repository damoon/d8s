package wedding

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
)

type xRegistryConfig string

type xRegistryAuth string

type dockerConfig struct {
	Auths map[string]dockerAuth `json:"auths"`
}
type dockerAuth struct {
	Auth string `json:"auth"`
}

func (d dockerConfig) mustToJSON() string {
	bytes, err := json.Marshal(d)
	if err != nil {
		log.Fatalf("encode to json failed: %v", err)
	}

	return string(bytes)
}

func (x xRegistryConfig) toDockerConfig() (dockerConfig, error) {
	js, err := base64.StdEncoding.DecodeString(string(x))
	if err != nil {
		return dockerConfig{}, fmt.Errorf("decode registry authentications: %v", err)
	}

	type RegistryCred struct {
		username      string
		password      string
		serveraddress string
	}
	creds := map[string]RegistryCred{}

	err = json.Unmarshal(js, &creds)
	if err != nil {
		return dockerConfig{}, fmt.Errorf("unmarshal registry authentications: %v", err)
	}

	dockerCfg := dockerConfig{
		Auths: make(map[string]dockerAuth),
	}
	for _, cred := range creds {
		dockerCfg.Auths[cred.serveraddress] = dockerAuth{
			Auth: base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", cred.username, cred.password))),
		}
	}

	return dockerCfg, nil
}

func (x xRegistryAuth) toDockerConfig() (dockerConfig, error) {
	js, err := base64.StdEncoding.DecodeString(string(x))
	if err != nil {
		return dockerConfig{}, fmt.Errorf("decode registry authentication: %v", err)
	}

	type registryCred struct {
		Username      string
		Password      string
		Serveraddress string
	}
	cred := registryCred{}

	err = json.Unmarshal(js, &cred)
	if err != nil {
		return dockerConfig{}, fmt.Errorf("unmarshal registry authentication: %v", err)
	}

	dockerCfg := dockerConfig{
		Auths: make(map[string]dockerAuth),
	}
	if cred.Username == "" && cred.Password == "" && cred.Serveraddress == "" {
		return dockerCfg, nil
	}

	dockerCfg.Auths[cred.Serveraddress] = dockerAuth{
		Auth: base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", cred.Username, cred.Password))),
	}

	return dockerCfg, nil
}
