package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"gorm.io/gorm"
)

type Marketplace struct {
	db        *gorm.DB
	mu        sync.RWMutex
	updateJob *time.Ticker
}

func NewMarketplace(database *gorm.DB) *Marketplace {
	m := &Marketplace{db: database}
	return m
}

func (m *Marketplace) StartUpdateChecker(interval time.Duration) {
	if m.updateJob != nil {
		m.updateJob.Stop()
	}
	m.updateJob = time.NewTicker(interval)
	go func() {
		for range m.updateJob.C {
			m.CheckAllUpdates()
		}
	}()
}

func (m *Marketplace) StopUpdateChecker() {
	if m.updateJob != nil {
		m.updateJob.Stop()
	}
}

type RatingSummary struct {
	Overall float64 `json:"overall"`
	Count   int     `json:"count"`
	Stars   []int   `json:"stars"` // [1star_count, 2star_count, ..., 5star_count]
}

func (m *Marketplace) GetRatingSummary(pluginID uint) (*RatingSummary, error) {
	var reviews []db.PluginReview
	if err := m.db.Where("plugin_id = ?", pluginID).Find(&reviews).Error; err != nil {
		return nil, err
	}

	summary := &RatingSummary{
		Stars: make([]int, 5),
	}

	if len(reviews) == 0 {
		return summary, nil
	}

	var total int
	for _, r := range reviews {
		if r.Rating >= 1 && r.Rating <= 5 {
			summary.Stars[r.Rating-1]++
			total += r.Rating
		}
	}

	summary.Count = len(reviews)
	summary.Overall = float64(total) / float64(len(reviews))
	return summary, nil
}

func (m *Marketplace) AddReview(pluginID, userID uint, username string, rating int, comment string) error {
	if rating < 1 || rating > 5 {
		return fmt.Errorf("rating must be between 1 and 5")
	}

	review := db.PluginReview{
		PluginID: pluginID,
		UserID:   userID,
		Username: username,
		Rating:   rating,
		Comment:  comment,
	}

	return m.db.Create(&review).Error
}

func (m *Marketplace) GetReviews(pluginID uint) ([]db.PluginReview, error) {
	var reviews []db.PluginReview
	err := m.db.Where("plugin_id = ?", pluginID).Order("created_at DESC").Find(&reviews).Error
	return reviews, err
}

type DependencyResult struct {
	Name             string `json:"name"`
	RequiredVersion  string `json:"required_version"`
	Optional         bool   `json:"optional"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installed_version"`
	Satisfied        bool   `json:"satisfied"`
}

func (m *Marketplace) GetDependencies(pluginID uint) ([]DependencyResult, error) {
	var deps []db.PluginDependency
	if err := m.db.Where("plugin_id = ?", pluginID).Preload("Dependency").Find(&deps).Error; err != nil {
		return nil, err
	}

	result := make([]DependencyResult, 0, len(deps))
	for _, dep := range deps {
		installed := dep.Dependency.ID != 0
		result = append(result, DependencyResult{
			Name:             dep.Dependency.Name,
			RequiredVersion:  dep.RequiredVersion,
			Optional:         dep.Optional,
			Installed:        installed,
			InstalledVersion: dep.Dependency.Version,
			Satisfied:        installed && compareVersions(dep.Dependency.Version, dep.RequiredVersion) >= 0,
		})
	}
	return result, nil
}

func (m *Marketplace) AddDependency(pluginID, dependencyID uint, requiredVersion string, optional bool) error {
	var exists db.PluginDependency
	if err := m.db.Where("plugin_id = ? AND dependency_id = ?", pluginID, dependencyID).First(&exists).Error; err == nil {
		return fmt.Errorf("dependency already exists")
	}

	dep := db.PluginDependency{
		PluginID:        pluginID,
		DependencyID:    dependencyID,
		RequiredVersion: requiredVersion,
		Optional:        optional,
	}
	return m.db.Create(&dep).Error
}

func (m *Marketplace) RemoveDependency(pluginID, dependencyID uint) error {
	return m.db.Where("plugin_id = ? AND dependency_id = ?", pluginID, dependencyID).Delete(&db.PluginDependency{}).Error
}

func (m *Marketplace) ResolveInstallationOrder(pluginID uint) ([]uint, error) {
	var plugin db.Plugin
	if err := m.db.First(&plugin, pluginID).Error; err != nil {
		return nil, err
	}

	visited := make(map[uint]bool)
	inStack := make(map[uint]bool)
	var order []uint

	var dfs func(id uint) error
	dfs = func(id uint) error {
		if visited[id] {
			return nil
		}
		if inStack[id] {
			return fmt.Errorf("circular dependency detected")
		}

		inStack[id] = true

		var deps []db.PluginDependency
		if err := m.db.Where("plugin_id = ?", id).Find(&deps).Error; err != nil {
			return err
		}

		for _, dep := range deps {
			if !dep.Optional {
				if err := dfs(dep.DependencyID); err != nil {
					return err
				}
			}
		}

		inStack[id] = false
		visited[id] = true
		order = append(order, id)

		return nil
	}

	if err := dfs(pluginID); err != nil {
		return nil, err
	}

	return order, nil
}

func (m *Marketplace) CheckAllUpdates() {
	var plugins []db.Plugin
	if err := m.db.Find(&plugins).Error; err != nil {
		slog.Error("Failed to fetch plugins for update check", "err", err)
		return
	}

	for _, plugin := range plugins {
		if err := m.CheckPluginUpdate(&plugin); err != nil {
			slog.Debug("Update check failed for plugin", "plugin", plugin.Name, "err", err)
		}
	}
}

