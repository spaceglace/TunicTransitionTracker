package settings

import (
	"encoding/json"
	"entrance1/log"
	"io/ioutil"
	"os"
)

type (
	Settings struct {
		SecretLegend string `json:"secretLegend"`
		Address      string `json:"address"`
	}
)

var (
	State Settings
)

func Load() {
	// no longer assume there's a settings.json
	var settings Settings
	s, err := os.Open("settings.json")
	if err != nil {
		q, _ := json.MarshalIndent(Settings{
			Address: ":8000",
		}, "", "	")
		ioutil.WriteFile("settings.json", q, os.ModePerm)
		log.Log.Warn("No settings found! Please configure through the frontend or via settings.json")
	} else {
		json.NewDecoder(s).Decode(&settings)
	}

	State = settings
}
