package server

import (
	"encoding/json"
	"entrance1/log"
	"entrance1/settings"
	"entrance1/tracker"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
)

func Listen() {
	e := echo.New()
	e.HideBanner = true
	// generic 'allow all' cors config
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
	}))

	e.Static("/", "frontend/")

	e.GET("/spoiler", func(c echo.Context) error {
		return c.JSON(http.StatusOK, tracker.State)
	})

	e.GET("/settings", func(c echo.Context) error {
		return c.JSON(http.StatusOK, settings.State)
	})

	e.POST("/settings", func(c echo.Context) error {
		payload := settings.Settings{}
		if err := c.Bind(&payload); err != nil {
			log.Log.Error("Failed to read new settings",
				zap.Error(err),
			)
			return err
		}
		old := settings.State
		settings.State = payload
		f, err := os.OpenFile("settings.json", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
		if err != nil {
			log.Log.Error("Failed to open settings file for writing",
				zap.Error(err),
			)
			return err
		}

		q, err := json.MarshalIndent(payload, "", "	")
		if err != nil {
			log.Log.Error("Failed to marshall settings struct into string",
				zap.Error(err),
			)
			return err
		}

		_, err = f.Write(q)
		if err != nil {
			log.Log.Error("Failed to write settings file",
				zap.Error(err),
			)
			return err
		}
		f.Close()

		if old.Address != settings.State.Address {
			log.Log.Warn("Binding address has changed! PLEASE RESTART THIS FOR CHANGES TO TAKE EFFECT")
		}

		return c.JSON(http.StatusOK, settings.State)
	})

	log.Log.Error("Exiting server", zap.Error(e.Start(settings.State.Address)))
}
