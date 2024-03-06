package main

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	portalRegex   = regexp.MustCompile(`^randomizer entered portal ([^|]+)\|1$`)
	entranceRegex = regexp.MustCompile(`\s+- (.+) -- (.+)$`)
	itemRegex     = regexp.MustCompile(`^\s+([-x]) ([^-]+) - ([^:]+): `)

	logger *zap.Logger
)

const ()

type (
	Settings struct {
		SecretLegend string `json:"secretLegend"`
		Address      string `json:"address"`
	}

	Debug struct {
		Name          string
		Hash          string
		Seed          string
		SpoilerSeed   string
		SpoilerMod    time.Time
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
	Current struct {
		Scene      string
		Respawn    string
		Dath       string
		HasLaurels bool
		HasDath    bool
	}
	Save struct {
		Debug   Debug
		Totals  Totals
		Current Current
		Scenes  map[string]Scene
		Codes   map[string]map[string]bool
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

func getSceneFromFlag(flag string) string {
	rawScene := strings.Split(flag, "|")[1]
	scene, err := TranslateScene(rawScene)
	if err != nil {
		logger.Error("Failed to translate current scene",
			zap.String("scene", rawScene),
		)
	}
	return scene
}

func parseWithSpoiler(recent, saves, spoilerLoc string) (Save, error) {
	payload := Save{
		Debug:  Debug{},
		Totals: Totals{},
		Scenes: map[string]Scene{},
		Codes:  map[string]map[string]bool{},
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

	// populate our payload with each code family
	for family, section := range codes {
		payload.Codes[family] = map[string]bool{}
		for code := range section {
			payload.Codes[family][code] = false
		}
	}

	// get spoiler.log update time
	spoilerStat, err := os.Stat(spoilerLoc)
	if err != nil {
		logger.Error("Failed to get spoiler log stats",
			zap.String("spoiler location", spoilerLoc),
			zap.Error(err),
		)
		return payload, fmt.Errorf("Failed to stat spoiler log: %w", err)
	}
	payload.Debug.SpoilerMod = spoilerStat.ModTime()

	hash := md5.Sum([]byte(recent + spoilerStat.ModTime().String()))
	payload.Debug.Hash = hex.EncodeToString(hash[:])

	// open spoiler.log
	spoilerReader, err := os.Open(spoilerLoc)
	if err != nil {
		logger.Error("Failed to open spoiler log",
			zap.String("spoiler location", spoilerLoc),
			zap.Error(err),
		)
		return payload, fmt.Errorf("Failed to open spoiler log: %w", err)
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

	// open save file
	payload.Debug.Name = recent
	saveReader, err := os.Open(path.Join(saves, recent))
	if err != nil {
		logger.Error("Failed to open spoiler log",
			zap.String("save location", saves),
			zap.String("most recent", recent),
			zap.Error(err),
		)
		return payload, fmt.Errorf("Failed to open most recent save file: %w", err)
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
		} else if line == "inventory quantity Dath Stone|1" {
			payload.Current.HasDath = true
		} else if line == "inventory quantity Hyperdash|1" {
			payload.Current.HasLaurels = true
		} else if strings.HasPrefix(line, "seed|") {
			payload.Debug.Seed = strings.Split(line, "|")[1]
		} else if strings.HasPrefix(line, "last spawn scene name|") {
			payload.Current.Scene = getSceneFromFlag(line)
		} else if strings.HasPrefix(line, "last campfire scene name|") {
			payload.Current.Respawn = getSceneFromFlag(line)
		} else if strings.HasPrefix(line, "randomizer last campfire scene name for dath stone|") {
			payload.Current.Dath = getSceneFromFlag(line)
		}

		// holy cross code flags
		for family, section := range codes {
			for check, code := range section {
				if line == code {
					payload.Codes[family][check] = true
				}
			}
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

	logger.Debug("Finished parsing",
		zap.Int("items", payload.Totals.Checks.Total),
		zap.Int("entrances", payload.Totals.Entrances.Total),
		zap.String("hash", payload.Debug.Hash),
	)

	return payload, nil
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
			payload.Current.Scene = scene
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
	// no longer assume there's a settings.json
	var settings Settings
	s, err := os.Open("settings.json")
	if err != nil {
		q, _ := json.MarshalIndent(Settings{
			Address: ":8000",
		}, "", "	")
		ioutil.WriteFile("settings.json", q, os.ModePerm)
		logger.Warn("No settings found! Please configure through the frontend or via settings.json")
	} else {
		json.NewDecoder(s).Decode(&settings)
	}

	return settings
}

func main() {
	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
	}))

	pe := zap.NewProductionEncoderConfig()
	pe.EncodeTime = zapcore.ISO8601TimeEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(pe)
	level := zap.DebugLevel
	core := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level)
	logger = zap.New(core)

	settings := loadSettings()
	spoiler := filepath.Join(settings.SecretLegend, "Randomizer", "Spoiler.log")
	saves := filepath.Join(settings.SecretLegend, "SAVES")

	logger.Info("Welcome to the Tunic Transition Tracker!",
		zap.String("listener", settings.Address),
	)

	tick := time.NewTicker(250 * time.Millisecond)
	state := Save{}
	go func() {
		for {
			<-tick.C
			changedSave := false
			changedSpoiler := false
			check := mostRecentSave(saves)
			// TODO: handle if there's no save file
			changedSave = check != state.Debug.Name

			// check the spoiler.log for updates
			spoilerStat, err := os.Stat(spoiler)
			if err != nil {
				//TODO: handle if spoiler stat errors
			}
			changedSpoiler = !state.Debug.SpoilerMod.Equal(spoilerStat.ModTime())

			if changedSave || changedSpoiler {
				logger.Debug("Detected update",
					zap.Bool("save updated", changedSave),
					zap.Bool("spoiler updated", changedSpoiler),
					zap.String("save name", check),
					zap.Time("spoiler update", spoilerStat.ModTime()),
				)
				blob, err := parseWithSpoiler(check, saves, spoiler)
				state = blob
				if err != nil {
					logger.Error("Error attempting to parse save state",
						zap.Error(err),
					)
				}
			}
		}
	}()

	e.Static("/", "frontend/")

	e.GET("/spoiler", func(c echo.Context) error {
		return c.JSON(http.StatusOK, state)
	})

	e.GET("/nospoiler", func(c echo.Context) error {
		payload := parseWithoutSpoiler(saves)
		logger.Warn("mostly unimplemented")
		return c.JSON(http.StatusOK, payload)
	})

	e.GET("/settings", func(c echo.Context) error {
		return c.JSON(http.StatusOK, settings)
	})

	e.POST("/settings", func(c echo.Context) error {
		payload := Settings{}
		if err := c.Bind(&payload); err != nil {
			logger.Error("Failed to read new settings",
				zap.Error(err),
			)
			return err
		}
		old := settings
		settings = payload
		f, err := os.OpenFile("settings.json", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
		if err != nil {
			logger.Error("Failed to open settings file for writing",
				zap.Error(err),
			)
			return err
		}

		q, err := json.MarshalIndent(payload, "", "	")
		if err != nil {
			logger.Error("Failed to marshall settings struct into string",
				zap.Error(err),
			)
			return err
		}

		_, err = f.Write(q)
		if err != nil {
			logger.Error("Failed to write settings file",
				zap.Error(err),
			)
			return err
		}
		f.Close()

		if old.Address != settings.Address {
			logger.Warn("Binding address has changed! PLEASE RESTART THIS FOR CHANGES TO TAKE EFFECT")
		}

		return c.JSON(http.StatusOK, settings)
	})

	logger.Error("Exiting server", zap.Error(e.Start(":8000")))
}
