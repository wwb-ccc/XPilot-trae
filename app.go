package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"xpilot/backend/db"
	"xpilot/backend/engine"
	"xpilot/backend/tracker"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx     context.Context
	cancel  context.CancelFunc
	db      *db.Database
	tracker *tracker.Tracker
	checker *engine.AchievementChecker
}

func NewApp() *App {
	database, err := db.NewDatabase("")
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	checker := engine.NewAchievementChecker(database)

	a := &App{
		db:      database,
		checker: checker,
	}

	a.tracker = tracker.NewTracker(a.onSessionEnd)

	return a
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Create cancelable context for tracker
	trackerCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	// Start tracker in background
	go a.tracker.Start(trackerCtx)

	// Start systray in background
	go systray.Run(onSystrayReady, onSystrayExit)
}

// onSessionEnd is called when a tracking session ends
func (a *App) onSessionEnd(processName, windowTitle string, startTime, endTime time.Time) {
	cat := engine.GetCategory(processName)
	durationSeconds := int(endTime.Sub(startTime).Seconds())
	xp := engine.CalculateXp(cat, durationSeconds)

	_, err := a.db.InsertSession(processName, windowTitle, cat, startTime, endTime, durationSeconds, xp)
	if err != nil {
		log.Printf("Failed to insert session: %v", err)
		return
	}

	today := time.Now().Format("2006-01-02")
	minutes := durationSeconds / 60
	_ = a.db.UpdateDailyStats(today, minutes, xp)

	todaySessions, err := a.db.GetSessionsByDate(today)
	if err == nil {
		newAchievements, err := a.checker.CheckAchievements(todaySessions)
		if err == nil && len(newAchievements) > 0 {
			for _, ach := range newAchievements {
				log.Printf("Achievement unlocked: %s (%s)", ach.Name, ach.Description)
				a.showNotificationWindow(ach.Name, ach.Description, ach.Icon, ach.XpReward)
			}
		}
	}
}

// shutdown is called when the app is shutting down
func (a *App) shutdown(ctx context.Context) {
	if a.cancel != nil {
		a.cancel()
	}
	if a.db != nil {
		a.db.Close()
	}
}

// -------- Bound Methods (called by frontend) --------

// GetNow returns current active session data
func (a *App) GetNow() map[string]interface{} {
	win := a.tracker.GetCurrentWindow()
	duration := a.tracker.GetCurrentDuration()
	isTracking := win.ProcessName != ""

	cat := engine.GetCategory(win.ProcessName)
	xp := engine.CalculateXp(cat, int(duration.Seconds()))
	minutes := int(duration.Minutes())

	totalXp, _ := a.db.GetTotalXp()
	level, currentXp, nextLevelXp, progress := engine.GetLevelProgress(totalXp + xp)

	return map[string]interface{}{
		"app":             a.getChineseName(win.ProcessName),
		"processName":     win.ProcessName,
		"title":           win.WindowTitle,
		"windowTitle":     win.WindowTitle,
		"category":        cat,
		"minutes":         minutes,
		"durationSec":     int(duration.Seconds()),
		"xp":              xp,
		"isTracking":      isTracking,
		"level":           level,
		"currentXp":       currentXp,
		"nextLevelXp":     nextLevelXp,
		"progressPercent": progress,
	}
}

type sessionWithName struct {
	db.Session
	App string `json:"app"`
}

// GetTodaySessions returns today's sessions
func (a *App) GetTodaySessions() []sessionWithName {
	today := time.Now().Format("2006-01-02")
	sessions, err := a.db.GetSessionsByDate(today)
	if err != nil || sessions == nil {
		return []sessionWithName{}
	}

	result := make([]sessionWithName, len(sessions))
	for i, s := range sessions {
		result[i] = sessionWithName{
			Session: s,
			App:     a.getChineseName(s.ProcessName),
		}
	}
	return result
}

// GetTodaySummary returns today's summary
func (a *App) GetTodaySummary() map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	sessions, err := a.db.GetSessionsByDate(today)
	if err != nil {
		sessions = []db.Session{}
	}

	totalMin := 0
	totalXp := 0
	appSet := make(map[string]bool)
	catMinutes := make(map[string]int)

	for _, s := range sessions {
		totalMin += s.DurationSeconds / 60
		totalXp += s.Xp
		appSet[s.ProcessName] = true
		catMinutes[s.Category] += s.DurationSeconds / 60
	}

	stats, _ := a.db.GetDailyStats(today)
	achCount := 0
	if stats != nil {
		achCount = stats.AchievementCount
	}

	return map[string]interface{}{
		"totalMinutes":     totalMin,
		"totalXp":          totalXp,
		"appCount":         len(appSet),
		"sessionCount":     len(sessions),
		"achievementCount": achCount,
		"categories":       catMinutes,
	}
}

