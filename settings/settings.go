package settings

import (
	"encoding/json"
	"entrance1/log"
	"io/ioutil"
	"os"

	"go.uber.org/zap"
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
		standard := Settings{
			Address: ":8000",
		}
		q, _ := json.MarshalIndent(standard, "", "	")
		ioutil.WriteFile("settings.json", q, os.ModePerm)
		log.Log.Warn("No valid settings found! Running on default listener -- this can be configured via API or settings.json",
			zap.String("listener", standard.Address),
		)
		State = standard
	} else {
		json.NewDecoder(s).Decode(&settings)
		State = settings
	}
}
