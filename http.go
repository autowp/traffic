package traffic

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// BanPOSTRequest BanPOSTRequest
type BanPOSTRequest struct {
	IP       net.IP        `json:"ip"`
	Duration time.Duration `json:"duration"`
	ByUserID int           `json:"by_user_id"`
	Reason   string        `json:"reason"`
}

// GetRouter GetRouter
func (s *Service) GetRouter() *gin.Engine {
	return s.router
}

func (s *Service) setupRouter() {
	r := gin.Default()

	r.POST("/ban", func(c *gin.Context) {

		request := BanPOSTRequest{}
		err := c.BindJSON(&request)

		if err != nil {
			fmt.Println(err)
			c.String(http.StatusBadRequest, err.Error())
			return
		}

		err = s.Ban.Add(request.IP, request.Duration, request.ByUserID, request.Reason)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		c.Header("Location", "/ban/"+request.IP.String())

		c.Status(http.StatusCreated)
	})

	r.DELETE("/ban/:ip", func(c *gin.Context) {
		ip := net.ParseIP(c.Param("ip"))
		if ip == nil {
			c.String(http.StatusBadRequest, "Invalid IP")
			return
		}

		err := s.Ban.Remove(ip)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		c.Status(http.StatusNoContent)
	})

	r.GET("/ban/:ip", func(c *gin.Context) {
		ip := net.ParseIP(c.Param("ip"))
		if ip == nil {
			c.String(http.StatusBadRequest, "Invalid IP")
			return
		}

		ban, err := s.Ban.Get(ip)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		if ban == nil {
			c.Status(http.StatusNotFound)
			return
		}

		c.JSON(http.StatusOK, ban)
	})

	s.router = r
}
