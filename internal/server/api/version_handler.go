package api

import (
	"net/http"

	"hawkeye/internal/server/storage"

	"github.com/gin-gonic/gin"
)

// getVersionResponse is the response for the /api/v1/version endpoint.
type getVersionResponse struct {
	Version string             `json:"version"`
	Agents  []agentVersionInfo `json:"agents"`
}

type agentVersionInfo struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Online  bool   `json:"online"`
}

// getVersion returns the server version and all agent versions.
func getVersion(db *storage.DB, serverVersion string) gin.HandlerFunc {
	return func(c *gin.Context) {
		agents, err := db.GetAllAgents()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		agentInfos := make([]agentVersionInfo, 0, len(agents))
		for _, a := range agents {
			agentInfos = append(agentInfos, agentVersionInfo{
				ID:      a.ID,
				Name:    a.Name,
				Version: a.AgentVersion,
				Online:  a.Status == "online",
			})
		}

		c.JSON(http.StatusOK, getVersionResponse{
			Version: serverVersion,
			Agents:  agentInfos,
		})
	}
}
