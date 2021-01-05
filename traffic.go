package traffic

import (
	"fmt"
	"github.com/autowp/traffic/util"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/streadway/amqp"
	"net"
	"net/http"
	"sync"
	"time"
)

const banByUserID = 9

// Traffic Traffic
type Traffic struct {
	Monitoring          *Monitoring
	Whitelist           *Whitelist
	Ban                 *Ban
	whitelistStopTicker chan bool
	banStopTicker       chan bool
	logger              *util.Logger
}

// AutobanProfile AutobanProfile
type AutobanProfile struct {
	Limit  int
	Reason string
	Group  []string
	Time   time.Duration
}

// AutobanProfiles AutobanProfiles
var AutobanProfiles = []AutobanProfile{
	{
		Limit:  10000,
		Reason: "daily limit",
		Group:  []string{},
		Time:   time.Hour * 10 * 24,
	},
	{
		Limit:  3600,
		Reason: "hourly limit",
		Group:  []string{"hour"},
		Time:   time.Hour * 5 * 24,
	},
	{
		Limit:  1200,
		Reason: "ten min limit",
		Group:  []string{"hour", "tenminute"},
		Time:   time.Hour * 24,
	},
	{
		Limit:  700,
		Reason: "min limit",
		Group:  []string{"hour", "tenminute", "minute"},
		Time:   time.Hour * 12,
	},
}

// BanPOSTRequest BanPOSTRequest
type BanPOSTRequest struct {
	IP       net.IP        `json:"ip"`
	Duration time.Duration `json:"duration"`
	ByUserID int           `json:"by_user_id"`
	Reason   string        `json:"reason"`
}

// WhitelistPOSTRequest WhitelistPOSTRequest
type WhitelistPOSTRequest struct {
	IP          net.IP `json:"ip"`
	Description string `json:"description"`
}

// TopItem TopItem
type TopItem struct {
	IP          net.IP   `json:"ip"`
	Count       int      `json:"count"`
	Ban         *BanItem `json:"ban"`
	InWhitelist bool     `json:"in_whitelist"`
}

// NewTraffic constructor
func NewTraffic(wg *sync.WaitGroup, pool *pgxpool.Pool, rabbitMQ *amqp.Connection, logger *util.Logger, monitoringQueue string) (*Traffic, error) {

	ban, err := NewBan(wg, pool, logger)
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}

	monitoring, err := NewMonitoring(wg, pool, rabbitMQ, monitoringQueue, logger)
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}

	whitelist, err := NewWhitelist(pool)
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}

	s := &Traffic{
		Monitoring: monitoring,
		Whitelist:  whitelist,
		Ban:        ban,
		logger:     logger,
	}

	whitelistTicker := time.NewTicker(whitelistPeriod)
	s.whitelistStopTicker = make(chan bool)
	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("AutoWhitelist scheduler started")
	loop:
		for {
			select {
			case <-whitelistTicker.C:
				err := s.AutoWhitelist()
				if err != nil {
					logger.Warning(err)
				}
			case <-s.whitelistStopTicker:
				whitelistTicker.Stop()
				break loop
			}
		}
		fmt.Println("AutoWhitelist scheduler stopped")
	}()

	banTicker := time.NewTicker(banPeriod)
	s.banStopTicker = make(chan bool)
	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("AutoBan scheduler started")
	loop:
		for {
			select {
			case <-banTicker.C:
				err := s.AutoBan()
				if err != nil {
					logger.Warning(err)
				}
			case <-s.banStopTicker:
				banTicker.Stop()
				break loop
			}
		}

		fmt.Println("AutoBan scheduler stopped")
	}()

	return s, nil
}

func (s *Traffic) AutoBanByProfile(profile AutobanProfile) error {

	ips, err := s.Monitoring.ListByBanProfile(profile)
	if err != nil {
		return err
	}

	for _, ip := range ips {
		exists, err := s.Whitelist.Exists(ip)
		if err != nil {
			return err
		}
		if exists {
			continue
		}

		fmt.Printf("%s %v\n", profile.Reason, ip)

		if err := s.Ban.Add(ip, profile.Time, banByUserID, profile.Reason); err != nil {
			return err
		}
	}

	return nil
}

