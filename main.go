package main

import (
	"entrance1/log"
	"entrance1/server"
	"entrance1/settings"
	"entrance1/tracker"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

func main() {
	log.Initialize()
	settings.Load()

	log.Log.Info("Welcome to the Tunic Transition Tracker!",
		zap.String("path", settings.State.SecretLegend),
		zap.String("listener", settings.State.Address),
	)
	// poll for updates every 250ms
	tick := time.NewTicker(250 * time.Millisecond)
	go func() {
		consecutiveFailures := 0

		noDirWarningArm := true
		noSaveWarningArm := true

		for {
			<-tick.C
			spoiler := filepath.Join(settings.State.SecretLegend, "Randomizer", "Spoiler.log")
			saves := filepath.Join(settings.State.SecretLegend, "SAVES")

			// read all existing saves to get most recent
			check := ""
			mostRecentMod := time.Time{}
			files, err := os.ReadDir(saves)
			if err != nil {
				if noDirWarningArm {
					log.Log.Error("Could not read tunic SAVES directory",
						zap.String("saves", saves),
					)
				}
				noDirWarningArm = false
				continue
			}
			// if we get here, assume we read the directory correctly
			noDirWarningArm = true

			// iterate over each file in save directory
			for _, file := range files {
				name := file.Name()
				if !strings.HasSuffix(name, ".tunic") {
					continue
				}
				info, err := file.Info()
				if err != nil {
					// do not warn because we'd be spamming 10x a second
					continue
				}

				if info.ModTime().After(mostRecentMod) {
					check = name
					mostRecentMod = info.ModTime()
				}
			}
			// make sure we found at least one save file
			if check == "" {
				if noSaveWarningArm {
					log.Log.Error("Could not find any .tunic files in SAVES directory",
						zap.String("saves", saves),
					)
				}
				noSaveWarningArm = false
				continue
			}
			// if we made it past the check, re-arm the no-save warning
			noSaveWarningArm = true
			changedSave := check != tracker.State.Debug.Name

			// check the spoiler.log for updates
			spoilerStat, err := os.Stat(spoiler)
			if err != nil {
				consecutiveFailures++
				log.Log.Error("Could not poll spoiler log. File may be busy?",
					zap.Int("failures", consecutiveFailures),
					zap.Error(err),
				)
				continue
			}
			changedSpoiler := !tracker.State.Debug.SpoilerMod.Equal(spoilerStat.ModTime())

			// run a full update if either file we care about changed
			if changedSave || changedSpoiler {
				log.Log.Debug("Detected update",
					zap.Bool("save updated", changedSave),
					zap.Bool("spoiler updated", changedSpoiler),
					zap.String("save name", check),
					zap.Time("spoiler update", spoilerStat.ModTime()),
				)
				if err := tracker.ParseWithSpoiler(check, saves, spoiler); err != nil {
					log.Log.Error("Error attempting to parse save state",
						zap.Error(err),
					)
				}
			}
			// if we made it to the end, it was a successful update
			consecutiveFailures = 0
		}
	}()

	server.Listen()
}
