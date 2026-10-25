package game

import (
	"fmt"
	"time"
)

var raceNames = map[uint8]string{
	1: "Human", 2: "Orc", 3: "Dwarf", 4: "Night Elf", 5: "Undead",
	6: "Tauren", 7: "Gnome", 8: "Troll", 10: "Blood Elf", 11: "Draenei",
}

var hordeRaces = map[uint8]bool{2: true, 5: true, 6: true, 8: true, 10: true}

// Class names, and a key the stylesheet colours them by.
var classNames = map[uint8][2]string{
	1: {"Warrior", "warrior"}, 2: {"Paladin", "paladin"}, 3: {"Hunter", "hunter"},
	4: {"Rogue", "rogue"}, 5: {"Priest", "priest"}, 6: {"Death Knight", "deathknight"},
	7: {"Shaman", "shaman"}, 8: {"Mage", "mage"}, 9: {"Warlock", "warlock"}, 11: {"Druid", "druid"},
}

// Character is what the site shows of one character.
type Character struct {
	Name    string `json:"name"`
	Level   int    `json:"level"`
	Race    string `json:"race"`
	Class   string `json:"class"`
	ClassID string `json:"classKey"`
	Faction string `json:"faction"` // "alliance" or "horde"
	Played  string `json:"played,omitempty"`
}

// FactionName is "Alliance" or "Horde".
func (c Character) FactionName() string {
	if c.Faction == "horde" {
		return "Horde"
	}
	return "Alliance"
}

// RealmType is what the realm list calls a realm with this icon.
func RealmType(icon int) string {
	switch icon {
	case 1, 16:
		return "PvP"
	case 6:
		return "RP"
	case 8:
		return "RP-PvP"
	default: // 0 and 4 are both "Normal"
		return "PvE"
	}
}

func newCharacter(name string, race, class uint8, level int, played uint32, showPlayed bool) Character {
	c := Character{Name: name, Level: level, Race: raceNames[race], Faction: "alliance"}
	if c.Race == "" {
		c.Race = "Unknown"
	}
	if hordeRaces[race] {
		c.Faction = "horde"
	}
	if cl, ok := classNames[class]; ok {
		c.Class, c.ClassID = cl[0], cl[1]
	} else {
		c.Class, c.ClassID = "Adventurer", "unknown"
	}
	if showPlayed {
		c.Played = PlayedTime(time.Duration(played) * time.Second)
	}
	return c
}

// PlayedTime is a short /played: "3d 4h", "5h 12m", "40m".
func PlayedTime(d time.Duration) string {
	mins := int(d / time.Minute)
	days, hours, m := mins/(24*60), (mins/60)%24, mins%60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, m)
	default:
		return fmt.Sprintf("%dm", m)
	}
}
