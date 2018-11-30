package server

import (
	"log"
	"net/http"

	"github.com/bleenco/abstruse/db"
	humanize "github.com/dustin/go-humanize"
	"github.com/felixge/httpsnoop"

	"github.com/bleenco/abstruse/config"
	"github.com/bleenco/abstruse/fs"
	"github.com/bleenco/abstruse/logger"

	"golang.org/x/net/http2"
)

// AbstruseConfig defines configuration of the main master server.
type AbstruseConfig struct {
	HTTPAddress  string
	HTTPSAddress string
	CertFile     string
	KeyFile      string
	GRPCConfig    *GRPCServerConfig
	Dir          string
	Debug        bool
}

// Abstruse represents main master server.
type Abstruse struct {
	config *AbstruseConfig
	router *Router
	server *http.Server
	logger *logger.Logger
	grpcserver *GRPCServer
	dir    string

	running chan error
}

// NewAbstruse creates a new main master server instance.
func NewAbstruse(c *AbstruseConfig) (*Abstruse, error) {
	log := logger.NewLogger("", true, c.Debug)

	dir := c.Dir
	if dir == "" {
		dir, _ = fs.GetHomeDir()
		dir = dir + "/abstruse"
	}

	cfg := config.ReadAndParseConfig(dir + "/config.json")

	if c.CertFile == "" || c.KeyFile == "" {
		c.CertFile = cfg.Security.Cert
		c.KeyFile = cfg.Security.CertKey
	}

	if c.GRPCConfig.Port == 0 {
		c.GRPCConfig.Port = cfg.GRPC.Port
	}

	if c.GRPCConfig.Cert == "" || c.GRPCConfig.CertKey == "" {
		c.GRPCConfig.Cert = cfg.GRPC.Cert
		c.GRPCConfig.CertKey = cfg.GRPC.CertKey
	}

	gRPCServer, err := NewGRPCServer(c.GRPCConfig, log)
	if err != nil {
		return nil, err
	}
	gRPCServer.logger = log

	return &Abstruse{
		config:  c,
		router:  NewRouter(),
		server:  &http.Server{},
		grpcserver: gRPCServer,
		logger:  log,
		dir:     dir,
		running: make(chan error, 1),
	}, nil
}

// Run starts the main server instance.
func (a *Abstruse) Run() error {
	if err := a.init(); err != nil {
		return err
	}

	handler := http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		m := httpsnoop.CaptureMetrics(a.router, res, req)
		a.logger.Infof(
			"%s %s (code=%d dt=%s written=%s remote=%s)",
			req.Method,
			req.URL,
			m.Code,
			m.Duration,
			humanize.Bytes(uint64(m.Written)),
			req.RemoteAddr,
		)
	})

	if a.config.HTTPAddress != "" {
		go func() {
			a.logger.Infof("http listening on %s", a.config.HTTPAddress)
			log.Fatal(http.ListenAndServe(a.config.HTTPAddress, handler))
		}()
	}

	if a.config.HTTPSAddress != "" && a.config.CertFile != "" && a.config.KeyFile != "" {
		go func() {
			a.server.Addr = a.config.HTTPSAddress
			a.server.Handler = handler
			http2.ConfigureServer(a.server, nil)

			a.logger.Infof("https listening on %s", a.config.HTTPSAddress)
			log.Fatal(a.server.ListenAndServeTLS(a.config.CertFile, a.config.KeyFile))
		}()
	}

	if a.config.GRPCConfig.Cert != "" && a.config.GRPCConfig.CertKey != "" {
		go func() {
			if err := a.grpcserver.Listen(); err != nil {
				log.Fatal(err)
			}
		}()
	} else {
		log.Fatal("gRPC server cannot work without certificate and key path specified. exiting...")
	}

	return a.wait()
}

// Close gracefully stops main server.
func (a *Abstruse) Close() {
	a.logger.Infof("closing down http server...")
	if err := a.server.Close(); err != nil {
		a.running <- err
	}

	a.running <- nil
}

func (a *Abstruse) init() error {
	if !fs.Exists(a.dir) {
		if err := fs.MakeDir(a.dir); err != nil {
			return err
		}
	}

	configPath := a.dir + "/config.json"
	if !fs.Exists(configPath) {
		if err := config.WriteDefaultConfig(configPath); err != nil {
			return err
		}
	}

	config := config.ReadAndParseConfig(configPath)

	dbopts := db.Options{
		Client:   config.Database.Client,
		Hostname: config.Database.Host,
		Port:     config.Database.Port,
		User:     config.Database.User,
		Password: config.Database.Password,
		Database: config.Database.Name,
		Charset:  config.Database.Charset,
	}
	if err := db.Connect(dbopts); err != nil {
		return err
	}

	return nil
}

func (a *Abstruse) wait() error {
	return <-a.running
}
