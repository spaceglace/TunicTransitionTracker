package tracker

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"entrance1/log"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
)

type (
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
	Door struct {
		Scene string
		Door  string
	}
	Scene struct {
		Totals    Totals
		Checks    map[string]bool
		Entrances map[string]Door
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

var (
	portalRegex   = regexp.MustCompile(`^randomizer entered portal ([^|]+)\|1$`)
	entranceRegex = regexp.MustCompile(`\s+- (.+) -- (.+)$`)
	itemRegex     = regexp.MustCompile(`^\s+([-x]) ([^-]+) - ([^:]+): `)

	State Save
)

func getSceneFromFlag(flag string) string {
	rawScene := strings.Split(flag, "|")[1]
	scene, err := TranslateScene(rawScene)
	if err != nil {
		log.Log.Error("Failed to translate current scene",
			zap.String("scene", rawScene),
		)
	}
	return scene
}

func ParseWithSpoiler(recent, saves, spoilerLoc string) error {
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
			Entrances: map[string]Door{},
		}
	}
	spoiler := map[string]string{}

	// populate our payload with each code family
	for family, section := range codesByScene {
		payload.Codes[family] = map[string]bool{}
		for code := range section {
			payload.Codes[family][code] = false
		}
	}

	// get spoiler.log update time
	spoilerStat, err := os.Stat(spoilerLoc)
	if err != nil {
		log.Log.Error("Failed to get spoiler log stats",
			zap.String("spoiler location", spoilerLoc),
			zap.Error(err),
		)
		return fmt.Errorf("Failed to stat spoiler log: %w", err)
	}
	payload.Debug.SpoilerMod = spoilerStat.ModTime()

	hash := md5.Sum([]byte(recent + spoilerStat.ModTime().String()))
	payload.Debug.Hash = hex.EncodeToString(hash[:])

	// open spoiler.log
	spoilerReader, err := os.Open(spoilerLoc)
	if err != nil {
		log.Log.Error("Failed to open spoiler log",
			zap.String("spoiler location", spoilerLoc),
			zap.Error(err),
		)
		return fmt.Errorf("Failed to open spoiler log: %w", err)
	}
	spoilerScanner := bufio.NewScanner(spoilerReader)
	spoilerScanner.Split(bufio.ScanLines)

	shopList := []string{}
	quiesce := false

	for spoilerScanner.Scan() {
		line := spoilerScanner.Text()
		// skip the "Major Items" section of the spoiler log
		if quiesce && strings.HasPrefix(line, "\t") {
			continue
		}
		quiesce = false
		if line == "Major Items" {
			quiesce = true
			continue
		}
		// look for the specific seed of the spoiler
		if strings.HasPrefix(line, "Seed: ") {
			payload.Debug.SpoilerSeed = strings.TrimPrefix(line, "Seed: ")
		}
		// check if this is an item line
		matches := itemRegex.FindStringSubmatch(line)
		if len(matches) > 0 {
			found := matches[1] != "-"
			_, ok := payload.Scenes[matches[2]]
			if !ok {
				log.Log.Warn("Ignoring unknown check location",
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
			// keep a separate listing of shop entrances
			if matches[2] == "Shop" || matches[2] == "Shop Portal" {
				shopList = append(shopList, matches[1])
			}
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
		log.Log.Error("Failed to open spoiler log",
			zap.String("save location", saves),
			zap.String("most recent", recent),
			zap.Error(err),
		)
		return fmt.Errorf("Failed to open most recent save file: %w", err)
	}

	saveScanner := bufio.NewScanner(saveReader)
	saveScanner.Split(bufio.ScanLines)
	entrances := map[string]struct{}{}

	foundShop := false

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
		for family, section := range codesByScene {
			for check, code := range section {
				if line == code {
					payload.Codes[family][check] = true
				}
			}
		}

		matches := portalRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			entrances[matches[1]] = struct{}{}
			// don't add mapping here, but remember we found the shop(s)
			if matches[1] == "Shop" {
				foundShop = true
				continue
			}
			// look up the entrance pairings
			mapping, ok := spoiler[matches[1]]
			if !ok {
				log.Log.Warn("Found entrance not present in spoiler log",
					zap.String("line", line),
				)
			} else {
				// look up what region this entrance is a part of
				region, ok := doorRegions[matches[1]]
				if !ok {
					log.Log.Warn("Found door with no associated region",
						zap.String("line", line),
					)
					continue
				}
				// look up what region this exit is a part of
				exitScene, ok := doorRegions[mapping]
				if !ok {
					log.Log.Warn("Found destination door with no associated region",
						zap.String("line", line),
						zap.String("origin", matches[1]),
						zap.String("destination", mapping),
					)
					continue
				}

				temp := payload.Scenes[region]
				temp.Entrances[matches[1]] = Door{exitScene, mapping}
				temp.Totals.Entrances.Total++
				payload.Scenes[region] = temp
			}
		}
	}
	saveReader.Close()

	// look for unfound entrances
	for scene, doors := range allDoors {
		for _, door := range doors {
			// skip shops "because they're weird" (thanks tunic randomizer)
			if door == "Shop" || door == "Shop Portal" {
				continue
			}

			_, ok := entrances[door]
			if !ok {
				payload.Totals.Entrances.Undiscovered++
				temp := payload.Scenes[scene]
				temp.Entrances[door] = Door{}
				temp.Totals.Entrances.Total++
				temp.Totals.Entrances.Undiscovered++
				payload.Scenes[scene] = temp
			}
		}
	}

	// populate shops
	if foundShop {
		temp := payload.Scenes["Shop"]
		for i, destination := range shopList {
			// get region for door
			region := doorRegions[destination]
			temp.Totals.Entrances.Total++
			temp.Entrances[fmt.Sprintf("Shop Portal %d", i+1)] = Door{region, destination}
		}
		payload.Scenes["Shop"] = temp
	}

	if payload.Debug.Seed != payload.Debug.SpoilerSeed {
		log.Log.Warn("save file seed does not match spoiler seed!",
			zap.String("save seed", payload.Debug.Seed),
			zap.String("spoiler seed", payload.Debug.SpoilerSeed),
		)
	}

	log.Log.Debug("Finished parsing",
		zap.Int("items", payload.Totals.Checks.Total),
		zap.Int("entrances", payload.Totals.Entrances.Total),
		zap.String("hash", payload.Debug.Hash),
	)

	State = payload
	return nil
}
