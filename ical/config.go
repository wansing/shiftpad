package ical

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	URL      string `json:"url"`
	Username string `json:"username"` // can be empty
	Password string `json:"password"` // can be empty
}

func LoadConfig(jsonfile string) (Config, error) {
	filecontent, err := os.ReadFile(jsonfile)
	if err != nil {
		return Config{}, fmt.Errorf("error opening ical config: %v", err)
	}
	var config Config
	if err := json.Unmarshal(filecontent, &config); err != nil {
		return Config{}, fmt.Errorf("error decoding ical config: %v", err)
	}
	return config, nil
}
