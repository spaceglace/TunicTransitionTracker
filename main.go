package main

import (
	"bufio"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

var (
	portalRegex   = regexp.MustCompile(`^randomizer entered portal ([^|]+)\|1$`)
	entranceRegex = regexp.MustCompile(`\s+- (.+) -- (.+)$`)
	itemRegex     = regexp.MustCompile(`^\s+([-x]) ([^-]+) - ([^:]+): `)

	logger *zap.Logger
)

type (
	Settings struct {
		SaveLocation    string `json:"saveFolder"`
		SpoilerLocation string `json:"spoilerLocation"`
	}

	Debug struct {
		Name          string
		Seed          string
		SpoilerSeed   string
		Archipelago   bool
		Randomized    bool
		HexQuest      bool
		EntranceRando bool
		FixedShops    bool
	}
	Total struct {
		Total        int
		Undiscovered int
	}
	Totals struct {
		Entrances Total
		Checks    Total
	}
	Scene struct {
		Totals    Totals
		Checks    map[string]bool
		Entrances map[string]string
	}
	Save struct {
		Debug  Debug
		Totals Totals
		Scene  string
		Scenes map[string]Scene
	}
)

func mostRecentSave(path string) string {
	// read all existing saves to get most recent
	mostRecent := ""
	mostRecentMod := time.Time{}
	files, err := os.ReadDir(path)
	if err != nil {
		panic(err)
	}
	// iterate over each file in save directory
	for _, file := range files {
		name := file.Name()
		if !strings.HasSuffix(name, ".tunic") {
			continue
		}
		info, err := file.Info()
		if err != nil {
			panic(err)
		}

		if info.ModTime().After(mostRecentMod) {
			mostRecent = name
			mostRecentMod = info.ModTime()
		}
		info.ModTime()
	}

	return mostRecent
}

func parseWithSpoiler(saveLoc, spoilerLoc string) Save {
	payload := Save{
		Debug:  Debug{},
		Totals: Totals{},
		Scenes: map[string]Scene{},
	}
	// populate our payload with every scene
	for _, a := range sceneNames {
		payload.Scenes[a] = Scene{
			Totals: Totals{
				Entrances: Total{},
				Checks:    Total{},
			},
			Checks:    map[string]bool{},
			Entrances: map[string]string{},
		}
	}
	spoiler := map[string]string{}

	// open spoiler.log
	spoilerReader, err := os.Open(spoilerLoc)
	if err != nil {
		panic(err)
	}
	spoilerScanner := bufio.NewScanner(spoilerReader)
	spoilerScanner.Split(bufio.ScanLines)

	quiesce := false
	for spoilerScanner.Scan() {
		line := spoilerScanner.Text()

		if quiesce && strings.HasPrefix(line, "\t") {
			continue
		}
		quiesce = false
		if line == "Major Items" {
			quiesce = true
			continue
		}

		if strings.HasPrefix(line, "Seed: ") {
			payload.Debug.SpoilerSeed = strings.TrimPrefix(line, "Seed: ")
		}

		// check if this is an item line
		matches := itemRegex.FindStringSubmatch(line)
		if len(matches) > 0 {
			found := matches[1] != "-"
			_, ok := payload.Scenes[matches[2]]
			if !ok {
				logger.Warn("Ignoring unknown check location",
					zap.String("location", matches[2]),
				)
				continue
			}

			payload.Totals.Checks.Total++
			temp := payload.Scenes[matches[2]]
			temp.Totals.Checks.Total++
			temp.Checks[matches[3]] = found
			if !found {
				payload.Totals.Checks.Undiscovered++
				temp.Totals.Checks.Undiscovered++
			}
			payload.Scenes[matches[2]] = temp
		}

		// check if this is an entrance connection line
		matches = entranceRegex.FindStringSubmatch(line)
		if len(matches) > 0 {
			payload.Totals.Entrances.Total += 2
			spoiler[matches[1]] = matches[2]
			spoiler[matches[2]] = matches[1]
		}
	}
	spoilerReader.Close()

	logger.Debug("Finished parsing Spoiler.log",
		zap.Int("Items", payload.Totals.Checks.Total),
		zap.Int("Entrances", payload.Totals.Entrances.Total),
	)

	// open save file
	recent := mostRecentSave(saveLoc)
	payload.Debug.Name = recent
	saveReader, err := os.Open(path.Join(saveLoc, recent))
	if err != nil {
		panic(err)
	}
	saveScanner := bufio.NewScanner(saveReader)
	saveScanner.Split(bufio.ScanLines)
	entrances := map[string]struct{}{}

	for saveScanner.Scan() {
		line := saveScanner.Text()

		// easy checks first
		if line == "archipelago|1" {
			payload.Debug.Archipelago = true
			payload.Debug.Randomized = true
		} else if line == "randomizer|1" {
			payload.Debug.Randomized = true
		} else if line == "randomizer hexagon quest enabled|1" {
			payload.Debug.HexQuest = true
		} else if line == "randomizer entrance rando enabled|1" {
			payload.Debug.EntranceRando = true
		} else if line == "randomizer ER fixed shop|1" {
			payload.Debug.FixedShops = true
		} else if strings.HasPrefix(line, "seed|") {
			payload.Debug.Seed = strings.Split(line, "|")[1]
		} else if strings.HasPrefix(line, "last spawn scene name|") {
			rawScene := strings.Split(line, "|")[1]
			scene, err := TranslateScene(rawScene)
			if err != nil {
				logger.Error("Failed to translate current scene",
					zap.String("scene", rawScene),
				)
			}
			payload.Scene = scene
		}

		matches := portalRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			entrances[matches[1]] = struct{}{}
			// we hate shops
			if matches[1] == "Shop Portal" || matches[1] == "Shop" {
				continue
			}
			// look up the entrance pairings
			mapping, ok := spoiler[matches[1]]
			if !ok {
				logger.Warn("Found entrance not present in spoiler log",
					zap.String("line", line),
				)
			} else {
				// look up what region this entrance is a part of
				region, ok := doorRegions[matches[1]]
				if !ok {
					logger.Warn("Found door with no associated region",
						zap.String("line", line),
					)
					continue
				}

				temp := payload.Scenes[region]
				temp.Entrances[matches[1]] = mapping
				temp.Totals.Entrances.Total++
				payload.Scenes[region] = temp
			}
		}
	}
	saveReader.Close()

	// look for unfound entrances
	for scene, doors := range allDoors {
		for _, door := range doors {
			_, ok := entrances[door]
			if !ok {
				payload.Totals.Entrances.Undiscovered++
				temp := payload.Scenes[scene]
				temp.Entrances[door] = ""
				temp.Totals.Entrances.Total++
				temp.Totals.Entrances.Undiscovered++
				payload.Scenes[scene] = temp
			}
		}
	}

	return payload
}

