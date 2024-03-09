package tracker

var (
	codesByScene = map[string]map[string]string{
		"Global": {
			"Firecracker": "Granted Firecracker|1",
			"Firebomb":    "Granted Firebomb|1",
			"Icebomb":     "Granted Icebomb|1",
		},
		"Overworld": {
			"Upper Flowers Fairy":      "SV_Fairy_1_Overworld_Flowers_Upper_Revealed|1",
			"Lower Flower Fairy":       "SV_Fairy_2_Overworld_Flowers_Lower_Revealed|1",
			"Moss Fairy":               "SV_Fairy_3_Overworld_Moss_Revealed|1",
			"Compass Fairy":            "SV_Fairy_11_WeatherVane_Revealed|1",
			"Fountain Fairy":           "SV_Fairy_16_Fountain_Revealed|1",
			"Back To Work Treasure":    "SV_Overworld Redux_Starting Island Trophy Chest|1",
			"Vintage Treasure":         "SV_Overworld Redux_Tropical Secret Chest - Environmental Filigree Safe (Non Fairy)|1",
			"Power Up Treasure":        "SV_Overworld Redux_Windchime Chest - Environmental Filigree Safe (Non Fairy)|1",
			"Sacred Geometry Treasure": "SV_Overworld Redux_Windmill Chest - Environmental Filigree Safe (Non Fairy)|1",
			"Fountain Cross Door":      "SV_Overworld Redux_Filigree_Door_Basic (1)|1",
			"Southeast Cross Door":     "SV_Overworld Redux_Filigree_Door_Basic|1",
			"Fire Wand Obelisk Page":   "SV_Overworld Redux_Obelisk_Solved|1",
		},
		"Cube Cave": {
			"Cube Fairy": "SV_Fairy_14_Cube_Revealed|1",
		},
		"Caustic Light Cave": {
			"Casting Light Fairy": "SV_Fairy_4_Caustics_Revealed|1",
		},
		"Ruined Passage": {
			"Ruined Passage Door": "SV_Ruins Passage_secret filigree door|1",
		},
		"Patrol Cave": {
			"Patrol Fairy": "SV_Fairy_13_Patrol_Revealed|1",
		},
		"Secret Gathering Place": {
			"Waterfall Fairy": "SV_Fairy_5_Waterfall_Revealed|1",
		},
		"Hourglass Cave": {
			"Hourglass Door":  "SV_Town Basement_secret filigree door|1",
			"Hourglass Fairy": "SV_Fairy_10_3DPillar_Revealed|1",
		},
		"Maze Cave": {
			"Maze Fairy": "SV_Fairy_15_Maze_Revealed|1",
		},
		"Old House": {
			"Old House Fairy": "SV_Fairy_12_House_Revealed|1",
			"Old House Door":  "SV_Overworld Interiors_Filigree_Door|1",
		},
		"Sealed Temple": {
			"Temple Fairy": "SV_Fairy_6_Temple_Revealed|1",
		},
		"East Forest": {
			"Dancer Fairy":  "SV_Fairy_8_Dancer_Revealed|1",
			"Obelisk Fairy": "SV_Fairy_20_ForestMonolith_Revealed|1",
		},
		"West Garden": {
			"Sword Door":  "SV_Archipelagos Redux_Filigree_Door_Basic|1",
			"Tiles Fairy": "SV_Fairy_18_GardenCourtyard_Revealed|1",
			"Tree Fairy":  "SV_Fairy_17_GardenTree_Revealed|1",
		},
		"Library Hall": {
			"Library Fairy": "SV_Fairy_9_Library_Rug_Revealed|1",
		},
		"Eastern Vault Fortress": {
			"Candles Fairy": "SV_Fairy_19_FortressCandles_Revealed|1",
		},
		"Quarry": {
			"Quarry Fairy": "SV_Fairy_7_Quarry_Revealed|1",
		},
		"Lower Mountain": {
			"Top Of Mountain Door": "SV_Mountain___Final Door Spell Listener|1",
		},
		"Cathedral": {
			"Secret Legend Door": "SV_Cathedral Redux_secret filigree door|1",
		},
	}
)