func (s *Traffic) AutoBan() error {
	for _, profile := range AutobanProfiles {
		if err := s.AutoBanByProfile(profile); err != nil {
			return err
		}
	}

	return nil
}

func (s *Traffic) AutoWhitelist() error {

	items, err := s.Monitoring.ListOfTop(1000)
	if err != nil {
		return err
	}

	for _, item := range items {
		fmt.Printf("Check IP %v\n", item.IP)
		if err := s.AutoWhitelistIP(item.IP); err != nil {
			return err
		}
	}

	return nil
}

func (s *Traffic) AutoWhitelistIP(ip net.IP) error {
	ipText := ip.String()

	fmt.Print(ipText + ": ")

	inWhitelist, err := s.Whitelist.Exists(ip)
	if err != nil {
		return err
	}

	match, desc := s.Whitelist.MatchAuto(ip)

	if !match {
		fmt.Println("")
		return nil
	}

	if inWhitelist {
		fmt.Println("whitelist, skip")
	} else {
		if err := s.Whitelist.Add(ip, desc); err != nil {
			return err
		}
	}

	if err := s.Ban.Remove(ip); err != nil {
		return err
	}

	if err := s.Monitoring.ClearIP(ip); err != nil {
		return err
	}

	fmt.Println(" whitelisted")

	return nil
}

func (s *Traffic) SetupRouter(r *gin.Engine) {
	r.GET("/whitelist", func(c *gin.Context) {
		list, err := s.Whitelist.List()
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		c.JSON(http.StatusOK, list)
	})

	r.POST("/whitelist", func(c *gin.Context) {

		request := WhitelistPOSTRequest{}
		err := c.BindJSON(&request)

		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}

		err = s.Whitelist.Add(request.IP, request.Description)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		err = s.Ban.Remove(request.IP)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		c.Header("Location", "/whitelist/"+request.IP.String())

		c.Status(http.StatusCreated)
	})

	r.GET("/whitelist/:ip", func(c *gin.Context) {
		ip := net.ParseIP(c.Param("ip"))
		if ip == nil {
			c.String(http.StatusBadRequest, "Invalid IP")
			return
		}

		item, err := s.Whitelist.Get(ip)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		if item == nil {
			c.Status(http.StatusNotFound)
			return
		}

		c.JSON(http.StatusOK, item)
	})

	r.DELETE("/whitelist/:ip", func(c *gin.Context) {
		ip := net.ParseIP(c.Param("ip"))
		if ip == nil {
			c.String(http.StatusBadRequest, "Invalid IP")
			return
		}

		err := s.Whitelist.Remove(ip)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		c.Status(http.StatusNoContent)
	})

	r.GET("/top", func(c *gin.Context) {

		items, err := s.Monitoring.ListOfTop(50)

		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		result := make([]TopItem, len(items))
		for idx, item := range items {

			ban, err := s.Ban.Get(item.IP)
			if err != nil {
				c.String(http.StatusInternalServerError, err.Error())
				return
			}

			inWhitelist, err := s.Whitelist.Exists(item.IP)
			if err != nil {
				c.String(http.StatusInternalServerError, err.Error())
				return
			}

			result[idx] = TopItem{
				IP:          item.IP,
				Count:       item.Count,
				Ban:         ban,
				InWhitelist: inWhitelist,
			}
		}

		c.JSON(http.StatusOK, result)
	})

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
}

// Close Destructor
func (s *Traffic) Close() error {
	s.banStopTicker <- true
	close(s.banStopTicker)
	s.whitelistStopTicker <- true
	close(s.whitelistStopTicker)

	err := s.Ban.Close()
	if err != nil {
		s.logger.Warning(err)
	}
	err = s.Monitoring.Close()
	if err != nil {
		s.logger.Warning(err)
	}

	return nil
}