type appRank struct {
	Name       string  `json:"name"`
	App        string  `json:"app"`
	Minutes    int     `json:"minutes"`
	Xp         int     `json:"xp"`
	Category   string  `json:"category"`
	Percentage float64 `json:"percentage"`
}

// GetAppsRank returns today's app rankings
func (a *App) GetAppsRank() []appRank {
	today := time.Now().Format("2006-01-02")
	sessions, err := a.db.GetSessionsByDate(today)
	if err != nil {
		return []appRank{}
	}

	type appStat struct {
		Minutes  int
		Xp       int
		Category string
	}
	appStats := make(map[string]*appStat)

	for _, s := range sessions {
		if _, ok := appStats[s.ProcessName]; !ok {
			appStats[s.ProcessName] = &appStat{Category: s.Category}
		}
		appStats[s.ProcessName].Minutes += s.DurationSeconds / 60
		appStats[s.ProcessName].Xp += s.Xp
	}

	var ranks []appRank
	maxMinutes := 0
	for _, stat := range appStats {
		if stat.Minutes > maxMinutes {
			maxMinutes = stat.Minutes
		}
	}

	for name, stat := range appStats {
		pct := 0.0
		if maxMinutes > 0 {
			pct = float64(stat.Minutes) / float64(maxMinutes) * 100.0
		}
		ranks = append(ranks, appRank{
			Name:       name,
			App:        a.getChineseName(name),
			Minutes:    stat.Minutes,
			Xp:         stat.Xp,
			Category:   stat.Category,
			Percentage: pct,
		})
	}

	sort.Slice(ranks, func(i, j int) bool {
		return ranks[i].Minutes > ranks[j].Minutes
	})

	return ranks
}

type weekStat struct {
	Date             string `json:"date"`
	TotalMinutes     int    `json:"totalMinutes"`
	TotalXp          int    `json:"totalXp"`
	AchievementCount int    `json:"achievementCount"`
}

// GetWeekStats returns last 7 days stats
func (a *App) GetWeekStats() []weekStat {
	stats, err := a.db.GetWeekStats()
	if err != nil {
		stats = []db.DailyStats{}

	}

	today := time.Now()
	statsMap := make(map[string]db.DailyStats)
	for _, s := range stats {
		statsMap[s.Date] = s
	}

	var result []weekStat
	for i := 6; i >= 0; i-- {
		date := today.AddDate(0, 0, -i).Format("2006-01-02")
		if s, ok := statsMap[date]; ok {
			result = append(result, weekStat{
				Date:             s.Date,
				TotalMinutes:     s.TotalMinutes,
				TotalXp:          s.TotalXp,
				AchievementCount: s.AchievementCount,
			})
		} else {
			result = append(result, weekStat{
				Date:             date,
				TotalMinutes:     0,
				TotalXp:          0,
				AchievementCount: 0,
			})
		}
	}
	return result
}

type achievementStatus struct {
	db.Achievement
	Unlocked bool `json:"unlocked"`
}

// GetAchievements returns all achievements with unlock status
func (a *App) GetAchievements() []achievementStatus {
	achievements, err := a.db.GetAchievements()
	if err != nil {
		return []achievementStatus{}
	}

	unlockedIDs, err := a.db.GetUnlockedAchievementIDs()
	if err != nil {
		unlockedIDs = []int{}
	}

	unlockedSet := make(map[int]bool)
	for _, id := range unlockedIDs {
		unlockedSet[id] = true
	}

	var result []achievementStatus
	for _, ach := range achievements {
		result = append(result, achievementStatus{
			Achievement: ach,
			Unlocked:    unlockedSet[ach.ID],
		})
	}
	return result
}

// GetProfile returns user profile data
func (a *App) GetProfile() map[string]interface{} {
	totalXp, err := a.db.GetTotalXp()
	if err != nil {
		totalXp = 0
	}

	level, currentXp, nextLevelXp, progress := engine.GetLevelProgress(totalXp)

	streakDays, _ := a.db.GetStreakDays()

	today := time.Now().Format("2006-01-02")
	stats, _ := a.db.GetDailyStats(today)
	todayXp := 0
	todayMin := 0
	if stats != nil {
		todayXp = stats.TotalXp
		todayMin = stats.TotalMinutes
	}

	unlocked, _ := a.db.GetUnlockedAchievements()
	achCount := len(unlocked)

	allSessions, _ := a.db.GetSessionsByDate(today)
	sessionCount := len(allSessions)

	return map[string]interface{}{
		"level":           level,
		"totalXp":         totalXp,
		"currentXp":       currentXp,
		"nextLevelXp":     nextLevelXp,
		"progressPercent": progress,
		"streakDays":      streakDays,
		"todayXp":         todayXp,
		"todayMinutes":    todayMin,
		"achievements":    achCount,
		"sessions":        sessionCount,
	}
}

// GetAppNames returns process name to Chinese name mapping
func (a *App) GetAppNames() map[string]string {
	names, _ := a.db.GetAppNames()
	return names
}