func (m *Marketplace) CheckPluginUpdate(plugin *db.Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var status db.PluginUpdateStatus
	if err := m.db.FirstOrCreate(&status, db.PluginUpdateStatus{PluginID: plugin.ID}).Error; err != nil {
		return err
	}

	latestVersion, releaseNotes, err := m.fetchLatestVersion(plugin)
	if err != nil {
		status.LastCheckedAt = time.Now()
		m.db.Save(&status)
		return err
	}

	status.LastCheckedAt = time.Now()
	status.LatestVersion = latestVersion
	status.ReleaseNotes = releaseNotes

	if latestVersion != "" && compareVersions(latestVersion, plugin.Version) > 0 {
		status.UpdateAvailable = true
	} else {
		status.UpdateAvailable = false
	}

	return m.db.Save(&status).Error
}

func (m *Marketplace) fetchLatestVersion(plugin *db.Plugin) (string, string, error) {
	if plugin.Homepage == "" {
		return "", "", nil
	}

	if strings.Contains(plugin.Homepage, "github.com") {
		return m.fetchGitHubLatestVersion(plugin.Homepage)
	}

	return "", "", nil
}

func (m *Marketplace) fetchGitHubLatestVersion(homepage string) (string, string, error) {
	re := regexp.MustCompile(`github\.com/([^/]+)/([^/#?]+)`)
	matches := re.FindStringSubmatch(homepage)
	if len(matches) < 3 {
		return "", "", nil
	}
	owner, repo := matches[1], strings.TrimSuffix(matches[2], ".git")
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ForgeC2-Plugin-Marketplace")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("github api status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", err
	}

	var release struct {
		TagName     string `json:"tag_name"`
		HTMLURL     string `json:"html_url"`
		Body        string `json:"body"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return "", "", err
	}
	return release.TagName, release.HTMLURL, nil
}

func (m *Marketplace) GetUpdateStatus(pluginID uint) (*db.PluginUpdateStatus, error) {
	var status db.PluginUpdateStatus
	err := m.db.FirstOrCreate(&status, db.PluginUpdateStatus{PluginID: pluginID}).Error
	return &status, err
}

func (m *Marketplace) UpdatePlugin(pluginID uint) error {
	var plugin db.Plugin
	if err := m.db.First(&plugin, pluginID).Error; err != nil {
		return err
	}

	var status db.PluginUpdateStatus
	if err := m.db.First(&status, "plugin_id = ?", pluginID).Error; err != nil {
		return err
	}

	if !status.UpdateAvailable || status.LatestVersion == "" {
		return fmt.Errorf("no update available")
	}

	plugin.Version = status.LatestVersion
	status.UpdateAvailable = false

	return m.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&plugin).Error; err != nil {
			return err
		}
		return tx.Save(&status).Error
	})
}

func (m *Marketplace) ExportPlugin(pluginID uint) ([]byte, error) {
	var plugin db.Plugin
	if err := m.db.First(&plugin, pluginID).Error; err != nil {
		return nil, err
	}

	var deps []db.PluginDependency
	if err := m.db.Where("plugin_id = ?", pluginID).Find(&deps).Error; err != nil {
		return nil, err
	}

	type exportDep struct {
		Name            string `json:"name"`
		RequiredVersion string `json:"required_version"`
		Optional        bool   `json:"optional"`
	}

	exportDeps := make([]exportDep, 0, len(deps))
	for _, dep := range deps {
		var depPlugin db.Plugin
		if err := m.db.First(&depPlugin, dep.DependencyID).Error; err == nil {
			exportDeps = append(exportDeps, exportDep{
				Name:            depPlugin.Name,
				RequiredVersion: dep.RequiredVersion,
				Optional:        dep.Optional,
			})
		}
	}

	type exportData struct {
		Plugin       db.Plugin   `json:"plugin"`
		Dependencies []exportDep `json:"dependencies"`
	}

	data := exportData{
		Plugin:       plugin,
		Dependencies: exportDeps,
	}

	return json.MarshalIndent(data, "", "  ")
}

func (m *Marketplace) ImportPlugin(data []byte) (*db.Plugin, error) {
	type importDep struct {
		Name            string `json:"name"`
		RequiredVersion string `json:"required_version"`
		Optional        bool   `json:"optional"`
	}

	type importData struct {
		Plugin       db.Plugin   `json:"plugin"`
		Dependencies []importDep `json:"dependencies"`
	}

	var importDataObj importData
	if err := json.Unmarshal(data, &importDataObj); err != nil {
		return nil, err
	}

	plugin := importDataObj.Plugin
	plugin.ID = 0

	if err := m.db.Create(&plugin).Error; err != nil {
		return nil, err
	}

	for _, dep := range importDataObj.Dependencies {
		var depPlugin db.Plugin
		if err := m.db.Where("name = ?", dep.Name).First(&depPlugin).Error; err != nil {
			slog.Warn("Dependency not found, skipping", "name", dep.Name)
			continue
		}

		if err := m.AddDependency(plugin.ID, depPlugin.ID, dep.RequiredVersion, dep.Optional); err != nil {
			slog.Warn("Failed to add dependency", "err", err)
		}
	}

	return &plugin, nil
}

func (m *Marketplace) GetCategories() ([]string, error) {
	var categories []string
	err := m.db.Model(&db.Plugin{}).Distinct("category").Pluck("category", &categories).Error
	return categories, err
}

func (m *Marketplace) ListPluginsByCategory(category string) ([]db.Plugin, error) {
	var plugins []db.Plugin
	if category == "" {
		err := m.db.Find(&plugins).Error
		return plugins, err
	}
	err := m.db.Where("category = ?", category).Find(&plugins).Error
	return plugins, err
}

func compareVersions(v1, v2 string) int {
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &n1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &n2)
		}
		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}
	return 0
}
