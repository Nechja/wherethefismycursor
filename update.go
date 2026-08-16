//go:build windows

package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	repoSlug          = "Nechja/wherethefismycursor"
	releasesPageURL   = "https://github.com/" + repoSlug + "/releases"
	latestReleaseAPI  = "https://api.github.com/repos/" + repoSlug + "/releases/latest"
	updateCheckDelay  = 15 * time.Second
	updateCheckEvery  = 6 * time.Hour
	updateHTTPTimeout = 10 * time.Second

	wmAppUpdateChecked = wmApp + 2

	shellVerbOpen = "open"
)

var (
	updateMu  sync.Mutex
	latestTag string
)

func parseSemver(s string) (int, int, int, bool) {
	parts := strings.Split(strings.TrimPrefix(s, "v"), ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return 0, 0, 0, false
		}
		nums[i] = n
	}
	return nums[0], nums[1], nums[2], true
}

func newerVersion() (string, bool) {
	updateMu.Lock()
	tag := latestTag
	updateMu.Unlock()
	curMaj, curMin, curPat, ok := parseSemver(appVersion)
	if !ok {
		return "", false
	}
	relMaj, relMin, relPat, ok := parseSemver(tag)
	if !ok {
		return "", false
	}
	newer := relMaj > curMaj ||
		(relMaj == curMaj && relMin > curMin) ||
		(relMaj == curMaj && relMin == curMin && relPat > curPat)
	if !newer {
		return "", false
	}
	return tag, true
}

func startUpdateChecker() {
	go func() {
		time.Sleep(updateCheckDelay)
		for {
			checkLatestRelease()
			time.Sleep(updateCheckEvery)
		}
	}()
}

func checkLatestRelease() {
	client := &http.Client{Timeout: updateHTTPTimeout}
	req, err := http.NewRequest(http.MethodGet, latestReleaseAPI, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", appName)
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if json.NewDecoder(resp.Body).Decode(&release) != nil || release.TagName == "" {
		return
	}
	updateMu.Lock()
	latestTag = release.TagName
	updateMu.Unlock()
	pPostMessageW.Call(appHwnd, wmAppUpdateChecked, 0, 0)
}

func openReleasesPage() {
	pShellExecuteW.Call(0, uintptr(strPtr(shellVerbOpen)), uintptr(strPtr(releasesPageURL)), 0, 0, swShow)
}
