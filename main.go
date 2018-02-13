package main

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/apex/log"
	"github.com/apex/log/handlers/papertrail"
	"github.com/blockloop/darksky-alexa/alexa"
	"github.com/blockloop/darksky-alexa/cache"
	"github.com/blockloop/darksky-alexa/darksky"
	"github.com/blockloop/darksky-alexa/geo"
	"github.com/blockloop/darksky-alexa/handlers"
	"github.com/blockloop/darksky-alexa/ratelimiter"
	"github.com/blockloop/tea"
	"github.com/caarlos0/env"
	"github.com/garyburd/redigo/redis"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
	"github.com/pkg/errors"
)

var (
	config = struct {
		DarkskyToken     string `env:"DARKSKY_TOKEN,required"`
		RedisURL         string `env:"REDIS_URL"`
		RedisMaxIdle     int    `env:"REDIS_MAX_IDLE" envDefault:"5"`
		Port             int    `env:"PORT", envDefault:"3000"`
		RequestsPerDay   int64  `env:"REQUESTS_PER_DAY" envDefault:"1000"`
		IPRequestsPerDay int64  `env:"IP_REQUESTS_PER_DAY" envDefault:"50"`
		MockZipcode      string `env:"MOCK_ZIP_CODE"`
		Env              string `env:"ENV" envDefault:"development"`
		PapertrailAddr   string `env:"PAPERTRAIL_DEST"`
	}{}
)

func init() {
}

func main() {
	initLogging(config.PapertrailAddr)
	if err := env.Parse(&config); err != nil {
		log.WithError(err).Fatal("configuration failure")
	}
	if config.RedisURL == "" {
		server, err := miniredis.Run()
		if err != nil {
			log.WithError(err).Fatal("failed to start miniredis")
		}
		defer server.Close()
		config.RedisURL = "redis://" + server.Addr()
		config.RedisMaxIdle = 1
		log.WithField("miniredis.url", config.RedisURL).Info("using miniredis")
	}

	tea.Responder = render.JSON

	redisPool := initRedis(config.RedisURL, config.RedisMaxIdle)
	defer redisPool.Close()

	geodb := geo.New(redisPool)
	dsapi := darksky.New(config.DarkskyToken)
	redisCache := cache.NewRedis(redisPool)
	cachedDarksky := cache.NewWriteThrough(redisCache, dsapi)
	alexaAPI := initAlexaAPI(config.MockZipcode)

	mux := chi.NewMux()
	mux.Use(
		middleware.RealIP,
		middleware.RequestID,
		middleware.Timeout(time.Second*10),
		middleware.Logger,
		middleware.Recoverer,
	)

	mux.Get("/ping", handlers.Ping)

	rl := ratelimiter.New(redisPool, config.RequestsPerDay, config.IPRequestsPerDay)
	mux.With(rl).Post("/darksky", handlers.Alexa(alexaAPI, geodb, cachedDarksky))

	addr := ":3000"
	log.WithField("addr", addr).Info("server started")
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.WithError(err).Fatalf("shutting down")
	}
	log.Info("shutting down")
}

func initAlexaAPI(mockZip string) alexa.API {
	if mockZip == "" {
		return alexa.NewAPI()
	}
	return NewStubZipcodeLoader(mockZip)
}

func initRedis(url string, maxIdle int) *redis.Pool {
	dialRedis := func() (redis.Conn, error) {
		c, err := redis.DialURL(config.RedisURL, redis.DialConnectTimeout(5*time.Second))
		return c, errors.Wrap(err, "failed to dial redis")
	}
	if _, err := dialRedis(); err != nil {
		log.WithError(err).Fatal("failed to dial redis")
	}
	return redis.NewPool(dialRedis, maxIdle)
}

func initLogging(papertrailAddr string) {
	if papertrailAddr == "" {
		return
	}
	host, portstr, err := net.SplitHostPort(papertrailAddr)
	if err != nil || !strings.Contains(host, ".papertrailapp.com") {
		log.WithError(err).WithField("addr", papertrailAddr).
			Fatal("invalid papertrail address should be like logs2.papertrailapp.com:33078")
	}
	port, err := strconv.Atoi(portstr)
	if err != nil {
		log.WithError(err).WithField("addr", papertrailAddr).
			Fatal("invalid papertrail address: no port was found")
	}

	cname := strings.Split(host, ".")[0]
	hostname, err := os.Hostname()
	if err != nil {
		log.WithError(err).Info("failed to detect hostname")
	}

	conf := &papertrail.Config{
		Host:     cname,
		Port:     port,
		Hostname: hostname,
		Tag:      "darksky-alexa",
	}

	log.SetHandler(papertrail.New(conf))
}
