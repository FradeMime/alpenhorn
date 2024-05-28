// Copyright 2017 David Lazar. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"alpenhorn/config"
	// Register the convo inner config.

	_ "vuvuzela.io/vuvuzela/convo"
)

var (
	hostname      = flag.String("hostname", "", "hostname of config server")
	setConfigPath = flag.String("setConfig", "", "path to signed config to make current")
	persistPath   = flag.String("persist", "persist_config_server", "persistent data directory")
)

func main() {
	flag.Parse()

	if err := os.MkdirAll(*persistPath, 0700); err != nil {
		log.Fatal(err)
	}
	serverPath := filepath.Join(*persistPath, "config-server-state")

	if *setConfigPath != "" {
		setConfig(serverPath)
		return
	}

	server, err := config.LoadServer(serverPath)
	if os.IsNotExist(err) {
		fmt.Println("No server state found. Please initialize server with -setConfig.")
		os.Exit(1)
	} else if err != nil {
		log.Fatalf("error loading server state: %s", err)
	}

	if *hostname == "" {
		fmt.Println("Please set -hostname.")
		os.Exit(1)
	}

	// certManager := autocert.Manager{
	// 	Cache:      autocert.DirCache(filepath.Join(*persistPath, "ssl")),
	// 	Prompt:     autocert.AcceptTOS,
	// 	HostPolicy: autocert.HostWhitelist(*hostname),
	// }
	// // Listen on :80 for http-01 ACME challenge.
	// go http.ListenAndServe(":http", certManager.HTTPHandler(nil))

	// httpServer := &http.Server{
	// 	Addr:      ":https",
	// 	Handler:   server,
	// 	TLSConfig: &tls.Config{GetCertificate: certManager.GetCertificate},

	// 	ReadTimeout:  10 * time.Second,
	// 	WriteTimeout: 10 * time.Second,
	// }

	// log.Printf("Listening on https://%s", *hostname)
	// log.Fatal(httpServer.ListenAndServeTLS("", ""))

	httpServer := &http.Server{
		Addr:    ":http",
		Handler: server,

		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Listening on http://%s", *hostname)
	log.Fatal(httpServer.ListenAndServe())
}

func setConfig(serverPath string) {
	data, err := ioutil.ReadFile(*setConfigPath)
	if err != nil {
		log.Fatal(err)
	}

	conf := new(config.SignedConfig)
	err = json.Unmarshal(data, conf)
	if err != nil {
		log.Fatalf("error decoding config: %s", err)
	}

	server, err := config.LoadServer(serverPath)
	if err == nil {
		err = server.SetCurrentConfig(conf)
		if err != nil {
			log.Fatalf("error setting config: %s", err)
		}
		fmt.Printf("Set current %q config in existing server state.\n", conf.Service)
	} else if os.IsNotExist(err) {
		server, err := config.CreateServer(serverPath)
		if err != nil {
			log.Fatalf("error creating server state: %s", err)
		}
		fmt.Printf("Created new config server state: %s\n", serverPath)

		err = server.SetCurrentConfig(conf)
		if err != nil {
			log.Fatalf("error setting config: %s", err)
		}
		fmt.Printf("Set current %q config in new state.\n", conf.Service)
	} else {
		log.Fatalf("unexpected error loading server state: %s", err)
	}
}

func printCertificateInfo(cert *tls.Certificate) {
	for _, certDER := range cert.Certificate {
		cert, err := x509.ParseCertificate(certDER)
		if err != nil {
			log.Printf("Failed to parse certificate: %v", err)
			continue
		}

		// 打印证书的基本信息
		fmt.Printf("Subject: %s\n", cert.Subject)
		fmt.Printf("Issuer: %s\n", cert.Issuer)
		fmt.Printf("Validity: NotBefore=%s, NotAfter=%s\n", cert.NotBefore, cert.NotAfter)
		fmt.Printf("DNS Names: %v\n", cert.DNSNames)

		// 打印证书的PEM编码
		pemBlock := &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}
		fmt.Printf("PEM:\n%s\n", string(pem.EncodeToMemory(pemBlock)))
	}
}
