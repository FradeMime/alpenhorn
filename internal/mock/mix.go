// Copyright 2016 David Lazar. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package mock

import (
	"crypto/ed25519"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"alpenhorn/addfriend"
	"alpenhorn/dialing"
	"alpenhorn/edtls"
	"alpenhorn/log"

	"vuvuzela.io/crypto/rand"
	"vuvuzela.io/vuvuzela/mixnet"
	"vuvuzela.io/vuvuzela/mixnet/convopb"
)

type Mixchain struct {
	Servers []mixnet.PublicServerConfig

	mixServers []*mixnet.Server
	rpcServers []*grpc.Server
}

func (m *Mixchain) Close() error {
	for _, srv := range m.rpcServers {
		srv.Stop()
	}
	return nil
}

func LaunchMixchain(length int, coordinatorKey ed25519.PublicKey) *Mixchain {
	publicKeys := make([]ed25519.PublicKey, length)
	privateKeys := make([]ed25519.PrivateKey, length)
	listeners := make([]net.Listener, length)
	addrs := make([]string, length)
	for i := 0; i < length; i++ {
		publicKeys[i], privateKeys[i], _ = ed25519.GenerateKey(rand.Reader)
		// Use net.Listen instead of edtls.Listen because grpc will perform
		// the edtls handshake using the TLS credentials below.
		l, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			log.Panicf("net.Listen: %s", err)
		}
		listeners[i] = l
		addrs[i] = l.Addr().String()
	}

	mixServers := make([]*mixnet.Server, length)
	rpcServers := make([]*grpc.Server, length)
	for pos := length - 1; pos >= 0; pos-- {
		mixer := &mixnet.Server{
			SigningKey:     privateKeys[pos],
			CoordinatorKey: coordinatorKey,

			Services: map[string]mixnet.MixService{
				"AddFriend": &addfriend.Mixer{
					SigningKey: privateKeys[pos],
					Laplace: rand.Laplace{
						Mu: 100,
						B:  3.0,
					},
				},

				"Dialing": &dialing.Mixer{
					SigningKey: privateKeys[pos],
					Laplace: rand.Laplace{
						Mu: 100,
						B:  3.0,
					},
				},
			},
		}

		creds := credentials.NewTLS(edtls.NewTLSServerConfig(privateKeys[pos]))

		grpcServer := grpc.NewServer(grpc.Creds(creds))
		convopb.RegisterMixnetServer(grpcServer, mixer)

		mixServers[pos] = mixer
		rpcServers[pos] = grpcServer

		go func(pos int) {
			err := grpcServer.Serve(listeners[pos])
			if err != grpc.ErrServerStopped {
				log.Fatal("vrpc.Serve:", err)
			}
		}(pos)
	}

	serversPublic := make([]mixnet.PublicServerConfig, len(mixServers))
	for i, mixer := range mixServers {
		serversPublic[i] = mixnet.PublicServerConfig{
			Key:     mixer.SigningKey.Public().(ed25519.PublicKey),
			Address: addrs[i],
		}
	}

	return &Mixchain{
		Servers: serversPublic,

		mixServers: mixServers,
		rpcServers: rpcServers,
	}
}
