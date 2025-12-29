package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/agents"
)

// AgentsV2Handler provides the new unified agent configuration API
type AgentsV2Handler struct {
	manager *agents.Manager
}

// NewAgentsV2Handler creates a new AgentsV2Handler
func NewAgentsV2Handler(port int) (*AgentsV2Handler, error) {
	manager, err := agents.NewManager(port)
	if err != nil {
		return nil, err
	}
	return &AgentsV2Handler{manager: manager}, nil
}

// GetAgents returns the list of detected CLI agents
// GET /api/v2/agents
func (h *AgentsV2Handler) GetAgents(c *gin.Context) {
	agentsList, err := h.manager.DetectAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"agents": agentsList})
}

// GetAgent returns info for a specific agent
// GET /api/v2/agents/:id
func (h *AgentsV2Handler) GetAgent(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing agent id"})
		return
	}

	info, err := h.manager.GetAgentInfo(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, info)
}

// EnableAgent enables ProxyPilot for an agent
// POST /api/v2/agents/:id/enable
func (h *AgentsV2Handler) EnableAgent(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing agent id"})
		return
	}

	result := h.manager.Enable(id)
	if !result.Success {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":      false,
			"error":        result.Error,
			"instructions": result.Instructions,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"agent_id":    result.AgentID,
		"message":     result.Message,
		"config_path": result.ConfigPath,
	})
}

// DisableAgent disables ProxyPilot for an agent
// POST /api/v2/agents/:id/disable
func (h *AgentsV2Handler) DisableAgent(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing agent id"})
		return
	}

	result := h.manager.Disable(id)
	if !result.Success {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"agent_id": result.AgentID,
		"message":  result.Message,
	})
}

// EnableAllAgents enables ProxyPilot for all detected agents
// POST /api/v2/agents/enable-all
func (h *AgentsV2Handler) EnableAllAgents(c *gin.Context) {
	results := h.manager.EnableAll()
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// DisableAllAgents disables ProxyPilot for all configured agents
// POST /api/v2/agents/disable-all
func (h *AgentsV2Handler) DisableAllAgents(c *gin.Context) {
	results := h.manager.DisableAll()
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// GetAgentState returns the current ProxyPilot state for all agents
// GET /api/v2/agents/state
func (h *AgentsV2Handler) GetAgentState(c *gin.Context) {
	states := h.manager.GetState().GetAllAgentStates()
	c.JSON(http.StatusOK, gin.H{"states": states})
}

// SetProxyPort updates the proxy port used by the manager
func (h *AgentsV2Handler) SetProxyPort(port int) {
	h.manager.SetProxyURL(port)
}
