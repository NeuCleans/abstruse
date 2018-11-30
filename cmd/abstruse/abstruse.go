package main

import (
	"flag"
	"log"
	"os"

	"github.com/bleenco/abstruse/server"
)

var (
	httpAddr     = flag.String("http", "0.0.0.0:80", "HTTP listen address")
	httpsAddr    = flag.String("https", "0.0.0.0:443", "HTTPS listen address")
	certFile     = flag.String("cert", "", "path to cert file")
	keyFile      = flag.String("certkey", "", "path to cert key file")
	grpcPort     = flag.Int("grpc-port", 3330, "gRPC server port")
	gRPCCertFile = flag.String("grpc-cert", "", "path to cert file")
	gRPCKeyFile  = flag.String("grpc-certkey", "", "path to cert key file")
)

func main() {
	flag.Parse()

	config := &server.AbstruseConfig{
		HTTPAddress:  *httpAddr,
		HTTPSAddress: *httpsAddr,
		CertFile:     *certFile,
		KeyFile:      *keyFile,
		GRPCConfig: &server.GRPCServerConfig{
			Port:    *grpcPort,
			Cert:    *gRPCCertFile,
			CertKey: *gRPCKeyFile,
		},
	}
	abstruse, _ := server.NewAbstruse(config)

	if err := abstruse.Run(); err != nil {
		log.Fatal(err)
	}

	os.Exit(0)
}
