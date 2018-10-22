package hotlink

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

// WhitelistPOSTRequest WhitelistPOSTRequest
type WhitelistPOSTRequest struct {
	Host string `json:"host"`
}

// BlacklistPOSTRequest BlacklistPOSTRequest
type BlacklistPOSTRequest struct {
	Host string `json:"host"`
}

// SetupRouter SetupRouter
func (s *Hotlink) SetupRouter(r *gin.Engine) {
	r.GET("/hotlink/monitoring", func(c *gin.Context) {
		list, err := s.TopData()
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		c.JSON(http.StatusOK, list)
	})

	r.DELETE("/hotlink/monitoring", func(c *gin.Context) {

		host, exists := c.Params.Get("host")
		var err error
		if exists {
			err = s.DeleteByHost(host)
		} else {
			err = s.Delete()
		}
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		c.Status(http.StatusNoContent)
	})

	r.POST("hotlink/whitelist", func(c *gin.Context) {

		request := WhitelistPOSTRequest{}
		err := c.BindJSON(&request)

		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}

		err = s.AddToWhitelist(request.Host)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		c.Header("Location", "/hotlink/whitelist/"+url.PathEscape(request.Host))

		c.Status(http.StatusCreated)
	})

	r.POST("hotlink/blacklist", func(c *gin.Context) {

		request := BlacklistPOSTRequest{}
		err := c.BindJSON(&request)

		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}

		err = s.AddToWhitelist(request.Host)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		c.Header("Location", "/hotlink/blacklist/"+url.PathEscape(request.Host))

		c.Status(http.StatusCreated)
	})
}
