/*
Copyright © 2019 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/spf13/cobra"

	libgocrypto "github.com/openshift/library-go/pkg/crypto"
)

func defineFlags(cmd *cobra.Command) {
	cmd.Flags().String("address", "1.1.1.1", "Server address")
	cmd.Flags().String("port", "8443", "Server port")
	cmd.Flags().String("path", "/", "Content path")
	cmd.Flags().String("owner", "", "Object owner")
	cmd.Flags().String("scan-index", "", "The current index of the scan")
	cmd.Flags().String("tls-server-cert", "", "Path to the server cert")
	cmd.Flags().String("tls-server-key", "", "Path to the server key")
	cmd.Flags().String("tls-ca", "", "Path to the CA certificate")
}

type config struct {
	Address string
	Port    string
	Path    string
	Cert    string
	Key     string
	CA      string
}

func parseConfig(cmd *cobra.Command) *config {
	basePath := getValidStringArg(cmd, "path")
	index := getValidStringArg(cmd, "scan-index")
	conf := &config{
		Address: getValidStringArg(cmd, "address"),
		Port:    getValidStringArg(cmd, "port"),
		Path:    filepath.Join(basePath, index),
		Cert:    getValidStringArg(cmd, "tls-server-cert"),
		Key:     getValidStringArg(cmd, "tls-server-key"),
		CA:      getValidStringArg(cmd, "tls-ca"),
	}
	return conf
}

func getValidStringArg(cmd *cobra.Command, name string) string {
	val, _ := cmd.Flags().GetString(name)
	if val == "" {
		fmt.Fprintf(os.Stderr, "The command line argument '%s' is mandatory.\n", name)
		os.Exit(1)
	}
	return val
}

func ensureDir(path string) error {
	err := os.MkdirAll(path, 0750)
	if err != nil && !os.IsExist(err) {
		log.Fatal(err)
		return err
	}

	return nil
}

func main() {
	var srvCmd = &cobra.Command{
		Use:   "resultserver",
		Short: "A tool to receive raw SCAP scan results.",
		Long:  "A tool to receive raw SCAP scan results.",
		Run: func(cmd *cobra.Command, args []string) {
			server(parseConfig(cmd))
		},
	}

	defineFlags(srvCmd)

	if err := srvCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func server(c *config) {
	err := ensureDir(c.Path)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	caCert, err := ioutil.ReadFile(c.CA)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{}
	// Configures TLS 1.2
	tlsConfig = libgocrypto.SecureTLSConfig(tlsConfig)
	tlsConfig.ClientCAs = caCertPool
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	tlsConfig.BuildNameToCertificate()
	server := &http.Server{
		Addr:      c.Address + ":" + c.Port,
		TLSConfig: tlsConfig,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		filename := r.Header.Get("X-Report-Name")
		if filename == "" {
			log.Println("Rejecting. No \"X-Report-Name\" header given.")
			http.Error(w, "Missing report name header", 400)
			return
		}
		encoding := r.Header.Get("Content-Encoding")
		extraExtension := encoding
		if encoding != "" && encoding != "bzip2" {
			log.Println("Rejecting. Invalid \"Content-Encoding\" header given.")
			http.Error(w, "invalid content encoding header", 400)
			return
		} else if encoding == "bzip2" {
			// if the results are compressed, they are also base64-encoded, let's make this clear to the user
			extraExtension = "." + extraExtension + ".base64"
		}
		// TODO(jaosorior): Check that content-type is application/xml
		filePath := path.Join(c.Path, filename+".xml"+extraExtension)
		f, err := os.Create(filePath)
		if err != nil {
			log.Printf("Error creating file: %s", filePath)
			http.Error(w, "Error creating file", 500)
			return
		}
		// #nosec
		defer f.Close()

		_, err = io.Copy(f, r.Body)
		if err != nil {
			log.Printf("Error writing file %s", filePath)
			http.Error(w, "Error writing file", 500)
			return
		}
		log.Printf("Received file %s", filePath)
	})
	log.Println("Listening...")
	log.Fatal(server.ListenAndServeTLS(c.Cert, c.Key))
}
