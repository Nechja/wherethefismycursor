//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	configFileName   = "config.json"
	manifestFileName = "comctl32.manifest"

	styleSolid   = "solid"
	styleDual    = "dual"
	styleCompact = "compact"
	styleSplit   = "split"
	styleCross   = "cross"
	styleRainbow = "rainbow"
	stylePrism   = "prism"

	haloSizeMin    = 12
	haloSizeMax    = 60
	sensitivityMin = 1
	sensitivityMax = 10

	defaultHaloColor   = "#FFD25A"
	defaultHaloColor2  = "#2E6BFF"
	defaultRingColor   = "#4FC3FF"
	defaultHaloSize    = 24
	defaultSensitivity = 6

	runKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = appName
)

type rgb struct{ r, g, b uint8 }

type appConfig struct {
	HaloEnabled  bool   `json:"halo_enabled"`
	HaloStyle    string `json:"halo_style"`
	ShakeEnabled bool   `json:"shake_enabled"`
	HaloColor    string `json:"halo_color"`
	HaloColor2   string `json:"halo_color2"`
	RingColor    string `json:"ring_color"`
	HaloSize     int    `json:"halo_size"`
	Sensitivity  int    `json:"sensitivity"`
}

func defaultConfig() appConfig {
	return appConfig{
		HaloEnabled:  true,
		HaloStyle:    styleSolid,
		ShakeEnabled: true,
		HaloColor:    defaultHaloColor,
		HaloColor2:   defaultHaloColor2,
		RingColor:    defaultRingColor,
		HaloSize:     defaultHaloSize,
		Sensitivity:  defaultSensitivity,
	}
}

func validStyle(s string) bool {
	switch s {
	case styleSolid, styleDual, styleCompact, styleSplit, styleCross, styleRainbow, stylePrism:
		return true
	}
	return false
}

func migrateStyle(data []byte) string {
	var legacy struct {
		Cycle bool `json:"halo_cycle"`
		Dual  bool `json:"halo_dual"`
	}
	json.Unmarshal(data, &legacy)
	switch {
	case legacy.Cycle && legacy.Dual:
		return stylePrism
	case legacy.Cycle:
		return styleRainbow
	case legacy.Dual:
		return styleDual
	}
	return styleSolid
}

func parseColor(s string) (rgb, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return rgb{}, fmt.Errorf("want RRGGBB hex, got %q", s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return rgb{}, err
	}
	return rgb{uint8(v >> 16), uint8(v >> 8), uint8(v)}, nil
}

func hexColor(c rgb) string {
	return fmt.Sprintf("#%02X%02X%02X", c.r, c.g, c.b)
}

func configDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, appName)
	os.MkdirAll(dir, 0o755)
	return dir
}

func configPath() string { return filepath.Join(configDir(), configFileName) }

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func loadConfig() appConfig {
	c := defaultConfig()
	if data, err := os.ReadFile(configPath()); err == nil {
		json.Unmarshal(data, &c)
		if !validStyle(c.HaloStyle) {
			c.HaloStyle = migrateStyle(data)
		}
	}
	c.HaloSize = clampInt(c.HaloSize, haloSizeMin, haloSizeMax)
	c.Sensitivity = clampInt(c.Sensitivity, sensitivityMin, sensitivityMax)
	if _, err := parseColor(c.HaloColor); err != nil {
		c.HaloColor = defaultHaloColor
	}
	if _, err := parseColor(c.HaloColor2); err != nil {
		c.HaloColor2 = defaultHaloColor2
	}
	if _, err := parseColor(c.RingColor); err != nil {
		c.RingColor = defaultRingColor
	}
	return c
}

func saveConfig() {
	configGen++
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(configPath(), append(data, '\n'), 0o644)
}

func autostartEnabled() bool {
	var hKey uintptr
	r, _, _ := pRegOpenKeyExW.Call(hkeyCurrentUser,
		uintptr(strPtr(runKeyPath)), 0, keyQueryValue,
		uintptr(unsafe.Pointer(&hKey)))
	if r != 0 {
		return false
	}
	defer pRegCloseKey.Call(hKey)
	r, _, _ = pRegQueryValueExW.Call(hKey,
		uintptr(strPtr(runValueName)), 0, 0, 0, 0)
	return r == 0
}

func setAutostart(on bool) {
	var hKey uintptr
	r, _, _ := pRegOpenKeyExW.Call(hkeyCurrentUser,
		uintptr(strPtr(runKeyPath)), 0, keySetValue,
		uintptr(unsafe.Pointer(&hKey)))
	if r != 0 {
		return
	}
	defer pRegCloseKey.Call(hKey)
	if !on {
		pRegDeleteValueW.Call(hKey, uintptr(strPtr(runValueName)))
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	val, err := syscall.UTF16FromString(`"` + exe + `"`)
	if err != nil {
		return
	}
	pRegSetValueExW.Call(hKey,
		uintptr(strPtr(runValueName)), 0, regSz,
		uintptr(unsafe.Pointer(&val[0])), uintptr(len(val)*2))
}