func parseWithoutSpoiler(saveLoc string) Save {
	payload := Save{
		Debug:  Debug{},
		Totals: Totals{},
		Scenes: map[string]Scene{},
	}
	// populate our payload with every scene
	for _, a := range sceneNames {
		payload.Scenes[a] = Scene{
			Totals: Totals{
				Entrances: Total{},
				Checks:    Total{},
			},
			Checks:    map[string]bool{},
			Entrances: map[string]string{},
		}
	}

	// open save file
	recent := mostRecentSave(saveLoc)
	reader, err := os.Open(path.Join(saveLoc, recent))
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)
	entrancesLookup := map[string]struct{}{}
	entrances := []string{}

	// iterate over save file
	for scanner.Scan() {
		line := scanner.Text()

		// easy checks first
		if line == "archipelago|1" {
			payload.Debug.Archipelago = true
			payload.Debug.Randomized = true
		} else if line == "randomizer|1" {
			payload.Debug.Randomized = true
		} else if line == "randomizer hexagon quest enabled|1" {
			payload.Debug.HexQuest = true
		} else if line == "randomizer entrance rando enabled|1" {
			payload.Debug.EntranceRando = true
		} else if line == "randomizer ER fixed shop|1" {
			payload.Debug.FixedShops = true
		} else if strings.HasPrefix(line, "seed|") {
			payload.Debug.Seed = strings.Split(line, "|")[1]
		} else if strings.HasPrefix(line, "last spawn scene name|") {
			scene, err := TranslateScene(strings.Split(line, "|")[1])
			if err != nil {
				logger.Error("Failed to look up current scene",
					zap.Error(err),
				)
			}
			payload.Scene = scene
		}

		matches := portalRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			entrancesLookup[matches[1]] = struct{}{}
			entrances = append(entrances, matches[1])
		}

		//TODO: items lol
	}

	//TODO: don't assume entrance rando?

	lookahead := 6
	if payload.Debug.FixedShops {
		lookahead = 2
	}

	for i := 0; i < len(entrances); i += 2 {
		// could this support a shop?
		if i+lookahead < len(entrances) {
			// is there a shop?
			if entrances[i] == "Shop Portal" || entrances[i+1] == "Shop Portal" {
				for j := 0; j < lookahead; j++ {
					if entrances[i+j] == "Shop Portal" {
						continue
					}
					region, ok := doorRegions[entrances[i+j]]
					if !ok {
						logger.Warn("Found door with no associated region",
							zap.String("door", entrances[i]),
						)
						continue
					}
					payload.Totals.Entrances.Total++
					temp := payload.Scenes[region]
					temp.Totals.Entrances.Total++
					temp.Entrances[entrances[i+j]] = "Shop Portal"
					payload.Scenes[region] = temp
				}
				i += lookahead
			} else {
				// look up what region this entrance is a part of
				region1, ok := doorRegions[entrances[i]]
				if !ok {
					logger.Warn("Found door with no associated region",
						zap.String("door", entrances[i]),
					)
					continue
				}
				region2, ok := doorRegions[entrances[i+1]]
				if !ok {
					logger.Warn("Found door with no associated region",
						zap.String("door", entrances[i+1]),
					)
					continue
				}

				payload.Totals.Entrances.Total += 2

				temp1 := payload.Scenes[region1]
				temp1.Totals.Entrances.Total++
				temp1.Entrances[entrances[i]] = entrances[i+1]
				payload.Scenes[region1] = temp1

				temp2 := payload.Scenes[region1]
				temp2.Totals.Entrances.Total++
				temp2.Entrances[entrances[i+1]] = entrances[i]
				payload.Scenes[region2] = temp2
			}
		}
	}

	// look for unfound entrances
	for scene, doors := range allDoors {
		for _, door := range doors {
			_, ok := entrancesLookup[door]
			if !ok {
				payload.Totals.Entrances.Undiscovered++
				temp := payload.Scenes[scene]
				temp.Entrances[door] = ""
				temp.Totals.Entrances.Undiscovered++
				payload.Scenes[scene] = temp
			}
		}
	}

	return payload
}

