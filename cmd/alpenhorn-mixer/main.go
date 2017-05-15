// Copyright 2016 David Lazar. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"text/template"

	log "github.com/Sirupsen/logrus"
	"golang.org/x/crypto/ed25519"

	"vuvuzela.io/alpenhorn/addfriend"
	"vuvuzela.io/alpenhorn/config"
	"vuvuzela.io/alpenhorn/dialing"
	"vuvuzela.io/alpenhorn/encoding/toml"
	"vuvuzela.io/alpenhorn/mixnet"
	"vuvuzela.io/alpenhorn/vrpc"
	"vuvuzela.io/crypto/rand"
)

var (
	globalConfPath = flag.String("global", "", "global config file")
	confPath       = flag.String("conf", "", "config file")
	doinit         = flag.Bool("init", false, "create config file")
)

type Config struct {
	ListenAddr string
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey

	AddFriendNoise rand.Laplace
	DialingNoise   rand.Laplace
}

var funcMap = template.FuncMap{
	"base32": toml.EncodeBytes,
}

const confTemplate = `# Alpenhorn mixnet server config

listenAddr = {{.ListenAddr | printf "%q"}}

publicKey  = {{.PublicKey | base32 | printf "%q"}}
privateKey = {{.PrivateKey | base32 | printf "%q"}}

[addFriendNoise]
mu = {{.AddFriendNoise.Mu | printf "%0.1f"}}
b = {{.AddFriendNoise.B | printf "%0.1f"}}

[dialingNoise]
mu = {{.DialingNoise.Mu | printf "%0.1f"}}
b = {{.DialingNoise.B | printf "%0.1f"}}
`

func writeNewConfig() {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}

	conf := &Config{
		ListenAddr: "0.0.0.0:28000",
		PublicKey:  publicKey,
		PrivateKey: privateKey,

		AddFriendNoise: rand.Laplace{
			Mu: 100,
			B:  3.0,
		},

		DialingNoise: rand.Laplace{
			Mu: 100,
			B:  3.0,
		},
	}

	tmpl := template.Must(template.New("config").Funcs(funcMap).Parse(confTemplate))

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, conf)
	if err != nil {
		log.Fatalf("template error: %s", err)
	}
	data := buf.Bytes()

	path := "mixer-init.conf"
	err = ioutil.WriteFile(path, data, 0600)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %s\n", path)
}

func init() {
	//log.SetFormatter(&log.JSONFormatter{})
}

func main() {
	flag.Parse()

	if *doinit {
		writeNewConfig()
		return
	}

	if *globalConfPath == "" {
		fmt.Println("specify global config file with -global")
		os.Exit(1)
	}

	if *confPath == "" {
		fmt.Println("specify config file with -conf")
		os.Exit(1)
	}

	globalConf, err := config.ReadGlobalConfigFile(*globalConfPath)
	if err != nil {
		log.Fatal(err)
	}
	alpConf, err := globalConf.AlpenhornConfig()
	if err != nil {
		log.Fatalf("error reading alpenhorn config from %q: %s", *globalConfPath, err)
	}
	if alpConf.CDN.Key == nil || alpConf.CDN.Address == "" {
		log.Fatal("alpenhorn cdn is missing a key or address")
	}
	coordinatorKey := alpConf.Coordinator.Key
	if coordinatorKey == nil {
		log.Fatal("no alpenhorn coordinator key specified in global config")
	}

	data, err := ioutil.ReadFile(*confPath)
	if err != nil {
		log.Fatal(err)
	}
	conf := new(Config)
	err = toml.Unmarshal(data, conf)
	if err != nil {
		log.Fatalf("error parsing config %q: %s", *confPath, err)
	}

	mixers := alpConf.Mixers
	ourPos := -1
	for i, mixer := range mixers {
		if bytes.Equal(mixer.Key, conf.PublicKey) {
			ourPos = i
			break
		}
	}
	if ourPos < 0 {
		log.Fatal("our key was not found in the alpenhorn mixer list")
	}

	var prevServerKey ed25519.PublicKey
	if ourPos == 0 {
		prevServerKey = coordinatorKey
	} else {
		prevServerKey = mixers[ourPos-1].Key
		if prevServerKey == nil {
			// first mixer in the config file is called "mixer 1"
			log.Fatalf("alpenhorn mixer %d has no key", ourPos-1+1)
		}
	}

	var nextServer *vrpc.Client
	lastServer := ourPos == len(mixers)-1
	if !lastServer {
		next := mixers[ourPos+1]
		if next.Key == nil || next.Address == "" {
			log.Fatalf("alpenhorn mixer %d is missing a key or address", ourPos+1+1)
		}
		nextServer, err = vrpc.Dial("tcp", next.Address, next.Key, conf.PrivateKey, runtime.NumCPU())
		if err != nil {
			log.Fatalf("vrpc.Dial: %s", err)
		}
	}

	addFriendMixnet := &mixnet.Server{
		SigningKey:     conf.PrivateKey,
		ServerPosition: ourPos,
		NumServers:     len(mixers),
		NextServer:     nextServer,
		CDNAddr:        alpConf.CDN.Address,
		CDNPublicKey:   alpConf.CDN.Key,

		Mixer:   &addfriend.Mixer{},
		Laplace: conf.AddFriendNoise,
	}

	dialingMixnet := &mixnet.Server{
		SigningKey:     conf.PrivateKey,
		ServerPosition: ourPos,
		NumServers:     len(mixers),
		NextServer:     nextServer,
		CDNAddr:        alpConf.CDN.Address,
		CDNPublicKey:   alpConf.CDN.Key,

		Mixer:   &dialing.Mixer{},
		Laplace: conf.DialingNoise,
	}

	srv := new(vrpc.Server)
	if err := srv.Register(coordinatorKey, "DialingCoordinator", &mixnet.CoordinatorService{dialingMixnet}); err != nil {
		log.Fatalf("vrpc.Register: %s", err)
	}
	if err := srv.Register(coordinatorKey, "AddFriendCoordinator", &mixnet.CoordinatorService{addFriendMixnet}); err != nil {
		log.Fatalf("vrpc.Register: %s", err)
	}

	if err := srv.Register(prevServerKey, "DialingChain", &mixnet.ChainService{dialingMixnet}); err != nil {
		log.Fatalf("vrpc.Register: %s", err)
	}
	if err := srv.Register(prevServerKey, "AddFriendChain", &mixnet.ChainService{addFriendMixnet}); err != nil {
		log.Fatalf("vrpc.Register: %s", err)
	}

	err = srv.ListenAndServe(conf.ListenAddr, conf.PrivateKey)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}