// GetSkills returns all-time skill levels
func (a *App) GetSkills() []engine.Skill {
	skills, err := engine.GetAllTimeSkills(a.db)
	if err != nil {
		return []engine.Skill{}
	}
	return skills
}

type catItem struct {
	Category    string  `json:"category"`
	CategoryKey string  `json:"categoryKey"`
	Minutes     int     `json:"minutes"`
	Percentage  float64 `json:"percentage"`
}

// GetCategoryBreakdown returns category breakdown for pie chart
func (a *App) GetCategoryBreakdown() map[string]interface{} {
	today := time.Now().Format("2006-01-02")
	sessions, err := a.db.GetSessionsByDate(today)
	if err != nil {
		sessions = []db.Session{}
	}

	catMinutes := make(map[string]int)
	total := 0
	for _, s := range sessions {
		catMinutes[s.Category] += s.DurationSeconds / 60
		total += s.DurationSeconds / 60
	}

	var breakdown []catItem
	catOrder := []string{"office", "reading", "social", "gaming", "other"}
	catNames := map[string]string{
		"office": "办公", "reading": "阅读", "social": "社交",
		"gaming": "游戏", "other": "其他",
	}

	for _, cat := range catOrder {
		min := catMinutes[cat]
		pct := 0.0
		if total > 0 {
			pct = float64(min) / float64(total) * 100.0
		}
		breakdown = append(breakdown, catItem{
			Category:    catNames[cat],
			CategoryKey: cat,
			Minutes:     min,
			Percentage:  pct,
		})
	}

	return map[string]interface{}{
		"totalMinutes": total,
		"categories":   breakdown,
	}
}

// TestUnlockAchievement tests an achievement unlock notification
func (a *App) TestUnlockAchievement() map[string]interface{} {
	achievements, err := a.db.GetAchievements()
	if err != nil || len(achievements) == 0 {
		return map[string]interface{}{
			"message": "没有可用的成就",
		}
	}

	unlockedIDs, _ := a.db.GetUnlockedAchievementIDs()
	unlockedSet := make(map[int]bool)
	for _, id := range unlockedIDs {
		unlockedSet[id] = true
	}

	// Find first unlocked achievement
	for _, ach := range achievements {
		if unlockedSet[ach.ID] {
			a.showNotificationWindow(ach.Name, ach.Description, ach.Icon, ach.XpReward)
			return map[string]interface{}{
				"message":       "测试成功",
				"achievement":   ach,
				"newlyUnlocked": []db.Achievement{ach},
			}
		}
	}

	// If none unlocked, unlock the first one
	if len(achievements) > 0 {
		ach := achievements[0]
		ok, _ := a.db.UnlockAchievement(ach.ID)
		if ok {
			today := time.Now().Format("2006-01-02")
			a.db.IncrementAchievementCount(today)
		}
		a.showNotificationWindow(ach.Name, ach.Description, ach.Icon, ach.XpReward)
		return map[string]interface{}{
			"message":       "已解锁测试成就",
			"achievement":   ach,
			"newlyUnlocked": []db.Achievement{ach},
		}
	}

	return map[string]interface{}{
		"message": "没有可用成就",
	}
}

// showAchievementNotification sends a native system notification via Wails runtime.
// This shows a real desktop-level toast (Windows Action Center / macOS Notification
// Center / Linux libnotify), completely independent of the Wails window.
func (a *App) showNotificationWindow(name, description, icon string, xpReward int) {
	if a.ctx == nil {
		return
	}

	// Initialize notifications (idempotent)
	if err := runtime.InitializeNotifications(a.ctx); err != nil {
		log.Printf("notif: init failed: %v", err)
	}

	body := description
	if xpReward > 0 {
		body = fmt.Sprintf("%s  +%d XP", description, xpReward)
	}

	// Use a stable ID so re-firing replaces the previous one instead of stacking
	notifID := "xpilot-achievement-" + name

	err := runtime.SendNotification(a.ctx, runtime.NotificationOptions{
		ID:    notifID,
		Title: fmt.Sprintf("✦ 成就解锁 · %s", name),
		Body:  body,
	})
	if err != nil {
		log.Printf("notif: send failed: %v", err)
	} else {
		log.Printf("notif: sent achievement %q", name)
	}
}

// getChineseName returns the Chinese name for a process
func (a *App) getChineseName(processName string) string {
	names, _ := a.db.GetAppNames()
	if name, ok := names[processName]; ok {
		return name
	}
	if len(processName) > 4 && processName[len(processName)-4:] == ".exe" {
		return processName[:len(processName)-4]
	}
	return processName
}

// HideWindow hides the app window (called from systray)
func (a *App) HideWindow() {
	if a.ctx != nil {
		runtime.WindowHide(a.ctx)
	}
}

// ShowWindow shows the app window (called from systray)
func (a *App) ShowWindow() {
	if a.ctx != nil {
		runtime.WindowShow(a.ctx)
	}
}