func loadSettings() Settings {
	// assume there's a settings.json
	s, err := os.Open("settings.json")
	if err != nil {
		q, _ := json.MarshalIndent(Settings{}, "", "	")
		ioutil.WriteFile("settings.json", q, os.ModePerm)

		logger.Fatal("Expected 'settings.json' in same directory. Creating template...")
		panic(err)
	}
	var settings Settings
	json.NewDecoder(s).Decode(&settings)

	return settings
}

func main() {
	settings := loadSettings()

	e := echo.New()
	e.HideBanner = true

	address := ":8000"

	logger, _ = zap.NewProduction()
	logger.Info("Welcome to the Tunic Transition Tracker!",
		zap.String("api", address))

	/*
		// TODO: timer to poll for changes, vs recreating every call
		tick := time.NewTicker(500 * time.Millisecond)
		go func() {
			for {
				<-tick.C
			}
		}
	*/

	e.GET("/spoiler", func(c echo.Context) error {
		payload := parseWithSpoiler(settings.SaveLocation, settings.SpoilerLocation)
		logger.Debug("Running /spoiler")
		return c.JSON(http.StatusOK, payload)
	})

	e.GET("/nospoiler", func(c echo.Context) error {
		payload := parseWithoutSpoiler(settings.SaveLocation)
		logger.Debug("Running /nospoiler")
		return c.JSON(http.StatusOK, payload)
	})

	logger.Error("Exiting server", zap.Error(e.Start(address)))
}
