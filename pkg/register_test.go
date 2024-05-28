// Copyright 2016 David Lazar. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package pkg

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"alpenhorn/log"
)

func BenchmarkRegister(b *testing.B) {
	_, serverPriv, _ := ed25519.GenerateKey(rand.Reader)
	dbPath, err := ioutil.TempDir("", "alpenhorn_pkg_db_")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dbPath)

	conf := &Config{
		DBPath: dbPath,
		Logger: &log.Logger{
			Level:        log.ErrorLevel,
			EntryHandler: &log.OutputText{Out: log.Stderr},
		},
		SigningKey: serverPriv,
	}
	fmt.Println("-------------------------------")
	dataconf2, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		fmt.Println("nillllllllllllll")

		panic(err)
	}

	fmt.Printf("%s\n", dataconf2)
	fmt.Println("-------------------------------")

	srv, err := NewServer(conf)
	if err != nil {
		fmt.Println("=====================")

		b.Fatal(err)
	}
	defer srv.Close()

	userPub, _, _ := ed25519.GenerateKey(rand.Reader)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		args := &registerArgs{
			Username: fmt.Sprintf("%dbenchmark", i),
			LoginKey: userPub,
		}
		err = srv.register(args)
		if err != nil {
			b.Fatal(err)
		}
	}
}
